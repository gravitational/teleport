package main

import (
	"flag"
	"fmt"
	"gen-resource-ref/resource"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
	// result := make(map[resource.PackageInfo]resource.ReferenceEntry)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.Walk(*src, func(path string, info fs.FileInfo, err error) error {
		// There is an error with the path, so we can't load Go source
		if err != nil {
			return err
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, info.Name(), nil, parser.ParseComments)
		if err != nil {
			return err
		}
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
				Decl:        l,
				FilePath:    info.Name(),
				PackageName: file.Name.Name,
			}

		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't load Go source files: %v", err)
		os.Exit(1)
	}

	// TODO: If a struct type has a types.Metadata field, construct a
	// Resource from the struct type and insert it into the final data map
	// (unless a Resource already exists there).

	// TODO: Process the fields of the struct type using the rules described
	// below, looking up declaration data from the declaration map.
}
