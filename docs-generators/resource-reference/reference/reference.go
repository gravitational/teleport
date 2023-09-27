package reference

import (
	"errors"
	"fmt"
	"gen-resource-ref/resource"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Intended to be executed with a []ReferenceEntry
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

type TypeInfo struct {
	// Go package path (not a file path)
	Package string `json:"package"`
	// Name of the type, e.g., Metadata
	Name string `json:"name"`
}

type GeneratorConfig struct {
	RequiredTypes []TypeInfo `json:"required_types"`
	// Path to the root of the Go project directory
	SourcePath string `json:"source"`
	// Path of the resource reference
	DestinationPath string `json:"destination"`
}

// shouldProcess indicates whether we should generate reference entries from s,
// that is, whether s has any field types in
func shouldProcess(s *ast.StructType, types []TypeInfo) bool {
	// Only process types with a types.Metadata field, indicating a
	// dynamic resource.
	var m bool
	for _, fld := range s.Fields.List {
		if len(fld.Names) != 1 {
			continue
		}
		i, ok := fld.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		g, ok := i.X.(*ast.Ident)
		if !ok {
			continue
		}

		// TODO: What if the package name is empty because we're in the
		// desired package already?

		for _, ti := range types {
			// Use only the final segment of each desired package path
			// in the comparison, since that is what is preserved in the
			// AST.
			segs := strings.Split(ti.Package, "/")
			pkg := segs[len(segs)-1]

			if g.Name == pkg && i.Sel.Name == ti.Name {
				m = true
				break
			}
		}
	}

	return m
}

func Generate(out io.Writer, conf GeneratorConfig) error {
	allDecls := make(map[resource.PackageInfo]resource.DeclarationInfo)
	result := make(map[resource.PackageInfo]resource.ReferenceEntry)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.WalkDir(conf.SourcePath, func(path string, info fs.DirEntry, err error) error {
		if info.IsDir() {
			return nil
		}
		// There is an error with the path, so we can't load Go source
		if err != nil {
			return err
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
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

	for k, decl := range allDecls {
		// TODO: The code that checks whether a decl is a struct comes
		// from resource.getRawTypes. Refactor so we don't repeat the
		// struct-processing logic.
		if len(decl.Decl.Specs) == 0 {
			continue
		}

		var err error
		// Name the section after the first type declaration found. We expect
		// there to be one type spec.
		var t *ast.TypeSpec
		for _, s := range decl.Decl.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if t != nil {
				err = errors.New("declaration contains more than one type spec")
				continue
			}
			t = ts
		}
		// TODO: consider skipping instead of exiting with an error.
		if err != nil {
			return fmt.Errorf("type %v.%v in %v has more than one type spec", k.PackageName, k.TypeName, decl.FilePath)
		}

		// TODO: consider skipping instead of exiting with an error.
		if t == nil {
			return fmt.Errorf("type %v.%v in %v has no type spec", k.PackageName, k.TypeName, decl.FilePath)
		}

		str, ok := t.Type.(*ast.StructType)
		if !ok {
			continue
		}

		if !shouldProcess(str, conf.RequiredTypes) {
			continue
		}
		entries, err := resource.NewFromDecl(decl, allDecls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "issue creating a reference entry for declaration %v.%v in file %v", k.PackageName, k.TypeName, decl.FilePath)
			os.Exit(1)
		}

		for pi, e := range entries {
			result[pi] = e
		}
	}

	err = template.Must(template.New("Main reference").Parse(referenceTemplate)).Execute(out, result)
	return nil
}
