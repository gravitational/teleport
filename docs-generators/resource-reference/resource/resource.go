package resource

import "go/ast"

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

// NewFromDecl creates a Resource object from the provided *GenDecl. filepath is
// the Go source file where the declaration was made, and is used only for
// printing.
func NewFromDecl(decl *ast.GenDecl, filepath string) Resource {
	return Resource{
		SectionName: "",
		Description: decl.Doc.Text(),
		SourcePath:  filepath,
		Fields:      []Field{},
		YAMLExample: "",
	}
}
