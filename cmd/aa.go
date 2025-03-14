package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
)

func findMatchingFunctions(dir string) error {
	// Set up a new file set for parsing Go files.
	fset := token.NewFileSet()

	// Walk through all files in the directory.
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory.
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Process only Go files.
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			// Parse the Go file.
			node, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if err != nil {
				return err
			}

			// Type-check the parsed file.
			conf := types.Config{Importer: nil, Error: func(err error) {}}
			pkg, err := conf.Check(path, fset, []*ast.File{node}, nil)
			if err != nil {
				return err
			}

			// Traverse the AST of the parsed file.
			ast.Inspect(node, func(n ast.Node) bool {
				// Look for function declarations.
				if funcDecl, ok := n.(*ast.FuncDecl); ok {
					sig, ok := pkg.Scope().Lookup(funcDecl.Name.Name).Type().(*types.Signature)
					if !ok {
						return true
					}

					// Check function parameters.
					params := sig.Params()
					if params.Len() != 3 {
						return true
					}

					// Check first parameter is context.Context.
					if !isContextType(params.At(0).Type()) {
						return true
					}

					// Check second parameter is int.
					if !isIntType(params.At(1).Type()) {
						return true
					}

					// Check third parameter is string.
					if !isStringType(params.At(2).Type()) {
						return true
					}

					// Check function results.
					results := sig.Results()
					if results.Len() != 3 {
						return true
					}

					// Check first return type is a slice.
					if !isSliceType(results.At(0).Type()) {
						return true
					}

					// Check second return type is string.
					if !isStringType(results.At(1).Type()) {
						return true
					}

					// Check third return type is error.
					if !isErrorType(results.At(2).Type()) {
						return true
					}

					// If all checks pass, print the location of the function declaration.
					fmt.Printf("Found matching function %s at %s\n", funcDecl.Name.Name, fset.Position(funcDecl.Pos()))
				}
				return true
			})
		}
		return nil
	})

	return err
}

// Helper function to check if a type is context.Context.
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

// Helper function to check if a type is int.
func isIntType(t types.Type) bool {
	_, ok := t.(*types.Basic)
	return ok && t.String() == "int"
}

// Helper function to check if a type is string.
func isStringType(t types.Type) bool {
	_, ok := t.(*types.Basic)
	return ok && t.String() == "string"
}

// Helper function to check if a type is error.
func isErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == "error"
}

// Helper function to check if a type is a slice.
func isSliceType(t types.Type) bool {
	_, ok := t.(*types.Slice)
	return ok
}

func main() {
	// Specify the directory to search for functions.
	dir := "/Users/marek/go/src/github.com/gravitational/teleport"

	// Find all functions with the specified signature.
	err := findMatchingFunctions(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

