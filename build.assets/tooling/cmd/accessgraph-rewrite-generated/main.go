/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	accessGraphModule = "github.com/gravitational/access-graph/pkg/api"
	teleportModule    = "github.com/gravitational/teleport/lib/accessgraph/apiclient"
)

type rewriteMode int

const (
	rewriteClient rewriteMode = iota
	rewriteModel
)

func main() {
	if err := rewriteGenerated(); err != nil {
		fail(err)
	}
}

func rewriteGenerated() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}

	files := map[string]rewriteMode{
		"lib/accessgraph/apiclient/client.gen.go":                 rewriteClient,
		"lib/accessgraph/apiclient/models/graph/models.gen.go":    rewriteModel,
		"lib/accessgraph/apiclient/models/jsondiff/models.gen.go": rewriteModel,
		"lib/accessgraph/apiclient/models/logs/models.gen.go":     rewriteModel,
	}

	for relPath, mode := range files {
		path := filepath.Join(root, relPath)
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", relPath, err)
		}

		rewritten, err := rewriteGeneratedFile(source, mode)
		if err != nil {
			return fmt.Errorf("rewrite %s: %w", relPath, err)
		}

		formatted, err := format.Source(rewritten)
		if err != nil {
			return fmt.Errorf("format %s: %w", relPath, err)
		}

		if err := os.WriteFile(path, formatted, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

func rewriteGeneratedFile(source []byte, mode rewriteMode) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	if mode == rewriteClient {
		file.Name.Name = "accessgraph"
	}
	rewriteImports(file, mode)
	if mode == rewriteClient {
		addImport(file, "strconv", "")
	}
	rewriteUUIDSelectors(file)
	if mode == rewriteModel {
		rewriteRuntimeSelectors(file)
	}
	if mode == rewriteClient {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				rewriteBlock(fn.Body)
			}
		}
		if hasStyleParamCall(file) {
			return nil, fmt.Errorf("unconverted StyleParamWithOptions call remains")
		}
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && isTeleportRootModule(data) {
			return dir, nil
		}
		if parent := filepath.Dir(dir); parent == dir {
			return "", fmt.Errorf("could not find teleport go.mod from %s", dir)
		} else {
			dir = parent
		}
	}
}

func isTeleportRootModule(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "module github.com/gravitational/teleport" {
			return true
		}
	}
	return false
}

func rewriteImports(file *ast.File, mode rewriteMode) {
	seen := make(map[string]struct{})
	decls := file.Decls[:0]

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			decls = append(decls, decl)
			continue
		}

		specs := gen.Specs[:0]
		for _, spec := range gen.Specs {
			importSpec := spec.(*ast.ImportSpec)
			path := importPath(importSpec)
			remove := false

			switch path {
			case accessGraphModule + "/models/graph":
				setImportPath(importSpec, teleportModule+"/models/graph")
			case accessGraphModule + "/models/jsondiff":
				setImportPath(importSpec, teleportModule+"/models/jsondiff")
			case accessGraphModule + "/models/logs":
				setImportPath(importSpec, teleportModule+"/models/logs")
			case "github.com/oapi-codegen/runtime/types", teleportModule + "/runtime/types":
				setImportPath(importSpec, "github.com/google/uuid")
				importSpec.Name = nil
			case "github.com/google/uuid":
				importSpec.Name = nil
			case "github.com/oapi-codegen/runtime", teleportModule + "/jsonmerge":
				if mode == rewriteClient {
					remove = true
				} else {
					setImportPath(importSpec, teleportModule+"/jsonmerge")
					importSpec.Name = ast.NewIdent("jsonmerge")
				}
			}
			if remove {
				continue
			}

			key := importKey(importSpec)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			specs = append(specs, importSpec)
		}

		if len(specs) == 0 {
			continue
		}
		gen.Specs = specs
		decls = append(decls, gen)
	}

	file.Decls = decls
}

