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

func TestBasicSat(t *testing.T) {
	// (A or B) and (not A or not B)
	state := &state{
		clauses: []node{
			&nodeAnd{
				left: &nodeOr{
					left:  &nodeIdentifier{key: "A"},
					right: &nodeIdentifier{key: "B"},
				},
				right: &nodeOr{
					left: &nodeNot{
						left: &nodeIdentifier{key: "A"},
					},
					right: &nodeNot{
						left: &nodeIdentifier{key: "B"},
					},
				},
			},
		},
	}

	require.True(t, dpll(state))
	require.Equal(t, state.assignments, []assignment{
		{key: "A", value: true},
		{key: "B", value: false},
	})
}
