/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// EvaluateTraitsMap evaluates expression that must evaluate to either string or Set.
// traitsMap: key is name of the trait and values are list of predicate expressions.
func EvaluateTraitsMap[TEnv any](env TEnv, traitsMap map[string][]string, parseExpression func(input string) (typical.Expression[TEnv, any], error)) (Dict, error) {
	d, err := NewDict()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for key, values := range traitsMap {
		for _, expr := range values {
			e, err := parseExpression(expr)
			if err != nil {
				var u typical.UnknownIdentifierError
				if errors.As(err, &u) {
					id := u.Identifier()
					if id == expr {
						// If the entire expression evaluates to a single unknown
						// identifier, treat it as a string. This is to support rules like
						//   groups: [devs]
						// instead of requiring extra quotes like
						//   groups: ['"devs"']
						d[key] = union(d[key], NewSet(id))
						continue
					}
				}
				return nil, trace.Wrap(err, "error parsing expression: %q", expr)
			}

			result, err := e.Evaluate(env)
			if err != nil {
				return nil, trace.Wrap(err, "error evaluating expression: %q", expr)
			}

			s, err := traitsMapResultToSet(result, expr)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			d[key] = union(d[key], s)
		}
	}
	return d, nil
}
