package reference

import (
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

// shouldProcess indicates whether we should generate reference entries from d,
// that is, whether s has any field types in
func shouldProcess(d resource.DeclarationInfo, types []TypeInfo) bool {
	if len(d.Decl.Specs) == 0 {
		return false
	}

	// Name the section after the first type declaration found. We expect
	// there to be one type spec.
	var t *ast.TypeSpec
	for _, s := range d.Decl.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			continue
		}
		// There is more than one TypeSpec
		if t != nil {
			return false
		}
		t = ts
	}
	if t == nil {
		return false
	}

	str, ok := t.Type.(*ast.StructType)
	if !ok {
		return false
	}

	// Use only the final segment of each desired package path
	// in the comparison, since that is what is preserved in the
	// AST.
	finalTypes := make([]TypeInfo, len(types))
	for i, t := range types {
		segs := strings.Split(t.Package, "/")
		finalTypes[i] = TypeInfo{
			Package: segs[len(segs)-1],
			Name:    t.Name,
		}
	}

	// Only process types with a types.Metadata field, indicating a
	// dynamic resource.
	var m bool
	for _, fld := range str.Fields.List {
		if len(fld.Names) != 1 {
			continue
		}

		// If the field type does not have a package name, it
		// must come from the package where d was declared. This is the
		// initial assumption.
		gotpkg := d.PackageName
		var fldname string
		switch t := fld.Type.(type) {
		case *ast.SelectorExpr:
			// If the type of the field is an *ast.SelectorExpr,
			// it's of the form <package>.<type name>.
			g, ok := t.X.(*ast.Ident)
			if ok {
				gotpkg = g.Name
			}
			fldname = t.Sel.Name

		// There's no package, so only assign a name.
		case *ast.Ident:
			fldname = t.Name
		}

		for _, ti := range finalTypes {
			if gotpkg == ti.Package && fldname == ti.Name {
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
		if !shouldProcess(decl, conf.RequiredTypes) {
			continue
		}
		entries, err := resource.NewFromDecl(decl, allDecls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "issue creating a reference entry for declaration %v.%v in file %v: %v", k.PackageName, k.TypeName, decl.FilePath, err)
			os.Exit(1)
		}

		for pi, e := range entries {
			result[pi] = e
		}
	}

	err = template.Must(template.New("Main reference").Parse(referenceTemplate)).Execute(out, result)
	return nil
}
