// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package typical_test

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib/utils/typical"
)

type expressionEnv struct {
	traits map[string][]string
	labels func(key string) string
}

func Example() {
	parser, err := typical.NewParser[expressionEnv, bool](typical.ParserSpec{
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
