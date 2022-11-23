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
	equals(theory, add(theory, r1, r2), r3)

	clauses := theory.finish()
	instance := newInstance(clauses)
	assignments, err := instance.solve()
	require.Equal(t, clauseNoError, err)
	require.Equal(t, 11, r2.assignedValue(assignments))
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
	require.Equal(t, clauseUnsatisfiable, err)
}

func TestSATForwardMul(t *testing.T) {
	theory := newNumTheory()
	r1 := integer(theory, "r1")
	constantEquals(theory, r1, 3)
	r2 := integer(theory, "r2")
	constantEquals(theory, r2, 3)
	r3 := integer(theory, "r3")
	equals(theory, mul(theory, r1, r2), r3)

	clauses := theory.finish()
	instance := newInstance(clauses)
	assignments, err := instance.solve()
	require.Equal(t, clauseNoError, err)
	mapped := make(map[string]bool)
	for k, v := range assignments {
		mapped[theory.var_names[k]] = v
	}
	require.Equal(t, map[string]bool{}, mapped)
	require.Equal(t, 9, r3.assignedValue(assignments))
}
