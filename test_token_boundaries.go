package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	fmt.Println("=== Token Boundary Question ===\n")

	fmt.Println("Your argument: 123xyz has no whitespace, so it should be ONE token")
	fmt.Println("My claim: Scanner treats it as TWO tokens\n")

	fmt.Println("Let's examine similar cases:\n")

	// Case 1: Numbers and operators
	testScan("123+456", "Number + operator + number")
	testScan("123 + 456", "Same with spaces")

	// Case 2: Keywords and identifiers
	testScan("ifx", "Keyword 'if' + identifier 'x'?")
	testScan("if x", "Same with space")

	// Case 3: Number and identifier
	testScan("123xyz", "Number + identifier (your case)")
	testScan("123 xyz", "Same with space")

	// Case 4: Invalid octal (your comparison)
	testScan("0o789", "Invalid octal")
	testScan("0o78 9", "Same with space")

	fmt.Println("\n=== The Pattern ===")
	fmt.Println("Greedy maximal munch rule:")
	fmt.Println("  - ifx → ONE token IDENT('ifx'), NOT IF + IDENT('x')")
	fmt.Println("  - 123+456 → THREE tokens: INT + PLUS + INT")
	fmt.Println("  - 0o789 → ONE token INT('0o789') because digits() keeps consuming decimals")
	fmt.Println("  - 123xyz → ??? tokens")

	fmt.Println("\nQuestion: Why does 0o789 consume all but 123xyz stops?")
}

func testScan(src, desc string) {
	fmt.Printf("%s: %q\n", desc, src)

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	hasError := false
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		fmt.Printf("  ERROR: %s\n", msg)
		hasError = true
	}, 0)

	tokens := []string{}
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if lit == "" || lit == "\n" {
			tokens = append(tokens, tok.String())
		} else {
			tokens = append(tokens, fmt.Sprintf("%s(%q)", tok, lit))
		}
	}

	fmt.Printf("  Tokens: %v", tokens)
	if !hasError {
		fmt.Printf(" (no errors)")
	}
	fmt.Println()
}
