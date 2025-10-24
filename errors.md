# Deep Dive: Error Handling in the Go Parser

This document provides a comprehensive analysis of error handling mechanisms in the Go parser
(located in `src/go/parser/`).

## Overview

The Go parser demonstrates **robust error handling** designed to:

* Continue parsing after encountering errors (to report multiple issues)
* Recover gracefully by creating "Bad" AST nodes
* Provide accurate error positions and helpful messages
* Prevent infinite loops and stack exhaustion

## 1. Core Error Infrastructure

### Error Types

Location: `src/go/scanner/errors.go`

```go
// Individual error with position
type Error struct {
    Pos token.Position  // file:line:column
    Msg string
}

// Collection of errors
type ErrorList []*Error
```

**Key characteristics:**

* Errors are sorted by position (filename → line → column → message)
* ErrorList implements the `error` interface
* Format: `filename:line:column: message`

### Bailout Mechanism

Location: `src/go/parser/parser.go:267-272`

```go
type bailout struct {
    pos token.Pos
    msg string
}
```

Used to panic early when:

* Too many errors encountered (>10 by default)
* Max nesting depth exceeded (100,000 levels)
* Max scope depth exceeded during resolution (1,000 levels)

## 2. Error Reporting Functions

### Core Error Function

Location: `src/go/parser/parser.go:274-295`

```go
func (p *parser) error(pos token.Pos, msg string) {
    epos := p.file.Position(pos)

    if p.mode&AllErrors == 0 {
        n := len(p.errors)
        // Filter duplicate errors on same line
        if n > 0 && p.errors[n-1].Pos.Line == epos.Line {
            return // discard - likely spurious
        }
        // Stop after 10 errors
        if n > 10 {
            panic(bailout{})
        }
    }

    p.errors.Add(epos, msg)
}
```

**Behavior:**

* By default: filters duplicates on same line, stops after 10 errors
* With `AllErrors` mode: reports all errors without filtering

### Context-Aware Error Functions

**errorExpected** (parser.go:297-313): Enriches messages based on current token

```go
// Input: expected "';'"
// Output: expected ';', found newline
// Output: expected ';', found '}'
```

**expectClosing** (parser.go:336-344): Better errors for missing commas

```go
// Before newline in parameter list:
// "missing ',' before newline in parameter list"
```

**atComma** (parser.go:374-387): Context-aware comma checking

```go
// "missing ',' in argument list"
// "missing ',' before newline in composite literal"
```

## 3. Error Recovery Strategies

### Strategy 1: Synchronization Maps

Predefined token sets for recovery (parser.go:425-456):

```go
var stmtStart = map[token.Token]bool{
    token.BREAK, token.CONST, token.CONTINUE, token.DEFER,
    token.FOR, token.IF, token.RETURN, token.SWITCH, // ...
}

var declStart = map[token.Token]bool{
    token.IMPORT, token.CONST, token.TYPE, token.VAR,
}

var exprEnd = map[token.Token]bool{
    token.COMMA, token.COLON, token.SEMICOLON,
    token.RPAREN, token.RBRACK, token.RBRACE,
}
```

### Strategy 2: Advance Function

Location: parser.go:395-423

Skips tokens until reaching a synchronization point:

```go
func (p *parser) advance(to map[token.Token]bool) {
    for ; p.tok != token.EOF; p.next() {
        if to[p.tok] {
            // Only return if making progress
            if p.pos == p.syncPos && p.syncCnt < 10 {
                p.syncCnt++
                return
            }
            if p.pos > p.syncPos {
                p.syncPos = p.pos
                p.syncCnt = 0
                return
            }
        }
    }
}
```

**Protection against infinite loops:** Tracks last sync position and limits retries.

### Strategy 3: Bad AST Nodes

When parsing fails, create placeholder nodes:

* `ast.BadExpr` - Invalid expressions (parser.go:546, 738, 990, etc.)
* `ast.BadStmt` - Invalid statements (parser.go:1994, 2039, 2054, etc.)
* `ast.BadDecl` - Invalid declarations (parser.go:2875)

**Example from parseIndexOrSliceOrInstance** (parser.go:1568-1573):

```go
if p.tok == token.RBRACK {
    // Empty index - error but continue
    return &ast.IndexExpr{
        X: x,
        Index: &ast.BadExpr{From: rbrack, To: rbrack},
        Rbrack: rbrack,
    }
}
```

### Strategy 4: Nesting Depth Limits

**Parser nesting** (parser.go:117-133):

