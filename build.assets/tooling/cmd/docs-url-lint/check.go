// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Finding describes a single call to a configured error constructor,
// within one of the given targets, whose message doesn't reference
// Teleport documentation.
type Finding struct {
	Pos     token.Pos
	Message string
}

const docsURLSubstring = "goteleport.com/docs"

// checkTargets inspects pass for calls to any of the given constructors,
// within the given targets, returning a Finding for each one that doesn't
// reference Teleport documentation.
func checkTargets(pass *analysis.Pass, targets []Target, constructors []Target) []Finding {
	relevant := targetFunctions(targets, pass.Pkg.Path())
	if len(relevant) == 0 {
		return nil
	}

	constructorNames := errorConstructorNames(constructors)
	consts := packageConstants(pass.Files)

	var findings []Finding
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !relevant.matches(fn) {
				continue
			}

			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				msgArg, calleeName, ok := errorMessageArg(call, constructorNames)
				if !ok {
					return true
				}

				msg, ok := resolveMessage(msgArg, consts)
				if !ok {
					// Neither a plain string literal nor a reference to a
					// resolvable constant (e.g. a local variable, or a
					// concatenated expression); can't verify statically yet.
					return true
				}

				if !strings.Contains(strings.ToLower(msg), docsURLSubstring) {
					findings = append(findings, Finding{
						Pos:     call.Pos(),
						Message: fmt.Sprintf("%s call does not link to Teleport documentation", calleeName),
					})
				}

				return true
			})
		}
	}

	return findings
}

// functionScope describes which receiver(s) a targeted function name is
// restricted to, within one package.
type functionScope struct {
	// anyReceiver is true if at least one Target for this function name
	// left Receiver empty, meaning any receiver (or no receiver at all)
	// matches.
	anyReceiver bool
	// receivers is the set of specific receiver type names that match,
	// used only when anyReceiver is false.
	receivers map[string]bool
}

// targetFunctionScopes maps function names from targets that apply to the
// package at pkgPath to which receiver(s) they're restricted to.
type targetFunctionScopes map[string]functionScope

// matches reports whether fn is in scope: its name must be targeted, and
// if that target restricts to specific receivers, fn's receiver (if any)
// must be one of them.
func (s targetFunctionScopes) matches(fn *ast.FuncDecl) bool {
	scope, ok := s[fn.Name.Name]
	if !ok {
		return false
	}
	if scope.anyReceiver {
		return true
	}
	return scope.receivers[receiverTypeName(fn)]
}

// targetFunctions returns the function names from targets that apply to the
// package at pkgPath, along with which receiver(s) each is restricted to.
func targetFunctions(targets []Target, pkgPath string) targetFunctionScopes {
	fns := make(targetFunctionScopes)
	for _, t := range targets {
		if t.Package != pkgPath {
			continue
		}
		scope := fns[t.Function]
		if t.Receiver == "" {
			scope.anyReceiver = true
		} else {
			if scope.receivers == nil {
				scope.receivers = make(map[string]bool)
			}
			scope.receivers[t.Receiver] = true
		}
		fns[t.Function] = scope
	}
	return fns
}

// receiverTypeName returns the name of fn's receiver type (e.g.
// "ProvisionTokenV2" for "func (p *ProvisionTokenV2) CheckAndSetDefaults()"),
// or "" if fn has no receiver (a plain function, not a method).
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// errorConstructorNames returns the set of function names from
// constructors. The Package field isn't checked yet (see the TODO on
// errorMessageArg), so constructors are currently matched by name alone.
func errorConstructorNames(constructors []Target) map[string]bool {
	names := make(map[string]bool)
	for _, c := range constructors {
		names[c.Function] = true
	}
	return names
}

// errorMessageArg returns the message argument of call and the call's
// callee as written in source (e.g. "trace.BadParameter"), if call is a
// selector call whose name is in constructorNames.
//
// TODO: resolve call.Fun via pass.TypesInfo.Uses to confirm this resolves to
// the configured constructor's Package, not a same-named local function.
func errorMessageArg(call *ast.CallExpr, constructorNames map[string]bool) (arg ast.Expr, calleeName string, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !constructorNames[sel.Sel.Name] || len(call.Args) == 0 {
		return nil, "", false
	}
	return call.Args[0], selectorText(sel), true
}

// selectorText renders sel as written in source, e.g. "trace.BadParameter".
// Falls back to just the method/function name if sel.X isn't a simple
// package identifier (not expected for a package-qualified call like this).
func selectorText(sel *ast.SelectorExpr) string {
	if id, ok := sel.X.(*ast.Ident); ok {
		return id.Name + "." + sel.Sel.Name
	}
	return sel.Sel.Name
}

// stringLiteralValue returns the string value of expr, if expr is a string
// literal.
func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// resolveStringExpr returns the string value of expr, handling either a
// plain string literal or a chain of literals joined by string
// concatenation (e.g. "a" + "b" + "c", as in tsh.go's mlockFailureMessage).
func resolveStringExpr(expr ast.Expr) (string, bool) {
	if v, ok := stringLiteralValue(expr); ok {
		return v, true
	}

	bin, ok := expr.(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		return "", false
	}

	left, ok := resolveStringExpr(bin.X)
	if !ok {
		return "", false
	}
	right, ok := resolveStringExpr(bin.Y)
	if !ok {
		return "", false
	}
	return left + right, true
}

// packageConstants returns a map from constant name to its declared value
// expression, scanned across every file in files. Constants are commonly
// declared in a separate file (e.g. constants.go) from where they're used,
// so this looks across the whole package rather than just one file.
func packageConstants(files []*ast.File) map[string]ast.Expr {
	consts := make(map[string]ast.Expr)
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						consts[name.Name] = vs.Values[i]
					}
				}
			}
		}
	}
	return consts
}

// resolveMessage returns the string value of expr, resolving a reference to
// a package-level constant (via consts) if expr isn't itself a literal.
//
// TODO: resolve call.Fun via pass.TypesInfo.Uses to confirm an identifier
// really refers to a constant in this package, not a shadowing local
// variable with the same name.
func resolveMessage(expr ast.Expr, consts map[string]ast.Expr) (string, bool) {
	if v, ok := resolveStringExpr(expr); ok {
		return v, true
	}

	id, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}

	value, ok := consts[id.Name]
	if !ok {
		return "", false
	}

	return resolveStringExpr(value)
}
