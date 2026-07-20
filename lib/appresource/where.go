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

// Request encodes the elements of the HTTP request a where clause is
// evaluated against. It is deliberately not an *http.Request, to make
// clear which fields matter for evaluation.
type Request struct {
	Method string
}

// Identity is the caller a where clause is evaluated against. It is
// deliberately not a tlsca.Identity, to make clear which fields matter
// for evaluation.
type Identity struct {
	Name   string
	Roles  []string
	Traits map[string][]string
}

// Env holds the values one where clause evaluation reads.
type Env struct {
	Request  Request
	Identity Identity
}

// Where is a compiled where clause. Only CompileWhere returns a usable
// value. Evaluation writes nothing back to a Where, so a single Where can
// serve concurrent requests.
type Where struct {
	expression typical.Expression[Env, bool]
}

// CompileWhere parses and type-checks a where clause.
func CompileWhere(expr string) (*Where, error) {
	expression, err := whereParser.Parse(expr)
	if err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("compiling where clause %q", expr), err)
	}
	return &Where{expression: expression}, nil
}

// Evaluate reports whether the where clause matches the environment. On
// error it reports no match.
func (w *Where) Evaluate(env Env) (bool, error) {
	if err := validateEnv(env); err != nil {
		return false, trace.Wrap(err)
	}
	match, err := w.expression.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return match, nil
}

// validMethods are the HTTP methods a where clause evaluation accepts.
var validMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE"}

// validateEnv rejects an environment a where clause must not authorize,
// such as a request method outside the canonical HTTP method list.
func validateEnv(env Env) error {
	if !slices.Contains(validMethods, env.Request.Method) {
		return trace.BadParameter("unsupported HTTP method %q", env.Request.Method)
	}
	return nil
}

// whereParser is the shared cached parser for where clauses.
var whereParser = mustNewWhereParser()

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
			"user.traits": typical.DynamicMapFunction(func(e Env, key string) ([]string, error) {
				return e.Identity.Traits[key], nil
			}),
			"request.method": typical.DynamicVariable(func(e Env) (string, error) {
				return e.Request.Method, nil
			}),
		},
		Functions: map[string]typical.Function{
			// set builds a set from its string arguments for contains membership
			// tests. set and contains match the functions of the same names in
			// the role where-clause language, services.NewWhereParser.
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
