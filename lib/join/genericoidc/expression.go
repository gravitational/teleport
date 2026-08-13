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

package genericoidc

import (
	"fmt"
	"maps"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// Environment in which expressions will be evaluated.
type Environment struct {
	// Claims will be exposed under the `claims` field.
	Claims map[string]any
}

var booleanExpressionParser = func() *typical.Parser[*Environment, bool] {
	spec := expression.DefaultParserSpec[*Environment]()
	spec.GetUnknownIdentifier = func(env *Environment, fields []string) (any, error) {
		if len(fields) == 0 {
			return nil, trace.BadParameter("cannot get empty field")
		}

		if fields[0] != "claims" {
			return nil, trace.BadParameter("identifier %q is not defined", fields[0])
		}

		return getByFields(env.Claims, fields[1:])
	}

	// Add (overwrite) `set()` with an enhanced variant that can ingest and
	// flatten []string. The built-in `set()` only supports `string` values as
	// variadic args and there's no "foo..." operator equivalent, meaning it
	// can't convert []string -> Set, and unfortunately, essentially all
	// built-in predicate functions operate on Sets and not lists, with no
	// coercion.
	// This enhanced variant can wrap list variables (e.g. parsed lists in
	// claims documents) in addition to plain strings, or other sets. It's a
	// simple opt-in override (just changes the underlying impl from
	// `expression.NewSet()`) to avoid changing behavior for other predicate
	// uses.
	maps.Copy(spec.Functions, map[string]typical.Function{
		"set": typical.UnaryVariadicFunction[*Environment](
			func(args ...any) (expression.Set, error) {
				return expression.NewFlattenedSet(args...)
			}),
	})

	parser, err := typical.NewParser[*Environment, bool](spec)
	if err != nil {
		panic(fmt.Sprintf("failed to construct parser: %v", err))
	}
	return parser
}()

// getByFields attempts to fetch the value within `parent` by navigating
// sequentially through the fields named in `fields`.
func getByFields(parent map[string]any, fields []string) (any, error) {
	var field any = parent

	for i, key := range fields {
		identifier := strings.Join(fields[0:i+1], ".")

		// first, make sure `field` is actually a map before we try accessing
		// its children
		fieldAsMap, ok := field.(map[string]any)
		if !ok {
			return nil, trace.BadParameter("field at %q cannot have children", identifier)
		}

		// now try to find the child value
		child, ok := fieldAsMap[key]
		if !ok {
			return nil, trace.BadParameter("field not found: %s", identifier)
		}

		field = child
	}

	return field, nil
}

// evaluateExpression evaluates the given predicate expression using the
// `booleanExpressionParser` and provided environment.
func evaluateExpression(expr string, env *Environment) (bool, error) {
	e, err := booleanExpressionParser.Parse(expr)
	if err != nil {
		return false, trace.Wrap(err, "parsing expression: %s", expr)
	}

	rsp, err := e.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err, "evaluating expression: %s", expr)
	}

	return rsp, nil
}
