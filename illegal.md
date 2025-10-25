# Walkthrough: The ILLEGAL Token in Go's Scanner and Parser

This document explains how Go uses the `ILLEGAL` token to handle invalid source code.

## Table of Contents

* [Overview](#overview)
* [Token Definition](#token-definition)
* [When ILLEGAL is Emitted](#when-illegal-is-emitted)
* [Scanner Implementation](#scanner-implementation)
* [Parser Error Recovery](#parser-error-recovery)
* [Compiler Syntax Package](#compiler-syntax-package)
* [Semantic Usage in AST](#semantic-usage-in-ast)
* [Testing](#testing)
* [Summary](#summary)

## Overview

The `ILLEGAL` token represents completely unrecognizable characters in Go source code. It serves
multiple purposes:

* **Error Detection**: Identifies characters that don't match any valid Go token
* **Error Recovery**: Allows the parser to continue after errors
* **Semantic Marker**: Indicates special cases in AST nodes (e.g., range loops without variables)
* **Sentinel Value**: Signals invalid operations in type checking

### Critical Invariants

**Invariant 1: `ILLEGAL` ⟹ Error Reported**

When `token.ILLEGAL` is returned, the scanner's error handler has **always** been called. There is
no case where `ILLEGAL` is emitted without an associated error.

**Invariant 2: Error ⏸ `ILLEGAL`**

The converse is NOT true. Many scanner errors do NOT produce `ILLEGAL` tokens. When the scanner can
determine what the programmer likely intended, it returns the appropriate token type with an error:

* Invalid numeric literal `0b1.0` → `token.FLOAT` + error
* Unterminated string `"abc` → `token.STRING` + error
* Invalid escape sequence → string/char token + error

**Design Philosophy: Intent vs Recognition**

* **Intent unclear** → `ILLEGAL` token (e.g., `#`, `@`, `…`)
* **Intent clear but malformed** → Appropriate token + error (e.g., `0b1.0` → `FLOAT`)

This optimizes for error recovery and enables collecting multiple errors in one pass.

## Token Definition

**Location**: `src/go/token/token.go:21`

```go
// Token is the set of lexical tokens of the Go programming language.
type Token int

const (
	ILLEGAL Token = iota  // value: 0
	EOF
	COMMENT
	// ... other tokens
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	// ... other token strings
}
```

**Key Characteristics**:

* **Value**: 0 (the zero value for `token.Token`)
* **String Representation**: "ILLEGAL"
* Uninitialized token variables automatically have value `ILLEGAL`, making bugs easier to detect

## When ILLEGAL is Emitted

The `ILLEGAL` token is emitted **only** from the default case in the main scan loop
(`src/go/scanner/scanner.go:965`) for completely unrecognizable input:

### Scenarios that Produce ILLEGAL

* **Unrecognized characters**: `#`, `@`, `…`
* **Curly quotes**: `"`, `"` (common copy/paste error)
* **Misplaced byte order marks**: BOM (U+FEFF) after the first character
  * Error reported in `next()` at line 86
  * `ILLEGAL` returned in default case at line 965
  * Error reporting skipped at line 955 to avoid duplication

### Scenarios that Do NOT Produce ILLEGAL

These produce errors but return meaningful token types:

* **Invalid UTF-8**: Error reported, continues with replacement character
* **Invalid numeric literals**: Returns `token.FLOAT`/`INT`/`IMAG` + error
* **Malformed strings/runes**: Returns `token.STRING`/`CHAR` + error
* **Invalid escape sequences**: Returns string/char token + error
* **Invalid digit separators**: Returns numeric token + error

## Scanner Implementation

**Location**: `src/go/scanner/scanner.go`

### Main Scan Method

The scanner's `Scan()` method processes characters via a large switch statement. The default case
handles unrecognized characters:

```go
default:
	// next reports unexpected BOMs - don't repeat
	if ch != bom {
		if ch == '"' || ch == '"' {
			s.errorf(s.file.Offset(pos), "curly quotation mark %q (use neutral %q)", ch, '"')
		} else {
			s.errorf(s.file.Offset(pos), "illegal character %#U", ch)
		}
	}
	insertSemi = s.insertSemi
	tok = token.ILLEGAL
	lit = string(ch)  // Literal is the offending character (single rune)
```

**Key behavior: Each unrecognized character produces a separate ILLEGAL token**

Unlike numeric literals (which are consumed entirely), unrecognized characters are processed one at
a time:

* `###` produces THREE separate `ILLEGAL` tokens, one for each `#`
* Each token has `lit = "#"` (the single character)
* Three separate error messages are generated

**Example scan sequence**:

```
Input: "###"
Token 1: ILLEGAL (pos=0, lit="#") + error "illegal character U+0023 '#'"
Token 2: ILLEGAL (pos=1, lit="#") + error "illegal character U+0023 '#'"
Token 3: ILLEGAL (pos=2, lit="#") + error "illegal character U+0023 '#'"
Token 4: EOF
```

This is fundamentally different from number scanning, where the entire malformed literal is consumed
as a single token.

### Number Scanning

**Location**: `src/go/scanner/scanner.go:425`

Number scanning initializes `tok` to `ILLEGAL` but updates it as parsing progresses. **Syntax
errors during number scanning do NOT return `ILLEGAL`**:

```go
func (s *Scanner) scanNumber() (token.Token, string) {
	tok := token.ILLEGAL  // Initialize

	// integer part
	if s.ch != '.' {
		tok = token.INT
	}

	// fractional part
	if s.ch == '.' {
		tok = token.FLOAT  // Even if invalid!
		if prefix == 'o' || prefix == 'b' {
			s.error(s.offset, "invalid radix point in "+litname(prefix))
			// tok remains FLOAT, not ILLEGAL
		}
	}

	// exponent, suffix 'i', etc...
	return tok, lit
}
```

**Key behavior: The scanner consumes the entire malformed literal**

The scanner's `digits()` function continues consuming characters that "look like" they belong to the
literal, even if they're invalid for that base:

* `0b12345` → `token.INT` with literal `"0b12345"` + error "invalid digit '2' in binary literal"
* `0o89` → `token.INT` with literal `"0o89"` + error "invalid digit '8' in octal literal"
* `0b1.0` → `token.FLOAT` with literal `"0b1.0"` + error "invalid radix point in binary literal"

The entire malformed literal is consumed as a single token, not split into multiple tokens. The
error message pinpoints the specific invalid character within the literal.

**Edge case: Why letters sometimes stop consumption**

The key is in the `digits()` function's loop conditions:

* **For base ≤ 10** (binary, octal, decimal): `for isDecimal(s.ch) || s.ch == '_'`
  * Continues consuming ANY decimal digit (0-9), even if invalid for that base
  * `0o89` consumes both `8` and `9` because `isDecimal('8')` is true, then reports error
  * `0b12345` consumes all digits because `isDecimal('2')` through `isDecimal('5')` are true

* **For base > 10** (hexadecimal): `for isHex(s.ch) || s.ch == '_'`
  * Only continues if character is hex digit (0-9, a-f) or underscore
  * `0xGHI` stops at `G` because `isHex('G')` is false → two tokens: `"0x"` + `"GHI"`
  * `0xfghi` stops at `g` because `isHex('g')` is false → two tokens: `"0xf"` + `"ghi"`

* **Letters after any base**: Non-hex letters (g-z) always stop consumption
  * `0b1abc` → `"0b1"` + `"abc"` (no error, valid binary followed by identifier)
  * `123xyz` → `"123"` + `"xyz"` (no error, valid decimal followed by identifier)

The scanner uses `isDecimal()` and `isHex()` as **lookahead predicates** to decide whether to
continue consuming characters as part of the numeric literal.

**Why the inconsistency? A pragmatic choice**

You might notice an asymmetry:
* `0o89` consumes digits beyond base → "looks like attempted octal"
* `0xGHI` stops at non-hex letters → "doesn't look like hex"

Why not treat `G` as "looks like attempted hex" similar to `8` in octal?

The likely rationale (this behavior is consistent in both `go/scanner` and `cmd/compile/internal/syntax`):

1. **Digit typos are obvious**: `0o789` is clearly a typo in octal context (file permissions, etc.)
   * All digits 0-9 "look like" they belong in numbers
   * User likely meant `0o755` or `0o777`

2. **Letters beyond 'f' are ambiguous**: `0xGOOD` could be:
   * Typo in hex literal, OR
   * Missing space: `0x GOOD` (hex zero followed by identifier)

3. **Identifiers commonly use capital letters**: `GetValue`, `HTTPClient`, `ParseURL`
   * Treating `0xGOOD` as `0x` + `GOOD` preserves the identifier for error messages
   * Alternative (consuming as `"0xGOOD"`) would hide the identifier completely

4. **Different error recovery value**:
   * `0o789` → single token preserves the attempted literal for error context
   * `0xGOOD` → two tokens (`0x` + `GOOD`) helps identify what the identifier was

This is a pragmatic design choice that optimizes for the most likely programmer intent based on
common usage patterns.

**Trade-off: Less specific errors for adjacent tokens**

This design creates a limitation: `123xyz` produces two tokens (`INT` + `IDENT`) leading to a
generic parser error "expected ';', found xyz" rather than a specific scanner error about a
malformed literal. Compare to `0o789` which gives "invalid digit '8' in octal literal". The parser
could detect adjacent tokens via position information but doesn't, making error messages less
specific for cases like `123xyz`.

### Scanner API Contract

**Location**: `src/go/scanner/scanner.go:785-786`

```go
// If the returned token is [token.ILLEGAL], the literal string is the
// offending character.
```

This allows callers to show the actual problematic character in error messages.

## Parser Error Recovery

**Location**: `src/go/parser/parser.go`

The parser has **no special handling** for `ILLEGAL` tokens. It treats them like any unexpected
token and uses the general error recovery mechanism.

### Initial State

**Location**: `src/go/parser/parser.go:141`

The parser's initial (uninitialized) token value is `ILLEGAL`:

```go
// The very first token (!p.pos.IsValid()) is not initialized
// (it is token.ILLEGAL), so don't print it.
```

### Error Recovery Mechanism

**Location**: `src/go/parser/parser.go:396-423`

The `advance()` function skips tokens until finding a synchronization point (typically keywords or
operators indicating new statements/declarations):

```go
func (p *parser) advance(to map[token.Token]bool) {
	for ; p.tok != token.EOF; p.next() {
		if to[p.tok] {
			// Return if progress made or sync limit not reached
			if pos := p.file.Offset(p.pos); pos > p.syncPos && p.syncCnt < 10 {
				p.syncPos = pos
				p.syncCnt = 0
				return
			}
			if p.syncCnt < 10 {
				p.syncCnt++
				return
			}
		}
		p.next()
	}
}
```

**Example**:

```go
func foo() {
	x := 1
	y := @  // ILLEGAL token
	z := 3
}
```

When `@` produces an `ILLEGAL` token, the parser creates this AST:

1. Statement 1: `x := 1` (parsed successfully as `ast.AssignStmt`)
2. Statement 2: `y := BadExpr` (error placeholder, `ast.BadExpr` spans lines 5-7)
3. Error reported: "expected operand, found 'ILLEGAL'"

Note: `z := 3` is **not** parsed as a separate statement. The parser's error recovery consumed
tokens up to the closing brace, including `z := 3`, as part of the `BadExpr` region. The parser
creates a `BadExpr` node to preserve the AST structure while indicating an error occurred

## Compiler Syntax Package

**Location**: `src/cmd/compile/internal/syntax/`

The Go compiler's internal syntax package uses a different approach—**no `ILLEGAL` token**.

### No ILLEGAL Token

**Location**: `src/cmd/compile/internal/syntax/tokens.go:14`

```go
const (
	_    token = iota  // 0 is unused
	_EOF
	_Name
	_Literal
	// ... other tokens
)
```

### The `bad` Flag Alternative

**Location**: `src/cmd/compile/internal/syntax/scanner.go:40`

Instead of `ILLEGAL` tokens, the compiler uses a `bad` flag:

```go
type scanner struct {
	// ... fields ...
	tok   token
	lit   string
	bad   bool     // true if a syntax error occurred
	kind  LitKind
}
```

**Usage**: `src/cmd/compile/internal/syntax/scanner.go:63-68`

```go
func (s *scanner) setLit(kind LitKind, ok bool) {
	s.tok = _Literal
	s.lit = string(s.segment())
	s.bad = !ok  // Mark as bad if parsing failed
	s.kind = kind
}
```

**Error Handling**: `src/cmd/compile/internal/syntax/scanner.go:351-354`

```go
default:
	s.errorf("invalid character %#U", s.ch)
	s.nextch()
	goto redo  // Continue scanning
```

Both approaches achieve the same goal: error reporting without stopping compilation.

## Semantic Usage in AST

The `ILLEGAL` token isn't just for errors—it's used semantically to indicate special cases.

### RangeStmt Example

**Location**: `src/go/ast/ast.go:772`

```go
// A RangeStmt represents a for statement with a range clause.
RangeStmt struct {
	For        token.Pos
	Key, Value Expr        // may be nil
	TokPos     token.Pos   // invalid if Key == nil
	Tok        token.Token // ILLEGAL if Key == nil, ASSIGN, DEFINE
	Range      token.Pos
	X          Expr
	Body       *BlockStmt
}
```

**Usage**:

```go
// With variables: Tok = DEFINE
for k, v := range m {
	// Key = k, Value = v, Tok = token.DEFINE
}

// Without variables: Tok = ILLEGAL (valid usage, not an error)
for range slice {
	// Key = nil, Value = nil, Tok = token.ILLEGAL
}
```

### Type Checking

**Location**: `src/go/types/stmt.go`

```go
func assignOp(op token.Token) token.Token {
	if token.ADD_ASSIGN <= op && op <= token.AND_NOT_ASSIGN {
		return op + (token.ADD - token.ADD_ASSIGN)
	}
	return token.ILLEGAL  // Not a valid assignment operator
}

// Later:
if op == token.ILLEGAL {
	check.errorf(atPos(s.TokPos), InvalidSyntaxTree, "unknown assignment operation %s", s.Tok)
	return
}
```

## Testing

### Scanner Testing

**Location**: `src/go/scanner/scanner_test.go`

#### Test Structure (lines 751-757)

```go
var errors = []struct {
	src string        // Input source
	tok token.Token   // Expected token
	pos int           // Error position (offset)
	lit string        // Token literal
	err string        // Expected error message
}
```

#### ILLEGAL Token Test Cases (lines 758-760, 812, 818)

```go
{"\a", token.ILLEGAL, 0, "", "illegal character U+0007"},
{`#`, token.ILLEGAL, 0, "", "illegal character U+0023 '#'"},
{`…`, token.ILLEGAL, 0, "", "illegal character U+2026 '…'"},
{"\ufeff\ufeff", token.ILLEGAL, 3, "\ufeff\ufeff", "illegal byte order mark"},
{""abc"", token.ILLEGAL, 0, "abc", `curly quotation mark '"' (use neutral '"')`},
```

Each test verifies: token type, error position, error message.

#### Key Test Helper (lines 720-749)

```go
func checkError(t *testing.T, src string, tok token.Token, pos int, lit, err string) {
	// ... setup ...

	if tok0 != tok {
		t.Errorf("%q: got %s, expected %s", src, tok0, tok)
	}

	// Special handling: literal check skipped for ILLEGAL
	if tok0 != token.ILLEGAL && lit0 != lit {
		t.Errorf("%q: got literal %q, expected %q", src, lit0, lit)
	}

	// Verify error message and position
}
```

### Parser Testing

**Location**: `src/go/parser/error_test.go`

Parser tests use comment-based error markers in `.src` test files:

```go
// ERROR comments: /* ERROR "regex" */
var errRx = regexp.MustCompile(`^/\* *ERROR *(HERE|AFTER)? *"([^"]*)" *\*/$`)
```

**Example**: `src/go/parser/testdata/issue23434.src`

```go
func g() {
	m := make(map[string]! /* ERROR "expected type, found '!'" */ )
	for {  // Parser recovers and continues
		x := 1
		print(x)
	}
}
```

The `!` triggers an `ILLEGAL` token, parser reports the error, then continues successfully parsing
the `for` loop with no cascading errors.

### Test Coverage Summary

| Component | Test File | Key Tests | Coverage |
|-----------|-----------|-----------|----------|
| go/scanner | scanner_test.go | TestScanErrors (68+ cases) | ILLEGAL chars, UTF-8, BOM |
| go/scanner | scanner_test.go | TestUTF16 (2 cases) | Multi-error scenarios |
| go/parser | error_test.go | TestErrors (all .src files) | Error recovery |
| syntax scanner | scanner_test.go | TestScanErrors (70+ cases) | Similar with `bad` flag |

## Summary

### Design Patterns

1. **Zero Value Pattern**: `ILLEGAL = 0` makes uninitialized tokens identifiable
2. **Graceful Degradation**: Return best-guess token types for error recovery
3. **Multiple Error Collection**: Continue parsing after errors to find more issues
4. **Semantic Sentinel**: Valid use in AST for special cases (e.g., range without variables)
5. **Type Safety**: Sentinel value for invalid operations

### Recovery Strategy Across Components

| Component | Strategy | Details |
|-----------|----------|---------|
| **go/scanner** | Error callback + token | Returns ILLEGAL for unrecognizable input, best-guess tokens for malformed but recognizable input |
| **go/parser** | advance() sync | Skips unexpected tokens (including ILLEGAL) until reaching sync points |
| **cmd/compile/syntax** | bad flag | Sets `bad=true` on tokens, no ILLEGAL token needed |
| **go/types** | Sentinel check | Uses ILLEGAL to indicate invalid operators |

### Key Files

**Core Definition & Scanning**:

* `src/go/token/token.go:21` - definition
* `src/go/scanner/scanner.go:425,965` - emission
* `src/go/scanner/scanner_test.go:758-818` - tests

**Parsing & AST**:

* `src/go/parser/parser.go:141,396` - error recovery
* `src/go/ast/ast.go:772` - semantic use (RangeStmt)

**Type Checking**:

* `src/go/types/stmt.go` - operator validation

**Compiler** (different approach):

* `src/cmd/compile/internal/syntax/tokens.go:14` - no ILLEGAL token
* `src/cmd/compile/internal/syntax/scanner.go:40` - uses `bad` flag

### Key Takeaways

1. **ILLEGAL is rare**: Only for completely unrecognizable input (`#`, `@`, etc.)
2. **Errors ≠ ILLEGAL**: Most errors return appropriate tokens (FLOAT, STRING, etc.)
3. **Always an error**: ILLEGAL tokens always have associated error messages
4. **Parser agnostic**: Parser has no special ILLEGAL handling, just general error recovery
5. **Dual purpose**: Error marker AND valid semantic indicator (range loops, sentinel values)
6. **Compiler differs**: Internal compiler uses `bad` flag instead of ILLEGAL token

## Questions

TODO answer them later
* the Scanner only counts errors but does not collect them. errors are forwarded to the ErrorHandler
  if any. Does the parser then collect errors? And as what type? What info does it add to them? Does
  the parser differentiate between a scanner and parser error? How?
