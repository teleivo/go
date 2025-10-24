package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	// Test 1: Invalid numeric literal
	testScan("0b1.0", "Invalid numeric literal")

	// Test 2: Multiple unrecognized characters
	testScan("###", "Multiple unrecognized chars")

	// Test 3: Mixed valid and invalid
	testScan("x := #", "Mixed valid and invalid")
}

func testScan(src, desc string) {
	fmt.Printf("\n=== %s: %q ===\n", desc, src)

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	errorCount := 0
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		errorCount++
		fmt.Printf("  Error %d at pos %d: %s\n", errorCount, pos.Offset, msg)
	}, 0)

	tokenNum := 0
	for {
		pos, tok, lit := s.Scan()
		tokenNum++
		fmt.Printf("  Token %d: %s (pos=%d, lit=%q)\n", tokenNum, tok, file.Offset(pos), lit)
		if tok == token.EOF {
			break
		}
	}
	fmt.Printf("  Total errors: %d\n", errorCount)
}
