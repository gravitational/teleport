/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

const funcSigstorePolicySatisfied = "sigstore.policy_satisfied"

// SigstorePolicyNames finds the names of all Sigstore policies referred to by
// the given WorkloadIdentity rule expression, so that we can load an evaluate
// them before evaluating the expression.
//
// It parses the expression and walks the AST collecting the string literal
// arguments to the `sigstore.policy_satisfied` function.
//
// Example:
//
//	SigstorePolicyNames(`sigstore.policy_satisfied("foo") || sigstore.policy_satisfied("bar")`) -> ["foo", "bar"]
func SigstorePolicyNames(rule string) ([]string, error) {
	policyNames := make([]string, 0)

	var visit func(ast.Expr) error
	visit = func(expr ast.Expr) error {
		// This switch statement is based on the one in the predicate library
		// and supports the same subset of Go's AST nodes.
		switch node := expr.(type) {
		case *ast.CallExpr:
			funcName, err := funcName(node)
			if err != nil {
				return trace.Wrap(err, "determining function name")
			}

			if funcName == funcSigstorePolicySatisfied {
				// If this is a call to `sigstore.policy_satisfied`, collect the
				// string literal arguments.
				args, err := stringLitArgs(node)
				if err != nil {
					return err
				}
				policyNames = append(policyNames, args...)
			} else {
				// For all other function calls, evaluate the arguments in case one
				// is a sub-expression containing a call to `sigstore.policy_satisfied`.
				for _, arg := range node.Args {
					if err := visit(arg); err != nil {
						return err
					}
				}
			}
		case *ast.BinaryExpr:
			if err := visit(node.X); err != nil {
				return err
			}
			if err := visit(node.Y); err != nil {
				return err
			}
		case *ast.IndexExpr:
			if err := visit(node.X); err != nil {
				return err
			}
			if err := visit(node.Index); err != nil {
				return err
			}
		case *ast.SelectorExpr:
			if err := visit(node.X); err != nil {
				return err
			}
			if err := visit(node.Sel); err != nil {
				return err
			}
		case *ast.UnaryExpr:
			return visit(node.X)
		case *ast.ParenExpr:
			return visit(node.X)
		case *ast.BasicLit, *ast.Ident:
			// Nothing to do, these don't contain any sub-expressions.
		default:
			return trace.BadParameter("%T is not supported", expr)
		}
		return nil
	}

	expr, err := parser.ParseExpr(rule)
	if err != nil {
		return nil, trace.Wrap(err, "parsing rule expression: %q", rule)
	}
	if err := visit(expr); err != nil {
		return nil, err
	}
	return policyNames, nil
}

func funcName(call *ast.CallExpr) (string, error) {
	var nameParts func(expr ast.Expr) ([]string, error)
	nameParts = func(expr ast.Expr) ([]string, error) {
		switch node := expr.(type) {
		case *ast.SelectorExpr:
			a, err := nameParts(node.X)
			if err != nil {
				return nil, err
			}
			b, err := nameParts(node.Sel)
			if err != nil {
				return nil, err
			}
			return append(a, b...), nil
		case *ast.Ident:
			return []string{node.Name}, nil
		default:
			return nil, trace.BadParameter("%T is not supported", expr)
		}
	}

	parts, err := nameParts(call.Fun)
	if err != nil {
		return "", err
	}
	return strings.Join(parts, "."), nil
}

func stringLitArgs(call *ast.CallExpr) ([]string, error) {
	args := make([]string, 0, len(call.Args))
	for idx, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok {
			continue
		}
		if lit.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, trace.Wrap(err, "failed to unquote string literal %q (argument %d)", value, idx)
		}
		args = append(args, value)
	}
	return args, nil
}
