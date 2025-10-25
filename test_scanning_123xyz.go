package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	src := "123xyz"
	fmt.Printf("Scanning: %q\n\n", src)

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		fmt.Printf("ERROR at pos %d: %s\n", pos.Offset, msg)
	}, 0)

	fmt.Println("Tokens:")
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		fmt.Printf("  %s (pos=%d, lit=%q)\n", tok, file.Offset(pos), lit)
	}

	fmt.Println("\n=== Why no error? ===")
	fmt.Println("Step by step:")
	fmt.Println("1. Scanner sees '1' → enters number scanning mode")
	fmt.Println("2. digits() function: for isDecimal(s.ch) || s.ch == '_'")
	fmt.Println("3. Consumes '1', '2', '3' (all satisfy isDecimal)")
	fmt.Println("4. Reaches 'x': isDecimal('x') = false → loop STOPS")
	fmt.Println("5. Returns token INT with literal \"123\"")
	fmt.Println("6. Next scan starts at 'x'")
	fmt.Println("7. Scanner sees 'x' → enters identifier scanning mode")
	fmt.Println("8. Consumes 'xyz' as valid identifier")
	fmt.Println("\nResult: Two valid tokens, no error!")
	fmt.Println("  - INT(\"123\") - valid decimal number")
	fmt.Println("  - IDENT(\"xyz\") - valid identifier")
}
