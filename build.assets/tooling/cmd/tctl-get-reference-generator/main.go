// Teleport
// Copyright (C) 2025  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// TODO: getCollectionTypeCases
func getCollectionTypeCases(decls []ast.Decl, targetFuncName string) ([]TypeInfo, error) {
	var typeCases []TypeInfo
	var kindSwitch *ast.SwitchStmt

	for _, d := range decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fd.Name.Name != targetFuncName {
			continue
		}

		// Find the switch statement that evaluates the kind of the
		// input resource.
		for _, b := range fd.Body.List {
			s, ok := b.(*ast.SwitchStmt)
			if !ok {
				continue
			}

			sel, ok := s.Tag.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			if sel.Sel.Name != "Kind" {
				continue
			}

			if kindSwitch != nil {
				return nil, trace.Errorf(
					"expected one switch statement that evaluates resource kinds in %v, but got multiple",
					targetFuncName,
				)
			}
			kindSwitch = s
		}
	}

	for _, c := range kindSwitch.Body.List {
		clause := c.(*ast.CaseClause)
		for _, l := range clause.List {
			sel, ok := l.(*ast.SelectorExpr)
			if !ok {
				return nil, trace.Errorf(
					"in %v, all case clauses in the kind switch statement must be selector expressions",
					targetFuncName,
				)

			}

			typeCases = append(typeCases, TypeInfo{
				Package: sel.X.(*ast.Ident).Name,
				Name:    sel.Sel.Name,
			})
		}
	}

	if kindSwitch == nil {
		return nil, trace.Errorf("function %v does not switch on a resource kind", targetFuncName)
	}

	return typeCases, nil
}

// extractHandlersKeys TODO
func extractHandlersKeys(decls []ast.Decl, targetFuncName string) ([]TypeInfo, error) {
	var handlerKeys []TypeInfo

	for _, d := range decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name != targetFuncName {
			continue
		}

		if len(fd.Body.List) != 1 {
			return nil, trace.Errorf(
				"we expect the function that returns a handler map to include a single body statement, but %v has %v",
				targetFuncName,
				len(fd.Body.List),
			)
		}

		ret := fd.Body.List[0].(*ast.ReturnStmt)
		if len(ret.Results) != 1 {
			return nil, trace.Errorf(
				"we expect the function that returns a handler map return a single value, but %v returns %v",
				targetFuncName,
				len(ret.Results),
			)
		}

		m, ok1 := ret.Results[0].(*ast.CompositeLit)
		var ok2 bool
		if ok1 {
			_, ok2 = m.Type.(*ast.MapType)
		}
		if !ok1 || !ok2 {
			return nil, trace.Errorf(
				"we expect the function that returns a handler map return a map but %v does not",
				targetFuncName,
			)
		}

		for _, e := range m.Elts {
			kv := e.(*ast.KeyValueExpr)
			key := kv.Key.(*ast.SelectorExpr)
			pkg := key.X.(*ast.Ident).Name
			typ := key.Sel.Name
			handlerKeys = append(handlerKeys, TypeInfo{
				Package: pkg,
				Name:    typ,
			})
		}
	}

	return handlerKeys, nil
}

// PackageInfo is used to look up a Go declaration in a map of declaration names
// to resource data.
type PackageInfo struct {
	DeclName    string
	PackageName string
}

// DeclarationInfo includes data about a declaration so the generator can
// convert it into a ReferenceEntry.
type DeclarationInfo struct {
	FilePath    string
	Decl        ast.Decl
	PackageName string
	// Maps the file-scoped name of each import (if given) to the
	// corresponding full package path.
	NamedImports map[string]string
}

type SourceData struct {
	// TypeDecls maps package and declaration names to data that the generator
	// uses to format documentation for dynamic resource fields.
	TypeDecls map[PackageInfo]DeclarationInfo
	// PossibleFuncDecls are declarations that are not import, constant,
	// type or variable declarations.
	PossibleFuncDecls []DeclarationInfo
	// StringAssignments is used to look up the values of constants declared
	// in the source tree.
	StringAssignments map[PackageInfo]string
}

