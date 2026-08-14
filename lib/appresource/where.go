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
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// validMethods are the HTTP methods app access authorizes, in upper case:
// the methods a rule may name and the methods an evaluation accepts on a
// request. They are the request methods of RFC 9110 plus PATCH (RFC 5789)
// and less CONNECT. A CONNECT request targets an authority rather than a
// slash-path, so the tokenizer rejects it and a rule naming it could only
// ever be a dead rule.
var validMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
	http.MethodTrace,
}

// whereParser is the shared cached parser for where clauses.
var whereParser = mustNewWhereParser()

// Request encodes the elements of the HTTP request a where clause is
// evaluated against.
type Request struct {
	Method string
}

// Identity is the caller a where clause is evaluated against.
type Identity struct {
	Name   string
	Roles  []string
	Traits map[string][]string
}

// Env holds the values one where clause evaluation reads, and the state
// the audit wrappers record into. Request and Identity are deliberately
// not [http.Request] and tlsca.Identity, to make clear which fields
// matter for evaluation.
type Env struct {
	Request  Request
	Identity Identity
	// state collects what the audit wrappers record. It is unexported
	// because only an evaluation that intends to read the record back
	// needs it, and it is nil for an Env a caller built itself. The
	// wrappers tolerate a nil state, so a where clause that calls one
	// still evaluates.
	state *evalState
}

// evalState holds the side effects of one evaluation for the caller. It
// is held by pointer so the same instance is observed across the whole
// expression tree, even though Env is passed by value. On error the
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

// withAuditState returns a copy of the environment carrying a fresh
// audit state, so concurrent evaluations never share recorded codes or
// hints. The rule layer builds one per evaluation.
func (e Env) withAuditState() Env {
	e.state = &evalState{}
	return e
}

// Where is a compiled where clause. Only CompileWhere returns a usable
// value. Evaluation writes nothing back to a Where, so a single Where can
// serve concurrent requests. The caller must not mutate the slices or
// map in Env during an evaluation.
type Where struct {
	expression typical.Expression[Env, bool]
}

// NewEnv builds the environment a where clause is evaluated against and
// rejects a request no rule may authorize. Callers build it once per
// request, before matching any rule, because a rule without a where
// clause never reaches Evaluate.
func NewEnv(request Request, identity Identity) (Env, error) {
	if err := validateMethod(request.Method); err != nil {
		return Env{}, trace.Wrap(err)
	}
	return Env{Request: request, Identity: identity}, nil
}

// CompileWhere parses and type-checks a where clause.
func CompileWhere(expr string) (*Where, error) {
	expression, err := whereParser.Parse(expr)
	if err != nil {
		// The aggregate classifies the result as BadParameter for the
		// caller and keeps typical's typed error for errors.As. Parse does
		// not return a BadParameter for every failure.
		return nil, trace.NewAggregate(trace.BadParameter("compiling where clause %q", expr), err)
	}
	return &Where{expression: expression}, nil
}

// Evaluate reports whether the where clause matches the environment. The
// result is only meaningful when the error is nil.
func (w *Where) Evaluate(env Env) (bool, error) {
	if err := validateMethod(env.Request.Method); err != nil {
		return false, trace.Wrap(err)
	}
	match, err := w.expression.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return match, nil
}

// validateMethod rejects a request method outside the canonical HTTP
// method list. NewEnv runs it at the request boundary and Evaluate
// repeats it, so an environment a caller assembled itself still cannot
// authorize such a request.
func validateMethod(method string) error {
	if !slices.Contains(validMethods, method) {
		return trace.BadParameter("unsupported HTTP method %q", method)
	}
	return nil
}

// mustNewWhereParser builds the where clause parser and panics if the
// parser spec is invalid or the expression cache cannot be built.
func mustNewWhereParser() *typical.CachedParser[Env, bool] {
	p, err := typical.NewCachedParser[Env, bool](typical.ParserSpec[Env]{
		Variables: map[string]typical.Variable{
			// true and false are bound because typical has no bool literal.
			"true":  true,
			"false": false,
			"user.name": typical.DynamicVariable(func(e Env) (string, error) {
				return e.Identity.Name, nil
			}),
			"user.roles": typical.DynamicVariable(func(e Env) ([]string, error) {
				return e.Identity.Roles, nil
			}),
			// A key the identity does not have reads as an empty list, as in
			// the role where-clause language, so a mistyped key under a
			// negation matches every caller.
			"user.traits": typical.DynamicMapFunction(func(e Env, key string) ([]string, error) {
				return e.Identity.Traits[key], nil
			}),
			"request.method": typical.DynamicVariable(func(e Env) (string, error) {
				return e.Request.Method, nil
			}),
		},
		Functions: map[string]typical.Function{
			// allow_code records an audit code and reason and returns the
			// wrapped boolean, so it never flips the result. The record is
			// committed only when the wrapped expression is true. When
			// several allow_code calls fire on one evaluation, the last one
			// wins.
			"allow_code": typical.TernaryFunctionWithEnv(func(e Env, code, reason string, expr bool) (bool, error) {
				if err := validateAuditCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if expr && e.state != nil {
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
			"deny_hint": typical.TernaryFunctionWithEnv(func(e Env, code, reason string, expr bool) (bool, error) {
				if err := validateAuditCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if !expr && e.state != nil {
					e.state.denyHints = append(e.state.denyHints, Hint{Code: code, Reason: reason})
				}
				return expr, nil
			}),
			// set and contains are named after the functions in the role
			// where-clause language, services.NewWhereParser.
			"set": typical.UnaryVariadicFunction[Env](func(args ...string) ([]string, error) {
				return args, nil
			}),
			"contains": typical.BinaryFunction[Env](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
			"lower": typical.UnaryFunction[Env](func(s string) (string, error) {
				return strings.ToLower(s), nil
			}),
			"upper": typical.UnaryFunction[Env](func(s string) (string, error) {
				return strings.ToUpper(s), nil
			}),
			"has_prefix": typical.BinaryFunction[Env](func(s, prefix string) (bool, error) {
				return strings.HasPrefix(s, prefix), nil
			}),
			"has_suffix": typical.BinaryFunction[Env](func(s, suffix string) (bool, error) {
				return strings.HasSuffix(s, suffix), nil
			}),
			"has_substring": typical.BinaryFunction[Env](func(s, substr string) (bool, error) {
				return strings.Contains(s, substr), nil
			}),
		},
	})
	if err != nil {
		panic(trace.Wrap(err, "building the where clause parser (this is a bug)"))
	}
	return p
}

// compilePredicate parses and type-checks a predicate, and runs the
// compile-time audit code validation. Unlike CompileWhere it accepts the
// full predicate language, including the audit wrappers.
func compilePredicate(expr string) (typical.Expression[Env, bool], error) {
	pred, err := whereParser.Parse(expr)
	if err != nil {
		return nil, trace.BadParameter("compiling predicate %q: %v", expr, err)
	}
	if err := validateAuditCodes(expr); err != nil {
		return nil, trace.Wrap(err, "compiling predicate %q", expr)
	}
	return pred, nil
}
