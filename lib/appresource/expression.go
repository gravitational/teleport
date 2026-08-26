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
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// expressionParser parses app_resources_expressions entries.
var expressionParser = mustNewParser(expressionSpec())

// expressionSpec returns the parser spec for an app_resources_expressions
// entry. expressionSpec is a clean superset of whereSpec. Therefore a
// compiled and valid "where" expression can always be evaluated by
// [evaluateExpression].
func expressionSpec() typical.ParserSpec[Env] {
	spec := whereSpec()
	// allow_code records the audit code and reason if the given expression
	// is true, and returns the expression unchanged. A later true call
	// overrides the results of an earlier one.
	spec.Functions["allow_code"] = typical.TernaryFunctionWithEnv(func(e Env, code, reason string, expr bool) (bool, error) {
		if e.record == nil {
			return false, trace.BadParameter("evaluating allow_code without an audit record (this is a bug)")
		}
		if expr {
			e.record.AllowCode = code
			e.record.AllowReason = clamp(reason, maxReasonBytes)
		}
		return expr, nil
	})
	// deny_hint records a hint if the given expression is false, and
	// returns the expression unchanged. Each false call appends its hint.
	spec.Functions["deny_hint"] = typical.TernaryFunctionWithEnv(func(e Env, code, reason string, expr bool) (bool, error) {
		if e.record == nil {
			return false, trace.BadParameter("evaluating deny_hint without an audit record (this is a bug)")
		}
		if !expr && len(e.record.DenyHints) < maxHints {
			e.record.DenyHints = append(e.record.DenyHints, Hint{Code: code, Reason: clamp(reason, maxReasonBytes)})
		}
		return expr, nil
	})
	return spec
}

// compileExpression parses and type-checks one app_resources_expressions
// entry.
func compileExpression(expr string) (typical.Expression[Env, bool], error) {
	if strings.TrimSpace(expr) == "" {
		return nil, trace.BadParameter("an app_resources_expressions entry cannot be empty")
	}
	if len(expr) > maxExpressionBytes {
		return nil, trace.BadParameter("expression is %d bytes, over the %d byte maximum", len(expr), maxExpressionBytes)
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("compiling expression %q", expr), err)
	}
	expression, err := expressionParser.ParseAST(parsed)
	if err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("compiling expression %q", expr), err)
	}
	if err := validateAuditLiterals(parsed); err != nil {
		return nil, trace.Wrap(err, "compiling expression %q", expr)
	}
	return expression, nil
}

// evaluateExpression evaluates one compiled expression against env. The
// returned Result keeps the allow code and reason when the expression
// evaluated to true and the deny hints otherwise. evaluateExpression is
// used for desugared expressions and sugared "where" clauses.
func evaluateExpression(expression typical.Expression[Env, bool], env Env) (Result, error) {
	if err := validateMethod(env.Request.Method); err != nil {
		return Result{}, trace.Wrap(err)
	}
	var result Result
	env.record = &result.AuditRecord
	value, err := expression.Evaluate(env)
	if err != nil {
		return Result{}, trace.Wrap(err)
	}
	result.Value = value
	if value {
		result.AuditRecord.DenyHints = nil
	} else {
		result.AuditRecord.AllowCode = ""
		result.AuditRecord.AllowReason = ""
	}
	return result, nil
}

// validateAuditLiterals checks the code and any literal reason of every
// allow_code and deny_hint call in the parsed expression. The code must be
// a string literal and pass validateAuditCode.
func validateAuditLiterals(parsed ast.Expr) error {
	var err error
	ast.Inspect(parsed, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fnName, ok := auditCallName(call)
		if !ok {
			return true // not an allow_code or deny_hint call
		}
		code, ok := stringLiteral(call.Args[0])
		if !ok {
			err = trace.BadParameter("the code argument of %s must be a string literal", fnName)
			return false
		}
		err = validateAuditCode(code)
		if err != nil {
			return false
		}
		if reason, ok := stringLiteral(call.Args[1]); ok {
			err = validateReason(reason)
			if err != nil {
				return false
			}
		}
		return true
	})
	return trace.Wrap(err)
}

// clamp truncates s to at most limit bytes, cutting on a rune boundary.
func clamp(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
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

// auditCallName returns the name of a bare allow_code or deny_hint call.
// For any other call it returns false.
func auditCallName(call *ast.CallExpr) (string, bool) {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || (id.Name != "allow_code" && id.Name != "deny_hint") {
		return "", false
	}
	return id.Name, true
}
