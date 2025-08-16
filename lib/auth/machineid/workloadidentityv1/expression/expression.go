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
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// Environment in which expressions will be evaluated.
type Environment struct {
	// Attrs will be exposed as top-level variables in the expression language.
	Attrs *workloadidentityv1pb.Attrs

	// SigstorePolicyEvaluator is used when the user calls the `sigstore.policy_satisfied`
	// function in their expression.
	SigstorePolicyEvaluator SigstorePolicyEvaluator
}

// message satisfies messageEnv[T].
func (e *Environment) message() *workloadidentityv1pb.Attrs { return e.Attrs }

var (
	templateExpressionParser = must(expression.NewTraitsExpressionParser[*Environment](templateVars))

	booleanExpressionParser = func() *typical.Parser[*Environment, any] {
		vars := protoMessageVariables[*workloadidentityv1pb.Attrs, *Environment](false)

		// In the context of a boolean expression, we support multi-valued traits
		// so you can use `contains`.
		vars["user.traits"] = typical.DynamicMapFunction(func(env *Environment, key string) (expression.Set, error) {
			traits := make(map[string][]string)
			for _, v := range env.Attrs.GetUser().GetTraits() {
				traits[v.GetKey()] = append(traits[v.GetKey()], v.Values...)
			}
			return expression.NewSet(traits[key]...), nil
		})

		defParserSpec := expression.DefaultParserSpec[*Environment]()
		defParserSpec.Variables = vars
		defParserSpec.Functions[funcNameSigstorePolicySatisfied] = funcSigstorePolicySatisfied

		parser, err := typical.NewParser[*Environment, any](defParserSpec)
		if err != nil {
			panic(fmt.Sprintf("failed to construct parser: %v", err))
		}
		return parser
	}()

	templateVars = func() map[string]typical.Variable {
		vars := protoMessageVariables[*workloadidentityv1pb.Attrs, *Environment](true)

		// In the context of a template, traits must be single-valued and we
		// return an error if they're not. For completeness, it might better if
		// you could say `{{user.traits.foo.contains("bar")}}` or index the
		// values in a template — but we're optimizing for the most common case
		// where you want to interpolate the trait's value.
		vars["user.traits"] = typical.DynamicMapFunction(func(env *Environment, key string) (string, error) {
			traits := make(map[string][]string)
			for _, v := range env.Attrs.GetUser().GetTraits() {
				traits[v.GetKey()] = append(traits[v.GetKey()], v.Values...)
			}

			vals := traits[key]

			switch len(vals) {
			case 1:
				return vals[0], nil
			case 0:
				return "", trace.Errorf("no value for trait: %q", key)
			default:
				return "", trace.Errorf(
					"trait `%s` has multiple values (%s), only single-value traits may be used in a template",
					key,
					strings.Join(vals, ", "),
				)
			}
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
