package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	fmt.Println("=== Why does 0o89 consume but 0xGHI doesn't? ===\n")

	// Scenario 1: Typo in octal
	fmt.Println("Scenario 1: User likely meant octal")
	testScan("0o755", "Valid octal (file permissions)")
	testScan("0o775", "Valid octal (file permissions)")
	testScan("0o789", "Typo: user likely meant 0o755 or 0o777")
	fmt.Println("   → Digits 0-9 are commonly used, 0o789 is clearly a typo in octal context")

	// Scenario 2: Identifier vs hex literal
	fmt.Println("\nScenario 2: Identifier or hex literal?")
	testScan("0xDEADBEEF", "Valid hex")
	testScan("0xGOOD", "Is this hex+identifier or typo?")
	testScan("GetValue", "Clearly an identifier")
	fmt.Println("   → Letters beyond 'f' are common in identifiers (GetValue, GoLang, etc.)")
	fmt.Println("   → Treating 0xGOOD as 0x + GOOD seems more likely than typo")

	// Scenario 3: What if we changed behavior?
	fmt.Println("\nScenario 3: If we consumed 0xGOOD as one token:")
	fmt.Println("   Input: 0xGOOD")
	fmt.Println("   Current: INT(\"0x\") + error + IDENT(\"GOOD\") → user sees identifier clearly")
	fmt.Println("   Alternative: INT(\"0xGOOD\") + error → identifier disappears into error")
	fmt.Println("   → Current behavior preserves more semantic information")

	// Scenario 4: Real world comparison
	fmt.Println("\nScenario 4: Real world usage patterns:")
	testScan("0644", "Old octal (common in code)")
	testScan("0o644", "New octal (explicit)")
	testScan("0xCAFEBABE", "Hex constant (common)")
	testScan("GetHTTPClient", "Function name (very common)")
	fmt.Println("   → CamelCase identifiers with capital letters are extremely common")
	fmt.Println("   → Hex literals beyond 'f' are relatively rare")
}

func testScan(src, desc string) {
	fmt.Printf("\n   %s: %q\n", desc, src)

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	hasError := false
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		fmt.Printf("      Error: %s\n", msg)
		hasError = true
	}, 0)

	tokens := []string{}
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if lit == "" {
			tokens = append(tokens, tok.String())
		} else {
			tokens = append(tokens, fmt.Sprintf("%s(%q)", tok, lit))
		}
	}
	fmt.Printf("      Tokens: %v\n", tokens)
	if !hasError {
		fmt.Printf("      (No errors)\n")
	}
}