func addImport(file *ast.File, path string, name string) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec := spec.(*ast.ImportSpec)
			if importPath(importSpec) == path {
				return
			}
		}

		importSpec := &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(path),
			},
		}
		if name != "" {
			importSpec.Name = ast.NewIdent(name)
		}
		gen.Specs = append(gen.Specs, importSpec)
		return
	}
}

func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return spec.Path.Value
	}
	return path
}

func setImportPath(spec *ast.ImportSpec, path string) {
	spec.Path.Value = strconv.Quote(path)
}

func importKey(spec *ast.ImportSpec) string {
	name := ""
	if spec.Name != nil {
		name = spec.Name.Name
	}
	return name + "\x00" + importPath(spec)
}

func rewriteUUIDSelectors(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "UUID" {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "openapi_types" {
			selector.X = ast.NewIdent("uuid")
		}
		return true
	})
}

func rewriteRuntimeSelectors(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "runtime" {
			selector.X = ast.NewIdent("jsonmerge")
		}
		return true
	})
}

func rewriteBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}

	stmts := block.List[:0]
	for i := 0; i < len(block.List); i++ {
		if stmt, ok := rewritePathParamStatement(block.List, i); ok {
			stmts = append(stmts, stmt)
			i += 2
			continue
		}
		if stmt, ok := rewriteQueryParamStatement(block.List[i]); ok {
			stmts = append(stmts, stmt)
			continue
		}

		stmt := block.List[i]
		rewriteStmt(stmt)
		stmts = append(stmts, stmt)
	}
	block.List = stmts
}

func rewritePathParamStatement(stmts []ast.Stmt, index int) (ast.Stmt, bool) {
	if index+2 >= len(stmts) {
		return nil, false
	}

	pathParam, ok := pathParamVar(stmts[index])
	if !ok {
		return nil, false
	}

	call, ok := pathParamAssignment(stmts[index+1], pathParam)
	if !ok || call.style != "simple" || call.location != "ParamLocationPath" {
		return nil, false
	}
	if !isGeneratedErrorCheck(stmts[index+2]) {
		return nil, false
	}

	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(pathParam)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{urlPathEscapeCall(pathValueExpr(call))},
	}, true
}

func pathParamVar(stmt ast.Stmt) (string, bool) {
	declStmt, ok := stmt.(*ast.DeclStmt)
	if !ok {
		return "", false
	}
	genDecl, ok := declStmt.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR || len(genDecl.Specs) != 1 {
		return "", false
	}
	valueSpec, ok := genDecl.Specs[0].(*ast.ValueSpec)
	if !ok || len(valueSpec.Names) != 1 || len(valueSpec.Values) != 0 {
		return "", false
	}
	if ident, ok := valueSpec.Type.(*ast.Ident); !ok || ident.Name != "string" {
		return "", false
	}

	name := valueSpec.Names[0].Name
	if !strings.HasPrefix(name, "pathParam") {
		return "", false
	}
	return name, true
}

func pathParamAssignment(stmt ast.Stmt, pathParam string) (styleParamCall, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return styleParamCall{}, false
	}
	first, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || first.Name != pathParam {
		return styleParamCall{}, false
	}
	second, ok := assign.Lhs[1].(*ast.Ident)
	if !ok || second.Name != "err" {
		return styleParamCall{}, false
	}
	return parseStyleParamCall(assign.Rhs[0])
}

func rewriteQueryParamStatement(stmt ast.Stmt) (ast.Stmt, bool) {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok {
		return nil, false
	}

	call, ok := queryParamInit(ifStmt.Init)
	if !ok || call.style != "form" || call.location != "ParamLocationQuery" {
		return nil, false
	}
	if !isGeneratedErrorCheck(ifStmt) || !isGeneratedParseQueryElse(ifStmt.Else) {
		return nil, false
	}

	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("queryValues"),
				Sel: ast.NewIdent("Add"),
			},
			Args: []ast.Expr{stringExpr(call.name), queryValueExpr(call)},
		},
	}, true
}

