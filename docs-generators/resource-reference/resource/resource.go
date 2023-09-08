package resource

import (
	"errors"
	"go/ast"
	"strings"
)

type Resource struct {
	SectionName string
	Description string
	SourcePath  string
	Fields      []Field
	YAMLExample string
}

type Field struct {
	Name        string
	Description string
	Type        string
}

// getTypeSpec returns the type spec to use for further processing. Returns an
// error if there is either no type spec or more than one.
func getTypeSpec(decl *ast.GenDecl) (*ast.TypeSpec, error) {
	if len(decl.Specs) == 0 {
		return nil, errors.New("declaration has no specs")
	}

	// Name the section after the first type declaration found. We expect
	// there to be one type spec.
	var t *ast.TypeSpec
	for _, s := range decl.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if t != nil {
			return nil, errors.New("declaration contains more than one type spec")
		}
		t = ts
	}

	if t == nil {
		return nil, errors.New("no type spec found")
	}

	return t, nil
}

// getSectionName determines how to name a section of the resource reference
// after the provided declaration.
func getSectionName(spec *ast.TypeSpec) string {
	return spec.Name.Name
}

// makeYAMLExample creates an example YAML document illustrating the fields
// within the declaration.
func makeYAMLExample(fields *ast.FieldList) (string, error) {
	// TODO: make the YAML example
	return "", nil
}

// makeFields assembles a slice of human-readable information about fields
// within a Go struct.
func makeFields(fields *ast.FieldList) ([]Field, error) {
	// TODO: Make the field list
	return []Field{}, nil
}

// NewFromDecl creates a Resource object from the provided *GenDecl. filepath is
// the Go source file where the declaration was made, and is used only for
// printing.
func NewFromDecl(decl *ast.GenDecl, filepath string) (Resource, error) {
	ts, err := getTypeSpec(decl)
	if err != nil {
		return Resource{}, err
	}

	str, ok := ts.Type.(*ast.StructType)
	if !ok {
		return Resource{}, errors.New("the declaration is not a struct")
	}

	yml, err := makeYAMLExample(str.Fields)
	if err != nil {
		return Resource{}, err
	}

	fld, err := makeFields(str.Fields)
	if err != nil {
		return Resource{}, err
	}

	section := getSectionName(ts)
	return Resource{
		SectionName: section,
		Description: strings.ReplaceAll(decl.Doc.Text(), "\n", " "),
		SourcePath:  filepath,
		Fields:      fld,
		YAMLExample: yml,
	}, nil
}
