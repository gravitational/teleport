/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parse

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/gravitational/trace"
)

// maxASTDepth is the maximum depth of the AST that func walk will traverse.
// The limit exists to protect against DoS via malicious inputs.
const maxASTDepth = 1000

func parse(exprStr string) (Expr, error) {
	parsedExpr, err := parser.ParseExpr(exprStr)
	if err != nil {
		return nil, trace.BadParameter("failed to parse: %q, error: %s", exprStr, err)
	}
	expr, err := walk(parsedExpr, 0)
	fmt.Printf("parsed %s\n", expr)
	return expr, trace.Wrap(err)
}

// walk will walk the ast tree and create our own ast.
func walk(node ast.Node, depth int) (Expr, error) {
	if depth > maxASTDepth {
		return nil, trace.LimitExceeded("expression exceeds the maximum allowed depth")
	}

	switch e := node.(type) {
	case *ast.CallExpr:
		fields, args, err := parseCallExpr(e, depth)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return buildCallExpr(fields, args)
	case *ast.IndexExpr:
		fields, err := parseIndexExpr(e)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return buildVarExpr(fields)
	case *ast.SelectorExpr:
		fields, err := parseSelectorExpr(e, depth, []string{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return buildVarExpr(fields)
	case *ast.Ident:
		return buildVarExpr([]string{e.Name})
	case *ast.BasicLit:
		value, err := fetchStringLit(e)
		if err != nil {
			return nil, trace.BadParameter("unexpected literal: %s", err)
		}
		return buildStringLit(value)
	default:
		return nil, trace.BadParameter("%T is not supported", e)
	}
}

func parseCallExpr(e *ast.CallExpr, depth int) ([]string, []Expr, error) {
	var fields []string
	switch call := e.Fun.(type) {
	case *ast.Ident:
		fields = append(fields, call.Name)
	case *ast.SelectorExpr:
		// Selector expression looks like email.local(parameter)
		namespace, err := fetchIdentifier(call.X)
		if err != nil {
			return nil, nil, trace.BadParameter("unexpected namespace in selector: %s", err)
		}
		fields = append(fields, namespace, call.Sel.Name)
	default:
		return nil, nil, trace.BadParameter("unexpected function type %T", e.Fun)
	}

	args := make([]Expr, 0, len(e.Args))
	fmt.Printf("parseCallExpr (%v) %d %v\n", fields, len(e.Args), e.Args)
	for i := range e.Args {
		arg, err := walk(e.Args[i], depth+1)
		fmt.Printf("parseCallExpr for (%v) %v %v\n", fields, arg, err)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		args = append(args, arg)
	}
	fmt.Printf("parseCallExpr return (%v) %v\n", fields, args)
	return fields, args, nil
}

func parseSelectorExpr(e *ast.SelectorExpr, depth int, fields []string) ([]string, error) {
	if depth > maxASTDepth {
		return nil, trace.LimitExceeded("expression exceeds the maximum allowed depth")
	}
	fields = append([]string{e.Sel.Name}, fields...)
	switch l := e.X.(type) {
	case *ast.SelectorExpr:
		return parseSelectorExpr(l, depth+1, fields)
	case *ast.Ident:
		fields = append([]string{l.Name}, fields...)
		return fields, nil
	default:
		return nil, trace.BadParameter("unsupported selector type: %T", l)
	}
}

func parseIndexExpr(e *ast.IndexExpr) ([]string, error) {
	namespace, err := fetchIdentifier(e.X)
	if err != nil {
		return nil, trace.BadParameter("unexpected namespace in index: %s", err)
	}
	name, err := fetchStringLit(e.Index)
	if err != nil {
		return nil, trace.BadParameter("unexpected name in index: %s", err)
	}
	return []string{namespace, name}, nil
}

func fetchIdentifier(e ast.Node) (string, error) {
	v, ok := e.(*ast.Ident)
	if !ok {
		return "", trace.BadParameter("expected identifier, got: %T", e)
	}
	return v.Name, nil
}

func fetchStringLit(e ast.Node) (string, error) {
	v, ok := e.(*ast.BasicLit)
	if !ok {
		return "", trace.BadParameter("expected identifier, got: %T", e)
	}
	if v.Kind != token.STRING {
		return "", trace.BadParameter("expected string literal")
	}

	value, err := strconv.Unquote(v.Value)
	if err != nil {
		return "", trace.BadParameter("failed to unquote string literal: %s, error: %s", v.Value, err)
	}
	return value, nil
}
