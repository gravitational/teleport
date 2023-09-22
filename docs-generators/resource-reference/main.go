package main

import (
	"flag"
	"fmt"
	"gen-resource-ref/resource"
	"go/ast"
	"os"

	"golang.org/x/tools/go/packages"
)

const referenceTemplate string = `{{ range . }}
## {{ .SectionName }}

{{ .Description }}

{/*Automatically generated from: {{ .SourcePath}}*/}

|Field Name|Description|Type|
|---|---|---|
{{ range .Fields }}
|.Name|.Description|.Type|
{{ end }} 

{{ .YAMLExample }}
{{ end }}`

func main() {
	src := flag.String("source", ".", "the project directory in which to parse Go packages")
	flag.Parse()

	allDecls := make(map[resource.PackageInfo]resource.DeclarationInfo)
	result := make(map[resource.PackageInfo]resource.ReferenceEntry)

	pkgs, err := packages.Load(&packages.Config{
		Dir: *src,
	}, "./...")

	if err != nil {
		fmt.Fprintf(os.Stderr, "can't load Go source files: %v", err)
		os.Exit(1)
	}

	// Populate the map of all GenDecls in the source.
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			for _, decl := range file.Decls {
				l, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				if len(l.Specs) != 1 {
					continue
				}
				spec, ok := l.Specs[0].(*ast.TypeSpec)
				if !ok {
					continue
				}

				allDecls[resource.PackageInfo{
					TypeName:    spec.Name.Name,
					PackageName: file.Name.Name,
				}] = resource.DeclarationInfo{
					Decl: l,
					// TODO: Get the file path by walking
					// the directory and loading each file
					// individually. Otherwise, no way to
					// get a file path from an ast.File.
					//					FilePath:    fmt.Sprintf("myfile%v.go", n),
					PackageName: file.Name.Name,
				}

			}
		}
	}

	// TODO: If a struct type has a types.Metadata field, construct a
	// Resource from the struct type and insert it into the final data map
	// (unless a Resource already exists there).

	// TODO: Process the fields of the struct type using the rules described
	// below, looking up declaration data from the declaration map.
}