```go
const maxNestLev int = 1e5

func incNestLev(p *parser) *parser {
    p.nestLev++
    if p.nestLev > maxNestLev {
        p.error(p.pos, "exceeded max nesting depth")
        panic(bailout{})
    }
    return p
}
```

**Resolver scope depth** (resolver.go:56, 88-92):

```go
const maxScopeDepth int = 1e3

func (r *resolver) openScope(pos token.Pos) {
    r.depth++
    if r.depth > maxScopeDepth {
        panic(bailout{pos: pos, msg: "exceeded max scope depth during object resolution"})
    }
    r.topScope = ast.NewScope(r.topScope)
}
```

## 4. Complete Error Flow

### ParseFile Flow

Location: interface.go:83-133

```
1. Read source → readSource()
2. Initialize parser → p.init()
3. Parse with defer/recover:
   ┌──────────────────────────────┐
   │ defer func() {               │
   │   if e := recover(); e != nil│
   │     if bailout → add error   │
   │     else → re-panic          │
   │   }                          │
   │   p.errors.Sort()            │
   │   err = p.errors.Err()       │
   │ }()                          │
   └──────────────────────────────┘
4. Return AST + errors
```

**Key insight:** Even on panic, returns a valid (possibly empty) AST.

### Parser Initialization

Location: parser.go:76-85

```go
func (p *parser) init(file *token.File, src []byte, mode Mode) {
    p.file = file
    // Scanner errors go to same ErrorList
    eh := func(pos token.Position, msg string) {
        p.errors.Add(pos, msg)
    }
    p.scanner.Init(p.file, src, eh, scanner.ScanComments)
    // ...
}
```

Both **scanner** (lexical) and **parser** (syntactic) errors go to the same `ErrorList`.

## 5. Declaration Error Handling (Resolver)

The resolver handles semantic errors in `src/go/parser/resolver.go`.

### Redeclaration Errors

Location: resolver.go:127-155

```go
func (r *resolver) declare(..., idents ...*ast.Ident) {
    for _, ident := range idents {
        obj := ast.NewObj(kind, ident.Name)
        if alt := scope.Insert(obj); alt != nil && r.declErr != nil {
            prevDecl := r.sprintf("\n\tprevious declaration at %v", alt.Pos())
            r.declErr(ident.Pos(),
                fmt.Sprintf("%s redeclared in this block%s",
                    ident.Name, prevDecl))
        }
    }
}
```

### Short Variable Declaration Errors

Location: resolver.go:157-184

```go
func (r *resolver) shortVarDecl(decl *ast.AssignStmt) {
    n := 0 // count new variables
    for _, x := range decl.Lhs {
        if ident, isIdent := x.(*ast.Ident); isIdent {
            if alt := r.topScope.Insert(obj); alt != nil {
                ident.Obj = alt // redeclaration
            } else {
                n++ // new declaration
            }
        }
    }
    if n == 0 && r.declErr != nil {
        r.declErr(decl.Lhs[0].Pos(), "no new variables on left side of :=")
    }
}
```

### Undefined Label Errors

Location: resolver.go:112-125

```go
func (r *resolver) closeLabelScope() {
    for _, ident := range r.targetStack[n] {
        ident.Obj = scope.Lookup(ident.Name)
        if ident.Obj == nil && r.declErr != nil {
            r.declErr(ident.Pos(),
                fmt.Sprintf("label %s undefined", ident.Name))
        }
    }
}
```

## 6. Parser Modes for Error Control

Location: interface.go:43-57

```go
const (
    PackageClauseOnly    Mode = 1 << iota  // Stop after package
    ImportsOnly                             // Stop after imports
    ParseComments                           // Include comments
    Trace                                   // Debug trace
    DeclarationErrors                       // Enable resolution errors
    SkipObjectResolution                    // Skip resolution
    AllErrors                               // Report all (no filtering)
)
```

**Common combinations:**

* Default parsing: `0` (filters errors, stops at 10)
* Parse with comments: `ParseComments`
* All errors: `AllErrors` (for IDE/tooling)
* Skip resolution: `SkipObjectResolution` (recommended, faster)

## 7. Specific Error Recovery Patterns

### Pattern 1: Missing Expressions

From parseIndexOrSliceOrInstance (parser.go:1568-1573):

```go
if p.tok == token.RBRACK {
    p.errorExpected(p.pos, "operand")
    return &ast.IndexExpr{
        X: x,
        Index: &ast.BadExpr{From: rbrack, To: rbrack},
        Rbrack: rbrack,
    }
}
```

