package resource

import (
	"errors"
	"go/ast"
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

// getSectionName determines how to name a section of the resource reference
// after the provided declaration.
func getSectionName(decl *ast.GenDecl) (string, error) {
	if len(decl.Specs) == 0 {
		return "", errors.New("declaration has no specs")
	}

	// Name the section after the first type declaration found. We expect
	// there to be one type spec.
	var t string
	for _, s := range decl.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if t != "" {
			return "", errors.New("declaration contains more than one type spec")
		}
		t = ts.Name.Name
	}

	if t == "" {
		return "", errors.New("no type spec found")
	}
	return t, nil
}

// NewFromDecl creates a Resource object from the provided *GenDecl. filepath is
// the Go source file where the declaration was made, and is used only for
// printing.
func NewFromDecl(decl *ast.GenDecl, filepath string) (Resource, error) {
	section, err := getSectionName(decl)
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		SectionName: section,
		Description: decl.Doc.Text(),
		SourcePath:  filepath,
		Fields:      []Field{},
		YAMLExample: "",
	}, nil
}
