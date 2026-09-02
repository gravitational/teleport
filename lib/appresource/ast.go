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

package appresource

import (
	"go/ast"
	"go/token"
	"strconv"
)

// isPathMatch returns true when call is a path.match(...) call.
func isPathMatch(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "match" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "path"
}

// isIdentCall returns true when call is name(...) with no dot in name,
// such as allow_code(...) but not path.match(...).
func isIdentCall(call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

// auditCallName returns the name of a bare allow_code or deny_hint call.
// For any other call it returns the empty string.
func auditCallName(call *ast.CallExpr) string {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || (id.Name != "allow_code" && id.Name != "deny_hint") {
		return ""
	}
	return id.Name
}

// isAuditCall returns true when call is a bare allow_code or deny_hint call
// with its full three arguments.
func isAuditCall(call *ast.CallExpr) bool {
	return auditCallName(call) != "" && len(call.Args) == 3
}

// stringLiteral returns the value of a string-literal argument. For any
// other expression it returns false.
func stringLiteral(arg ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(arg).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