func NewSourceData(rootPath string) (SourceData, error) {
	// All declarations within the source tree. We use this to extract
	// information about dynamic resource fields, which we can look up by
	// package and declaration name.
	typeDecls := make(map[PackageInfo]DeclarationInfo)
	possibleFuncDecls := []DeclarationInfo{}
	stringAssignments := make(map[PackageInfo]string)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.Walk(rootPath, func(currentPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("loading Go source: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		// Open the file so we can pass it to ParseFile. Otherwise,
		// ParseFile always reads from the OS FS, not from fs.
		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			return err
		}

		str, err := GetTopLevelStringAssignments(file.Decls, file.Name.Name)
		if err != nil {
			return err
		}

		for k, v := range str {
			stringAssignments[k] = v
		}

		// Use a relative path from the source directory for cleaner
		// paths
		relDeclPath, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			return err
		}

		// Collect information from each file:
		// - Imported packages and their aliases
		// - Possible function declarations (for identifying relevant
		//   methods later)
		// - Type declarations
		pn := NamedImports(file)
		for _, decl := range file.Decls {
			di := DeclarationInfo{
				Decl:         decl,
				FilePath:     relDeclPath,
				PackageName:  file.Name.Name,
				NamedImports: pn,
			}
			l, ok := decl.(*ast.GenDecl)
			if !ok {
				possibleFuncDecls = append(possibleFuncDecls, di)
				continue
			}
			if len(l.Specs) != 1 {
				continue
			}
			spec, ok := l.Specs[0].(*ast.TypeSpec)
			if !ok {
				continue
			}

			typeDecls[PackageInfo{
				DeclName:    spec.Name.Name,
				PackageName: file.Name.Name,
			}] = DeclarationInfo{
				Decl:         l,
				FilePath:     relDeclPath,
				PackageName:  file.Name.Name,
				NamedImports: pn,
			}
		}
		return nil
	})
	if err != nil {
		return SourceData{}, fmt.Errorf("loading Go source files: %w", err)
	}
	return SourceData{
		TypeDecls:         typeDecls,
		PossibleFuncDecls: possibleFuncDecls,
		StringAssignments: stringAssignments,
	}, nil
}

type NotAGenDeclError struct{}

func (e NotAGenDeclError) Error() string {
	return "the declaration is not a GenDecl"
}

// NamedImports creates a mapping from the provided name of each package import
// to the original package path.
func NamedImports(file *ast.File) map[string]string {
	m := make(map[string]string)
	for _, i := range file.Imports {
		if i.Name == nil {
			continue
		}
		s := strings.Trim(i.Path.Value, "\"")
		p := strings.Split(s, "/")
		// Consumers check the named imports map against the final path
		// segment of a package path.
		if len(p) > 1 {
			s = p[len(p)-1]
		}
		m[i.Name.Name] = s
	}
	return m
}

// GetTopLevelStringAssignments collects all declarations of a var or a const
// within decls that assign a string value. Used to look up the values of these
// declarations.
func GetTopLevelStringAssignments(decls []ast.Decl, pkg string) (map[PackageInfo]string, error) {
	result := make(map[PackageInfo]string)

	// var and const assignments are GenDecls, so ignore any input Decls that
	// don't meet this criterion by making a slice of GenDecls.
	gd := []*ast.GenDecl{}
	for _, d := range decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		gd = append(gd, g)
	}

	// Whether in the "var =" format or "var (" format, each assignment is
	// an *ast.ValueSpec. Collect all ValueSpecs within a GenDecl that
	// declares a var or a const.
	vs := []*ast.ValueSpec{}
	for _, g := range gd {
		if g.Tok != token.VAR && g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			s, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			vs = append(vs, s)
		}
	}

	// Add the name and value of each var/const to the return as long as
	// there is one name and the value is a string literal.
	for _, v := range vs {
		if len(v.Names) != 1 {
			continue
		}
		if len(v.Values) != 1 {
			continue
		}

		l, ok := v.Values[0].(*ast.BasicLit)
		if !ok {
			continue
		}
		if l.Kind != token.STRING {
			continue
		}
		// String literal values are quoted. Remove the quotes so we can
		// compare values downstream.
		result[PackageInfo{
			DeclName:    v.Names[0].Name,
			PackageName: pkg,
		}] = strings.Trim(l.Value, "\"")

	}
	return result, nil
}

// TypeInfo represents the name and package name of an exported Go type. It
// makes no guarantees about whether the type was actually declared within the
// package.
type TypeInfo struct {
	// Go package path (not a file path)
	Package string `yaml:"package"`
	// Name of the type, e.g., Metadata
	Name string `yaml:"name"`
}

// getPackageInfoFromExpr extracts a package name and declaration name from an
// arbitrary expression. If the expression is not an expected kind,
// getPackageInfoFromExpr returns an empty PackageInfo.
func getPackageInfoFromExpr(expr ast.Expr) PackageInfo {
	var gopkg, fldname string
	switch t := expr.(type) {
	case *ast.StarExpr:
		return getPackageInfoFromExpr(t.X)
	case *ast.SelectorExpr:
		// If the type of the field is an *ast.SelectorExpr,
		// it's of the form <package>.<type name>.
		g, ok := t.X.(*ast.Ident)
		if ok {
			gopkg = g.Name
		}
		fldname = t.Sel.Name

	// There's no package, so only assign a name.
	case *ast.Ident:
		fldname = t.Name
	}
	return PackageInfo{
		DeclName:    fldname,
		PackageName: gopkg,
	}
}

// Generate uses the provided user-facing configuration to write the resource
// reference to fs.
func Generate() error {
	// TODO: have this select the correct path.
	// TODO: use the resulting sourceData
	sourceData, err := NewSourceData(".")
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	kindConsts := append(
		getCollectionTypeCases(sourceData.PossibleFuncDecls, "getCollection"),
		extractHandlersKeys(sourceData.PossibleFuncDecls, "Handlers")...,
	)

	return nil
}

func main() {
	err := Generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not generate the `tctl get` reference: %v\n", err)
		os.Exit(1)
	}
}
