/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package typical_test

import (
	"fmt"
	"slices"

	"github.com/gravitational/teleport/lib/utils/typical"
)

type expressionEnv struct {
	traits map[string][]string
	labels func(key string) string
}

func Example() {
	parser, err := typical.NewParser[expressionEnv, bool](typical.ParserSpec[expressionEnv]{
		Variables: map[string]typical.Variable{
			"traits": typical.DynamicVariable(func(e expressionEnv) (map[string][]string, error) {
				return e.traits, nil
			}),
			"labels": typical.DynamicMapFunction(func(e expressionEnv, key string) (string, error) {
				return e.labels(key), nil
			}),
			"true":  true,
			"false": false,
		},
		Functions: map[string]typical.Function{
			"contains": typical.BinaryFunction[expressionEnv](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
			"contains_all": typical.BinaryVariadicFunction[expressionEnv](func(list []string, strs ...string) (bool, error) {
				for _, str := range strs {
					if !slices.Contains(list, str) {
						return false, nil
					}
				}
				return true, nil
			}),
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	env := expressionEnv{
		traits: map[string][]string{
			"groups": {"devs", "security"},
		},
		labels: func(key string) string {
			if key == "owner" {
				return "devs"
			}
			return ""
		},
	}

	for _, expr := range []string{
		`contains(traits["groups"], labels["owner"])`,
		`contains_all(traits["groups"], "devs", "admins")`,
		`contains(traits["groups"], false)`,
	} {
		parsed, err := parser.Parse(expr)
		if err != nil {
			fmt.Println("parse error:", err)
			continue
		}
		match, err := parsed.Evaluate(env)
		if err != nil {
			fmt.Println("evaluation error:", err)
			continue
		}
		fmt.Println(match)
	}
	// Output:
	// true
	// false
	// parse error: parsing expression
	// 	parsing second argument to (contains)
	// 		expected type string, got value (false) with type (bool)
}
