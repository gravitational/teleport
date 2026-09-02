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

// validMethods are the HTTP methods a where clause evaluation accepts.
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
var whereParser = mustNewParser(whereSpec())

// Request encodes the elements of the HTTP request a where clause is
// evaluated against.
type Request struct {
	Method string
	Path   string
}

// Identity is the caller a where clause is evaluated against.
type Identity struct {
	Name   string
	Roles  []string
	Traits map[string][]string
}

// Env holds the values one where clause evaluation reads. Request and
// Identity are deliberately not [http.Request] and tlsca.Identity, to make
// clear which fields matter for evaluation.
type Env struct {
	Request  Request
	Identity Identity
	result   *Result // set by evaluateExpression for one evaluation
	tokens   []Token // the path tokenized by NewEnv, shared by every rule
}

// Where is a compiled where clause. Only CompileWhere returns a usable
// value. Evaluation writes nothing back to a Where, so a single Where can
// serve concurrent requests. The caller must not mutate the slices or
// map in Env during an evaluation.
type Where struct {
	expression typical.Expression[Env, bool]
}

// NewEnv builds the environment a where clause or an
// app_resources_expressions entry is evaluated against. NewEnv tokenizes
// the path once. A malformed path, a path containing an encoded slash
// (%2F), or an unsupported method is rejected before any rule is
// evaluated.
func NewEnv(request Request, identity Identity) (Env, error) {
	if err := validateMethod(request.Method); err != nil {
		return Env{}, trace.Wrap(err)
	}
	tokens, err := Tokenize(request.Path)
	if err != nil {
		return Env{}, trace.Wrap(err)
	}
	// No rule can match an encoded slash (%2F) in this version.
	if slices.ContainsFunc(tokens, Token.hasEncodedSlash) {
		return Env{}, trace.BadParameter("path contains an encoded slash (%%2F)")
	}
	return Env{Request: request, Identity: identity, tokens: tokens}, nil
}

// CompileWhere parses and type-checks a where clause.
func CompileWhere(expr string) (*Where, error) {
	if err := validateWhere(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	expression, err := whereParser.Parse(expr)
	if err != nil {
		// The aggregate classifies the result as BadParameter for the
		// caller and keeps typical's typed error for errors.As. Parse does
		// not return a BadParameter for every failure.
		return nil, trace.NewAggregate(trace.BadParameter("compiling where clause %q", expr), err)
	}
	return &Where{expression: expression}, nil
}

// Evaluate matches the where clause against the environment and returns the
// outcome in Result.Value.
func (w *Where) Evaluate(env Env) (Result, error) {
	return evaluateExpression(w.expression, env)
}

// validateMethod rejects a request method outside the canonical HTTP
// method list.
func validateMethod(method string) error {
	if !slices.Contains(validMethods, method) {
		return trace.BadParameter("unsupported HTTP method %q", method)
	}
	return nil
}

// mustNewParser builds a parser from spec and panics if the spec is invalid.
func mustNewParser(spec typical.ParserSpec[Env]) *typical.CachedParser[Env, bool] {
	p, err := typical.NewCachedParser[Env, bool](spec)
	if err != nil {
		panic(trace.Wrap(err, "building an app resource parser (this is a bug)"))
	}
	return p
}

// whereSpec returns the parser spec of the where clause language.
// expressionSpec extends it, so a binding or function added here reaches
// both parsers.
func whereSpec() typical.ParserSpec[Env] {
	return typical.ParserSpec[Env]{
		// Only a vars.<name> read parses as an unknown identifier. It is
		// string-typed, so a read outside a string position fails at parse.
		GetUnknownIdentifierVariable: func(fields []string) (typical.Variable, error) {
			if len(fields) != 2 || fields[0] != "vars" {
				return nil, trace.NotFound("unknown identifier %q", strings.Join(fields, "."))
			}
			name := fields[1]
			return typical.DynamicVariable(func(e Env) (string, error) {
				if e.result == nil {
					return "", trace.BadParameter("internal error: evaluating vars.%s without an evaluation result", name)
				}
				v, ok := e.result.vars[name]
				if !ok {
					return "", trace.BadParameter("vars.%s is read but not bound by a matched path", name)
				}
				return v, nil
			}), nil
		},
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
	}
}