func queryParamInit(stmt ast.Stmt) (styleParamCall, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return styleParamCall{}, false
	}
	first, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || first.Name != "queryFrag" {
		return styleParamCall{}, false
	}
	second, ok := assign.Lhs[1].(*ast.Ident)
	if !ok || second.Name != "err" {
		return styleParamCall{}, false
	}

	if call, ok := parseStyleParamCall(assign.Rhs[0]); ok {
		return call, true
	}
	return styleParamCall{}, false
}

func isGeneratedParseQueryElse(stmt ast.Stmt) bool {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || !isGeneratedErrorCheck(ifStmt) {
		return false
	}

	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return false
	}
	first, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || first.Name != "parsed" {
		return false
	}
	second, ok := assign.Lhs[1].(*ast.Ident)
	if !ok || second.Name != "err" {
		return false
	}

	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isSelector(call.Fun, "url", "ParseQuery") || len(call.Args) != 1 {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	if !ok || arg.Name != "queryFrag" {
		return false
	}

	_, ok = ifStmt.Else.(*ast.BlockStmt)
	return ok
}

func isGeneratedErrorCheck(stmt ast.Stmt) bool {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || len(ifStmt.Body.List) != 1 {
		return false
	}
	if _, ok := ifStmt.Body.List[0].(*ast.ReturnStmt); !ok {
		return false
	}

	binary, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}
	left, leftOK := binary.X.(*ast.Ident)
	right, rightOK := binary.Y.(*ast.Ident)
	return leftOK && rightOK && left.Name == "err" && right.Name == "nil"
}

func rewriteStmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		rewriteAssign(stmt)
	case *ast.ExprStmt:
		stmt.X = rewriteExpr(stmt.X)
	case *ast.IfStmt:
		if stmt.Init != nil {
			rewriteSimpleStmt(stmt.Init)
		}
		stmt.Cond = rewriteExpr(stmt.Cond)
		rewriteBlock(stmt.Body)
		rewriteElse(stmt.Else)
	case *ast.ForStmt:
		if stmt.Init != nil {
			rewriteSimpleStmt(stmt.Init)
		}
		if stmt.Cond != nil {
			stmt.Cond = rewriteExpr(stmt.Cond)
		}
		if stmt.Post != nil {
			rewriteSimpleStmt(stmt.Post)
		}
		rewriteBlock(stmt.Body)
	case *ast.RangeStmt:
		stmt.X = rewriteExpr(stmt.X)
		rewriteBlock(stmt.Body)
	case *ast.BlockStmt:
		rewriteBlock(stmt)
	case *ast.ReturnStmt:
		for i, expr := range stmt.Results {
			stmt.Results[i] = rewriteExpr(expr)
		}
	}
}

func rewriteElse(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		rewriteBlock(stmt)
	case *ast.IfStmt:
		rewriteStmt(stmt)
	}
}

func rewriteSimpleStmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		rewriteAssign(stmt)
	case *ast.ExprStmt:
		stmt.X = rewriteExpr(stmt.X)
	}
}

func rewriteAssign(assign *ast.AssignStmt) {
	for i, expr := range assign.Rhs {
		assign.Rhs[i] = rewriteExpr(expr)
	}
}

func rewriteExpr(expr ast.Expr) ast.Expr {
	switch expr := expr.(type) {
	case *ast.CallExpr:
		expr.Fun = rewriteExpr(expr.Fun)
		for i, arg := range expr.Args {
			expr.Args[i] = rewriteExpr(arg)
		}
	case *ast.UnaryExpr:
		expr.X = rewriteExpr(expr.X)
	case *ast.BinaryExpr:
		expr.X = rewriteExpr(expr.X)
		expr.Y = rewriteExpr(expr.Y)
	case *ast.ParenExpr:
		expr.X = rewriteExpr(expr.X)
	case *ast.SelectorExpr:
		expr.X = rewriteExpr(expr.X)
	case *ast.IndexExpr:
		expr.X = rewriteExpr(expr.X)
		expr.Index = rewriteExpr(expr.Index)
	case *ast.SliceExpr:
		expr.X = rewriteExpr(expr.X)
		if expr.Low != nil {
			expr.Low = rewriteExpr(expr.Low)
		}
		if expr.High != nil {
			expr.High = rewriteExpr(expr.High)
		}
		if expr.Max != nil {
			expr.Max = rewriteExpr(expr.Max)
		}
	}
	return expr
}

