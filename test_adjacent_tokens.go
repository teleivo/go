package main

import (
	"fmt"
	"go/parser"
	"go/token"
)

func main() {
	fmt.Println("=== Does the parser catch adjacent INT + IDENT? ===\n")

	testCases := []struct {
		code string
		desc string
	}{
		{"x := 123xyz", "INT directly adjacent to IDENT (no space)"},
		{"x := 123 xyz", "INT separated from IDENT (with space)"},
		{"x := 0o789", "Invalid octal (caught by scanner)"},
		{"x := 123", "Valid INT alone"},
		{"x := 123+xyz", "INT + operator + IDENT"},
		{"x := 123\nxyz", "INT and IDENT on different lines"},
	}

	for _, tc := range testCases {
		fmt.Printf("%s\n", tc.desc)
		fmt.Printf("  Code: %q\n", tc.code)

		src := "package main\nfunc main() {\n\t" + tc.code + "\n}"
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "test.go", src, 0)

		if err != nil {
			fmt.Printf("  Result: ERROR - %s\n", err)
		} else {
			fmt.Printf("  Result: NO ERROR (parsed successfully!)\n")
		}
		fmt.Println()
	}

	fmt.Println("=== Key Question ===")
	fmt.Println("Does the parser know that INT and IDENT were adjacent with no whitespace?")
	fmt.Println("Or does it just see them as two separate tokens?")
}
