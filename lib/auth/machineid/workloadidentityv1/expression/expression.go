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
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
	"github.com/gravitational/trace"
)

var (
	templateExpressionParser = must(expression.NewTraitsExpressionParser[*workloadidentityv1pb.Attrs](templateVars))
	booleanExpressionParser  = must(expression.NewTraitsExpressionParser[*workloadidentityv1pb.Attrs](expressionVars))

	templateVars = func() map[string]typical.Variable {
		vars := protoMessageVariables[*workloadidentityv1pb.Attrs]()

		// In the context of a template, traits must be single-valued and we
		// return an error if they're not. For completeness, it might better if
		// you could say `{{user.traits.foo.contains("bar")}}` or index the
		// values in a template — but we're optimizing for the most common case
		// where you want to interpolate the trait's value.
		vars["user.traits"] = typical.DynamicMapFunction(func(env *workloadidentityv1pb.Attrs, key string) (string, error) {
			traits := make(map[string][]string)
			for _, v := range env.GetUser().GetTraits() {
				traits[v.GetKey()] = append(traits[v.GetKey()], v.Values...)
			}

			vals := traits[key]

			switch len(vals) {
			case 0:
				return "", nil
			case 1:
				return vals[0], nil
			default:
				return "", trace.Errorf("trait %s has multiple values, only single-value traits may be used in a template", key)
			}
		})
		return vars
	}()

	expressionVars = func() map[string]typical.Variable {
		vars := protoMessageVariables[*workloadidentityv1pb.Attrs]()

		// In the context of a binary expression, we support multi-valued traits
		// so you can use `contains`.
		vars["user.traits"] = typical.DynamicMapFunction(func(env *workloadidentityv1pb.Attrs, key string) (expression.Set, error) {
			traits := make(map[string][]string)
			for _, v := range env.GetUser().GetTraits() {
				traits[v.GetKey()] = append(traits[v.GetKey()], v.Values...)
			}
			return expression.NewSet(traits[key]...), nil
		})
		return vars
	}()
)

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
