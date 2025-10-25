package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

func main() {
	src := `package main

func foo() {
	x := 1
	y := @
	z := 3
}
`

	fmt.Println("=== Source Code ===")
	for i, line := range strings.Split(src, "\n") {
		if line != "" {
			fmt.Printf("%d: %s\n", i+1, line)
		}
	}

	fmt.Println("\n=== Parsing with ILLEGAL token ===")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.AllErrors)

	if err != nil {
		fmt.Printf("\nParser errors:\n%s\n", err)
	}

	if file != nil {
		fmt.Println("\n=== AST Structure ===")
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "foo" {
				fmt.Printf("Function 'foo' has %d statements in its body:\n\n", len(fn.Body.List))

				for i, stmt := range fn.Body.List {
					pos := fset.Position(stmt.Pos())
					fmt.Printf("Statement %d (line %d): ", i+1, pos.Line)

					if assign, ok := stmt.(*ast.AssignStmt); ok {
						// Print LHS
						for j, lhs := range assign.Lhs {
							if j > 0 {
								fmt.Print(", ")
							}
							if ident, ok := lhs.(*ast.Ident); ok {
								fmt.Print(ident.Name)
							}
						}
						fmt.Printf(" %s ", assign.Tok)

						// Print RHS
						for j, rhs := range assign.Rhs {
							if j > 0 {
								fmt.Print(", ")
							}
							switch r := rhs.(type) {
							case *ast.BasicLit:
								fmt.Printf("%s (BasicLit)", r.Value)
							case *ast.BadExpr:
								from := fset.Position(r.From)
								to := fset.Position(r.To)
								fmt.Printf("BadExpr (lines %d-%d)", from.Line, to.Line)
							default:
								fmt.Printf("(%T)", r)
							}
						}
						fmt.Println()
					} else {
						fmt.Printf("%T\n", stmt)
					}
				}
			}
		}
	}

	fmt.Println("\n=== Explanation ===")
	fmt.Println("1. Statement 'x := 1' parsed successfully")
	fmt.Println("2. Statement 'y := @' created with BadExpr for the @ token")
	fmt.Println("3. The BadExpr spans from the @ to the end (parser consumed rest during error recovery)")
	fmt.Println("4. Statement 'z := 3' was NOT parsed as a separate statement")
	fmt.Println()
	fmt.Println("The parser's error recovery consumed tokens until the closing brace,")
	fmt.Println("so 'z := 3' became part of the BadExpr region.")
}
