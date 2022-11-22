// Copyright 2022 Gravitational, Inc
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

package predicate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testCase struct {
	name        string
	clause      node
	sat         bool
	assignments []assignment
}

func TestAssign(t *testing.T) {
	cases := []testCase{
		{
			name: "(a or b) and (not a or not b)",
			clause: &nodeAnd{
				left: &nodeOr{
					left:  &nodeIdentifier{key: "a"},
					right: &nodeIdentifier{key: "b"},
				},
				right: &nodeOr{
					left: &nodeNot{
						left: &nodeIdentifier{key: "a"},
					},
					right: &nodeNot{
						left: &nodeIdentifier{key: "b"},
					},
				},
			},
			sat: true,
			assignments: []assignment{
				{key: "a", value: true},
				{key: "b", value: false},
			},
		},
		{
			name: "(a or b) and (not a or not b) and b",
			clause: &nodeAnd{
				left: &nodeOr{
					left:  &nodeIdentifier{key: "a"},
					right: &nodeIdentifier{key: "b"},
				},
				right: &nodeAnd{
					left: &nodeOr{
						left: &nodeNot{
							left: &nodeIdentifier{key: "a"},
						},
						right: &nodeNot{
							left: &nodeIdentifier{key: "b"},
						},
					},
					right: &nodeIdentifier{key: "b"},
				},
			},
			sat: true,
			assignments: []assignment{
				{key: "a", value: false},
				{key: "b", value: true},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newState(c.clause)
			require.Equal(t, c.sat, dpll(s))

			if c.sat {
				require.Equal(t, c.assignments, s.assignments)
			}
		})
	}
}
