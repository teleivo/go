package main

import (
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
)

func main() {
	src := `package main
func main() {
	x := 123xyz
}`

	fmt.Println("=== SCANNER vs PARSER ===\n")
	fmt.Println("Source code:")
	fmt.Println(src)
	fmt.Println()

	// Test 1: Scanner (lexical analysis only)
	fmt.Println("1. SCANNER OUTPUT (tokenization only):")
	fmt.Println("   Scanner doesn't validate syntax, just breaks into tokens")
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("test.go", fset.Base(), len(src))

	scannerErrors := 0
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		fmt.Printf("   Scanner error: %s\n", msg)
		scannerErrors++
	}, 0)

	fmt.Println("   Tokens found:")
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT || tok == token.INT {
			fmt.Printf("     %s(%q) at pos %d\n", tok, lit, file.Offset(pos))
		}
	}
	fmt.Printf("   Scanner errors: %d\n\n", scannerErrors)

	// Test 2: Parser (syntax analysis)
	fmt.Println("2. PARSER OUTPUT (syntax validation):")
	fmt.Println("   Parser validates that token sequences are valid Go syntax")
	fset2 := token.NewFileSet()
	_, err := parser.ParseFile(fset2, "test.go", src, 0)
	if err != nil {
		fmt.Printf("   Parser error: %s\n", err)
	} else {
		fmt.Println("   No errors (valid Go syntax)")
	}

	fmt.Println("\n=== EXPLANATION ===")
	fmt.Println("Scanner's job:")
	fmt.Println("  - Break input into tokens (lexical analysis)")
	fmt.Println("  - 123xyz → INT(\"123\") + IDENT(\"xyz\") ✓")
	fmt.Println("  - Both tokens are individually valid")
	fmt.Println()
	fmt.Println("Parser's job:")
	fmt.Println("  - Validate token sequences (syntax analysis)")
	fmt.Println("  - INT followed immediately by IDENT is invalid ✗")
	fmt.Println("  - Expected operator, semicolon, etc. between them")
}
