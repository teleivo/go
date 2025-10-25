package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	fmt.Println("=== Scanner Behavior Tests ===\n")

	// Test 1: Invalid numeric literals - different bases
	fmt.Println("1. INVALID NUMERIC LITERALS")
	fmt.Println("   (Scanner consumes decimal digits but marks them as invalid)")
	testScan("0b12345", "Binary with invalid digits")
	testScan("0o89", "Octal with invalid digits")
	testScan("0b1.0", "Binary with decimal point")

	// Test 2: Letters after numbers
	fmt.Println("\n2. LETTERS AFTER NUMBERS")
	fmt.Println("   (Non-decimal/non-hex letters stop consumption)")
	testScan("0b1abc", "Binary + letters")
	testScan("0o7ghi", "Octal + letters")
	testScan("123xyz", "Decimal + letters")
	testScan("0xGHI", "Hex with no valid digits")
	testScan("0xfghi", "Hex 'f' then invalid letters")

	// Test 3: Unrecognized characters
	fmt.Println("\n3. UNRECOGNIZED CHARACTERS")
	fmt.Println("   (Each produces separate ILLEGAL token)")
	testScan("###", "Multiple hash symbols")
	testScan("x := #", "Valid code then illegal char")
	testScan("@!$", "Multiple different illegal chars")

	// Test 4: Curly quotes
	fmt.Println("\n4. CURLY QUOTES")
	fmt.Println("   (Special helpful error message)")
	testScan("\u201Chello\u201D", "Curly double quotes")
}

func testScan(src, desc string) {
	fmt.Printf("\n   %s: %q\n", desc, src)

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	errorCount := 0
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		errorCount++
		fmt.Printf("      Error %d at pos %d: %s\n", errorCount, pos.Offset, msg)
	}, 0)

	tokenNum := 0
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		tokenNum++
		fmt.Printf("      Token %d: %-10s (pos=%d, lit=%q)\n", tokenNum, tok, file.Offset(pos), lit)
	}
	if errorCount == 0 {
		fmt.Printf("      (No errors)\n")
	}
}
