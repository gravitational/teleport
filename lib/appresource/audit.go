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
	goparser "go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// Hint is a near-miss reason recorded by deny_hint: the code and
// reason of a wrapped condition that was reached and evaluated to
// false. Under &&, that is the near-miss where the checks to its left
// passed but this one did not.
type Hint struct {
	Code   string
	Reason string
}

// validateAuditCode checks an allow or deny code. A valid code is 1 to
// 256 bytes of [a-z0-9_] and does not start with the reserved
// teleport_ prefix.
func validateAuditCode(code string) error {
	for _, r := range code {
		legal := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !legal {
			return trace.BadParameter("code %q must contain only [a-z0-9_]", code)
		}
	}
	if len(code) < 1 || len(code) > 256 {
		return trace.BadParameter("code %q must be 1 to 256 characters", code)
	}
	if strings.HasPrefix(code, "teleport_") {
		return trace.BadParameter("code %q must not start with the reserved teleport_ prefix", code)
	}
	return nil
}

// validateAuditCodes rejects an illegal code in an allow_code("...",
// ...) or deny_hint("...", ...) call whose code is a string constant,
// so an illegal constant code fails at compile time rather than per
// request. A dynamic code is validated at evaluation instead. Both
// wrappers return the value of the expression they wrap, so a
// misplaced call can record a stray code or hint but can never flip
// the boolean result, and no placement check is needed.
//
// It also rejects a constant reason over the reason byte cap, so the
// expression surface bounds a reason the same way the sugared
// allow_reason and deny_reason_hint fields do. A dynamic reason is not
// length-checked.
//
// The predicate uses the same Go expression syntax the engine parses,
// so this reuses go/parser to walk the AST. An expression that does
// not parse is left to the engine, which reports the parse error.
func validateAuditCodes(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	var bad error
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if !isIdentCall(call, "allow_code") && !isIdentCall(call, "deny_hint") {
			return true
		}
		if code, ok := stringLiteral(call.Args[0]); ok {
			if err := validateAuditCode(code); err != nil && bad == nil {
				bad = err
			}
		}
		if len(call.Args) < 2 {
			return true
		}
		if reason, ok := stringLiteral(call.Args[1]); ok && len(reason) > maxReasonBytes && bad == nil {
			bad = trace.BadParameter("reason is %d bytes, over the %d byte cap", len(reason), maxReasonBytes)
		}
		return true
	})
	return trace.Wrap(bad)
}

// stringLiteral returns the value of a string-constant argument.
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

// isIdentCall reports whether call is a bare name(...) call, such as
// allow_code(...). It does not match a selector call like path.match(...).
func isIdentCall(call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}