type styleParamCall struct {
	style    string
	name     string
	value    ast.Expr
	location string
	typ      string
	format   string
}

func parseStyleParamCall(expr ast.Expr) (styleParamCall, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 5 {
		return styleParamCall{}, false
	}
	if !isSelector(call.Fun, "runtime", "StyleParamWithOptions") {
		return styleParamCall{}, false
	}

	style, ok := stringLiteral(call.Args[0])
	if !ok {
		return styleParamCall{}, false
	}
	name, ok := stringLiteral(call.Args[2])
	if !ok {
		return styleParamCall{}, false
	}
	location, typ, format, ok := styleParamOptions(call.Args[4])
	if !ok {
		return styleParamCall{}, false
	}

	return styleParamCall{
		style:    style,
		name:     name,
		value:    call.Args[3],
		location: location,
		typ:      typ,
		format:   format,
	}, true
}

func styleParamOptions(expr ast.Expr) (location string, typ string, format string, ok bool) {
	composite, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", "", "", false
	}

	for _, element := range composite.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "ParamLocation":
			if selector, ok := keyValue.Value.(*ast.SelectorExpr); ok {
				location = selector.Sel.Name
			}
		case "Type":
			typ, _ = stringLiteral(keyValue.Value)
		case "Format":
			format, _ = stringLiteral(keyValue.Value)
		}
	}

	return location, typ, format, location != ""
}

func pathValueExpr(call styleParamCall) ast.Expr {
	if call.format != "uuid" {
		return call.value
	}
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   call.value,
			Sel: ast.NewIdent("String"),
		},
	}
}

func stringConversion(expr ast.Expr) ast.Expr {
	if call, ok := expr.(*ast.CallExpr); ok {
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "string" {
			return expr
		}
	}
	return &ast.CallExpr{
		Fun:  ast.NewIdent("string"),
		Args: []ast.Expr{expr},
	}
}

func queryValueExpr(call styleParamCall) ast.Expr {
	switch {
	case call.format == "date-time":
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   call.value,
				Sel: ast.NewIdent("Format"),
			},
			Args: []ast.Expr{
				&ast.SelectorExpr{
					X:   ast.NewIdent("time"),
					Sel: ast.NewIdent("RFC3339Nano"),
				},
			},
		}
	case call.typ == "integer":
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("strconv"),
				Sel: ast.NewIdent("Itoa"),
			},
			Args: []ast.Expr{call.value},
		}
	case call.typ == "number":
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("strconv"),
				Sel: ast.NewIdent("FormatFloat"),
			},
			Args: []ast.Expr{
				&ast.CallExpr{
					Fun:  ast.NewIdent("float64"),
					Args: []ast.Expr{call.value},
				},
				&ast.BasicLit{Kind: token.CHAR, Value: "'f'"},
				&ast.UnaryExpr{
					Op: token.SUB,
					X:  &ast.BasicLit{Kind: token.INT, Value: "1"},
				},
				&ast.BasicLit{Kind: token.INT, Value: "32"},
			},
		}
	default:
		return stringConversion(call.value)
	}
}

func urlPathEscapeCall(expr ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("url"),
			Sel: ast.NewIdent("PathEscape"),
		},
		Args: []ast.Expr{expr},
	}
}

func isSelector(expr ast.Expr, packageName string, selectorName string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == packageName
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func stringExpr(value string) ast.Expr {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(value),
	}
}

func hasStyleParamCall(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		if call, ok := node.(*ast.CallExpr); ok && isSelector(call.Fun, "runtime", "StyleParamWithOptions") {
			found = true
			return false
		}
		return true
	})
	return found
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