### Pattern 2: Missing Middle Index in Slice

From parseIndexOrSliceOrInstance (parser.go:1618-1626):

```go
if index[1] == nil {
    p.error(colons[0], "middle index required in 3-index slice")
    index[1] = &ast.BadExpr{From: colons[0] + 1, To: colons[1]}
}
```

### Pattern 3: Invalid Statement Recovery

From parseStmt (parser.go:2505-2509):

```go
default:
    pos := p.pos
    p.errorExpected(pos, "statement")
    p.advance(stmtStart)  // Skip to next statement
    s = &ast.BadStmt{From: pos, To: p.pos}
}
```

### Pattern 4: Illegal Label Detection

From parseLabeledStmtOrExpr (parser.go:1991-1994):

```go
p.error(colon, "illegal label declaration")
return &ast.BadStmt{From: x[0].Pos(), To: colon + 1}, false
```

## 8. Error Position Safety

The safePos function (parser.go:458-475) prevents panics from invalid positions:

```go
func (p *parser) safePos(pos token.Pos) (res token.Pos) {
    defer func() {
        if recover() != nil {
            res = token.Pos(p.file.Base() + p.file.Size()) // EOF
        }
    }()
    _ = p.file.Offset(pos) // trigger panic if out-of-range
    return pos
}
```

This is crucial because AST nodes compute "artificial" end positions by adding 1 to token
positions.

## 9. Error Output Format

### Single Error Format

Location: scanner/errors.go:24-31

```go
func (e Error) Error() string {
    if e.Pos.Filename != "" || e.Pos.IsValid() {
        return e.Pos.String() + ": " + e.Msg
    }
    return e.Msg
}
```

Output: `filename:line:column: message`

Example: `/path/to/file.go:10:5: expected ';', found newline`

### ErrorList Format

Location: scanner/errors.go:90-98

```go
func (p ErrorList) Error() string {
    switch len(p) {
    case 0:
        return "no errors"
    case 1:
        return p[0].Error()
    }
    return fmt.Sprintf("%s (and %d more errors)", p[0], len(p)-1)
}
```

## 10. Parser Structure and State

Location: parser.go:36-74

```go
type parser struct {
    file    *token.File          // File handle
    errors  scanner.ErrorList    // Accumulated errors
    scanner scanner.Scanner      // Lexical scanner

    // Tracing/debugging
    mode   Mode     // Parsing mode
    trace  bool     // Tracing enabled
    indent int      // Trace indentation

    // Comments
    comments    []*ast.CommentGroup
    leadComment *ast.CommentGroup
    lineComment *ast.CommentGroup
    top         bool              // In top of file
    goVersion   string            // From //go:build

    // Next token (one-token lookahead)
    pos token.Pos  // Token position
    tok token.Token // Token type
    lit string      // Token literal

    // Error recovery
    syncPos token.Pos  // Last synchronization position
    syncCnt int        // Advance calls without progress

    // Non-syntactic control
    exprLev int  // Expression nesting level
    inRhs   bool // Parsing RHS expression

    imports []*ast.ImportSpec
    nestLev int  // Recursion depth tracking
}
```

## Key Architectural Files

| File | Lines | Purpose |
|------|-------|---------|
| `src/go/parser/parser.go` | ~3,800 | Core parser implementation |
| `src/go/parser/interface.go` | ~254 | Public API (ParseFile, etc.) |
| `src/go/parser/resolver.go` | ~400 | Identifier resolution & declaration errors |
| `src/go/scanner/errors.go` | ~121 | Error types (Error, ErrorList) |
| `src/go/scanner/scanner.go` | ~1,000+ | Lexical scanning & token errors |
| `src/go/parser/error_test.go` | ~203 | Error testing framework |

## Key Takeaways

1. **Robustness:** Parser always returns a valid AST, even with errors
2. **Multiple errors:** Reports many errors in one pass (configurable)
3. **Graceful degradation:** Bad* nodes maintain AST structure
4. **Context-aware messages:** Errors include helpful context about what was expected
5. **DoS prevention:** Depth limits prevent stack exhaustion attacks
6. **Loop prevention:** Synchronization tracking avoids infinite loops
7. **Position accuracy:** file:line:column format with proper sorting
8. **Separation of concerns:** Scanner (lexical), Parser (syntactic), Resolver (semantic)

The error handling in the Go parser is a **masterclass in resilient parsing** - it prioritizes
helping developers fix multiple issues while maintaining internal consistency and security.
