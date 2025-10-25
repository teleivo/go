package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	fmt.Println("=== Token Position Information ===\n")

	testCases := []string{
		"123xyz",
		"123 xyz",
		"0o789",
	}

	for _, src := range testCases {
		fmt.Printf("Input: %q\n", src)

		var s scanner.Scanner
		fset := token.NewFileSet()
		file := fset.AddFile("test.go", fset.Base(), len(src))

		s.Init(file, []byte(src), func(pos token.Position, msg string) {
			fmt.Printf("  Scanner Error: %s\n", msg)
		}, 0)

		fmt.Println("  Tokens:")
		for {
			pos, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			offset := file.Offset(pos)
			if lit == "" || lit == "\n" {
				fmt.Printf("    [offset %d] %s\n", offset, tok)
			} else {
				fmt.Printf("    [offset %d] %s(%q)\n", offset, tok, lit)
			}
		}
		fmt.Println()
	}

	fmt.Println("=== Analysis ===")
	fmt.Println("123xyz:")
	fmt.Println("  - INT at offset 0, length 3")
	fmt.Println("  - IDENT at offset 3, length 3")
	fmt.Println("  - They are ADJACENT: offset(IDENT) == offset(INT) + len(INT)")
	fmt.Println()
	fmt.Println("123 xyz:")
	fmt.Println("  - INT at offset 0, length 3")
	fmt.Println("  - IDENT at offset 4, length 3")
	fmt.Println("  - They are NOT adjacent: offset(IDENT) > offset(INT) + len(INT)")
	fmt.Println()
	fmt.Println("So the parser COULD detect adjacency, but it doesn't!")
	fmt.Println("The scanner COULD have consumed 123xyz as one invalid token.")
}
