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

func TestSATSimple(t *testing.T) {
	// 1 or 2
	clause1 := newClause(newSet([]int{1, 2}), make(set[int], 0))

	// !1 or !2
	clause2 := newClause(make(set[int], 0), newSet([]int{1, 2}))

	// 2
	clause3 := newClause(newSet([]int{2}), make(set[int], 0))

	instance := newInstance([]clause{clause1, clause2, clause3})
	assignments, err := instance.solve()
	require.Equal(t, clauseNoError, err)
	require.Equal(t, map[int]bool{1: false, 2: true}, assignments)
}

func TestSATIntAdd(t *testing.T) {
	theory := newNumTheory()
	r1 := integer(theory, "r1")
	constantEquals(theory, r1, 4)
	r3 := integer(theory, "r3")
	constantEquals(theory, r3, 15)
	r2 := integer(theory, "r2")
	equals(theory, add(theory, r1, r2).out, r3)

	clauses := theory.finish()
	instance := newInstance(clauses)
	assignments, err := instance.solve()
	require.Equal(t, clauseNoError, err)
	mapped := make(map[string]bool)
	for k, v := range assignments {
		mapped[theory.var_names[k]] = v
	}
	require.Equal(t, map[string]bool{"add.carry.0": false, "add.carry.1": false, "add.carry.2": false, "add.carry.3": false, "add.out.0": true, "add.out.1": true, "add.out.2": true, "add.out.3": true, "fa.0": true, "fa.1": false, "fa.2": false, "false": false, "r1.0": false, "r1.1": false, "r1.2": true, "r1.3": false, "r2.0": true, "r2.1": true, "r2.2": false, "r2.3": true, "r3.0": true, "r3.1": true, "r3.2": true, "r3.3": true}, mapped)
}

func TestUnsatIntEq(t *testing.T) {
	theory := newNumTheory()
	r1 := integer(theory, "r1")
	constantEquals(theory, r1, 4)
	r2 := integer(theory, "r2")
	constantEquals(theory, r2, 7)
	equals(theory, r1, r2)

	clauses := theory.finish()
	instance := newInstance(clauses)
	_, err := instance.solve()
	require.Equal(t, clauseNoError, err)
}
