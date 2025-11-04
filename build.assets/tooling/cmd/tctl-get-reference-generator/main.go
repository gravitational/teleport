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
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// TODO: getCollectionTypeCases
func getCollectionTypeCases(decls []DeclarationInfo, targetFuncName string) ([]PackageInfo, error) {
	var typeCases []PackageInfo
	var kindSwitch *ast.SwitchStmt

	for _, decl := range decls {
		d := decl.Decl
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

			typeCases = append(typeCases, getPackageInfoFromExpr(sel))
		}
	}

	if kindSwitch == nil {
		return nil, trace.Errorf("function %v does not switch on a resource kind", targetFuncName)
	}

	return typeCases, nil
}

// extractHandlersKeys TODO
func extractHandlersKeys(decls []DeclarationInfo, targetFuncName string) ([]PackageInfo, error) {
	var handlerKeys []PackageInfo

	for _, decl := range decls {
		d := decl.Decl
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
			handlerKeys = append(handlerKeys, getPackageInfoFromExpr(key))
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
	// PossibleFuncDecls are declarations that are not import, constant,
	// type or variable declarations.
	PossibleFuncDecls []DeclarationInfo
	// StringAssignments is used to look up the values of constants declared
	// in the source tree.
	StringAssignments map[PackageInfo]string
}

func parseDeclsFromFile(f io.Reader, rootPath, currentPath string) ([]DeclarationInfo, map[PackageInfo]string, error) {
	var possibleFuncDecls []DeclarationInfo
	stringAssignments := make(map[PackageInfo]string)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	str, err := GetTopLevelStringAssignments(file.Decls, file.Name.Name)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range str {
		stringAssignments[k] = v
	}

	// Use a relative path from the source directory for cleaner
	// paths
	relDeclPath, err := filepath.Rel(rootPath, currentPath)
	if err != nil {
		return nil, nil, err
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
		_, ok := decl.(*ast.GenDecl)
		if !ok {
			possibleFuncDecls = append(possibleFuncDecls, di)
		}
	}

	return possibleFuncDecls, stringAssignments, nil
}

func NewSourceData(rootPath string) (SourceData, error) {
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

		fileDecls, fileAssignments, err := parseDeclsFromFile(f, rootPath, currentPath)
		if err != nil {
			return err
		}

		possibleFuncDecls = append(possibleFuncDecls, fileDecls...)
		for k, v := range fileAssignments {
			stringAssignments[k] = v
		}

		return nil
	})
	if err != nil {
		return SourceData{}, fmt.Errorf("loading Go source files: %w", err)
	}
	return SourceData{
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

// Generate writes the resource reference to w.
func Generate(w io.Writer) error {
	sourceData, err := NewSourceData(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	typeCases, err := getCollectionTypeCases(sourceData.PossibleFuncDecls, "getCollection")
	if err != nil {
		return err
	}

	handlers, err := extractHandlersKeys(sourceData.PossibleFuncDecls, "Handlers")
	if err != nil {
		return err
	}

	for _, p := range append(typeCases, handlers...) {
		c, ok := sourceData.StringAssignments[p]
		if !ok {
			continue
		}

		fmt.Fprintf(w, "- `%v`\n", c)
	}

	return nil
}

func main() {
	var buf bytes.Buffer
	err := Generate(&buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not generate the `tctl get` reference: %v\n", err)
		os.Exit(1)
	}

	io.Copy(os.Stdout, &buf)
}
