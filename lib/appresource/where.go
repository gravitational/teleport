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
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// Request is the HTTP request subject a predicate evaluates against.
type Request struct {
	Method string
}

// Identity is the caller subject a predicate evaluates against.
type Identity struct {
	Name   string
	Roles  []string
	Traits map[string][]string
}

// env is the evaluation environment for one predicate evaluation. A
// fresh env, and therefore a fresh state, is built per request, so
// concurrent evaluations never share recorded codes or hints.
type env struct {
	request Request
	user    Identity
	state   *evalState
}

// evalState holds the side effects of one evaluation for the caller. It
// is held by pointer so the same instance is observed across the whole
// expression tree, even though env is passed by value. On error the
// state may be partially populated and must be discarded. allowCode is
// meaningful only when the evaluation returned true, and denyHints only
// when it returned false.
type evalState struct {
	// allowCode and allowReason hold the last successful allow_code call.
	allowCode   string
	allowReason string
	// denyHints records deny_hint calls in evaluation order.
	denyHints []Hint
}

// predicate is a parsed, type-checked app-access predicate ready to evaluate.
type predicate = typical.Expression[env, bool]

// parser is the shared, cached predicate parser. It registers the
// app-access bindings on top of the generic typical parser.
var parser = mustNewParser()

func mustNewParser() *typical.CachedParser[env, bool] {
	p, err := typical.NewCachedParser[env, bool](typical.ParserSpec[env]{
		Variables: map[string]typical.Variable{
			// true and false bind the bare literals, so a predicate of
			// just "true" (allow every request) or "false" (deny every
			// request) parses as an identifier. The engine has no native
			// boolean literal.
			"true":  true,
			"false": false,
			"user.name": typical.DynamicVariable(func(e env) (string, error) {
				return e.user.Name, nil
			}),
			"user.roles": typical.DynamicVariable(func(e env) ([]string, error) {
				return e.user.Roles, nil
			}),
			"user.traits": typical.DynamicMapFunction(func(e env, key string) ([]string, error) {
				return e.user.Traits[key], nil
			}),
			"request.method": typical.DynamicVariable(func(e env) (string, error) {
				return e.request.Method, nil
			}),
		},
		Functions: map[string]typical.Function{
			// allow_code records an audit code and reason and returns the
			// wrapped boolean, so it never flips the result. The record is
			// committed only when the wrapped expression is true. When
			// several allow_code calls fire on one evaluation, the last one
			// wins.
			"allow_code": typical.TernaryFunctionWithEnv(func(e env, code, reason string, expr bool) (bool, error) {
				if err := validateAuditCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if expr {
					e.state.allowCode = code
					e.state.allowReason = reason
				}
				return expr, nil
			}),
			// deny_hint records a near-miss hint and returns the wrapped
			// boolean, so it never flips the result. The hint is committed
			// only when the call is reached and the wrapped expression is
			// false. Under &&, that is the near-miss where the conditions on
			// its left matched but this one did not.
			"deny_hint": typical.TernaryFunctionWithEnv(func(e env, code, reason string, expr bool) (bool, error) {
				if err := validateAuditCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if !expr {
					e.state.denyHints = append(e.state.denyHints, Hint{Code: code, Reason: reason})
				}
				return expr, nil
			}),
			// List helpers, matching the functions of the same names in the
			// role where-clause language.
			"set": typical.UnaryVariadicFunction[env](func(args ...string) ([]string, error) {
				return args, nil
			}),
			"contains": typical.BinaryFunction[env](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
			// equals compares two strings, matching the role where-clause
			// function. It is a synonym for the == operator.
			"equals": typical.BinaryFunction[env](func(a, b string) (bool, error) {
				return a == b, nil
			}),
			// String helpers for case-insensitive comparisons in a where
			// clause, such as contains(set("get", "head"),
			// lower(request.method)).
			"lower": typical.UnaryFunction[env](func(s string) (string, error) {
				return strings.ToLower(s), nil
			}),
			"upper": typical.UnaryFunction[env](func(s string) (string, error) {
				return strings.ToUpper(s), nil
			}),
			// Substring helpers. has_substring is named apart from contains,
			// which is list membership.
			"has_prefix": typical.BinaryFunction[env](func(s, prefix string) (bool, error) {
				return strings.HasPrefix(s, prefix), nil
			}),
			"has_suffix": typical.BinaryFunction[env](func(s, suffix string) (bool, error) {
				return strings.HasSuffix(s, suffix), nil
			}),
			"has_substring": typical.BinaryFunction[env](func(s, substr string) (bool, error) {
				return strings.Contains(s, substr), nil
			}),
		},
	})
	if err != nil {
		panic(trace.Wrap(err, "building app-access predicate parser (this is a bug)"))
	}
	return p
}

// compilePredicate parses and type-checks a predicate string and runs
// the compile-time audit code validation.
func compilePredicate(expr string) (predicate, error) {
	pred, err := parser.Parse(expr)
	if err != nil {
		return nil, trace.Wrap(err, "compiling rule predicate %q", expr)
	}
	if err := validateAuditCodes(expr); err != nil {
		return nil, trace.Wrap(err, "compiling rule predicate %q", expr)
	}
	return pred, nil
}

// newEnv builds a fresh evaluation environment for one request.
func newEnv(request Request, identity Identity) env {
	return env{
		request: request,
		user:    identity,
		state:   &evalState{},
	}
}
