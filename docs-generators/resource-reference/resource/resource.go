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

func NewFromDecl(decl *ast.GenDecl) Resource {
	return Resource{}
}
