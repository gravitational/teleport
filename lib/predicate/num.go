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

const bitCount = 4

type numTheory struct {
	counter   int
	clauses   []clause
	vars      set[int]
	var_true  int
	var_false int
}

func newNumTheory() *numTheory {
	t := &numTheory{
		vars: make(set[int]),
	}

	t.var_true = t.addVar()
	t.var_false = t.addVar()
	return t
}

func (t *numTheory) addVar() int {
	v := t.counter
	t.vars[v] = struct{}{}
	t.counter++
	return v
}

func (t *numTheory) addClause(positive set[int], negative set[int]) {
	t.clauses = append(t.clauses, newClause(positive, negative))
}

func (t *numTheory) finish() []clause {
	return t.clauses
}

type integerS struct {
	bits []int
}

func integer(theory *numTheory) *integerS {
	bits := make([]int, 0)
	for i := 0; i < bitCount; i++ {
		bits = append(bits, theory.addVar())
	}

	return &integerS{bits}
}

func constantEquals(theory *numTheory, x *integerS, value int) {
	for j := 0; j < bitCount; j++ {
		bit := (value >> j) & 1
		if bit == 1 {
			theory.addClause(newSet([]int{x.bits[j]}), newSet[int](nil))
		} else {
			theory.addClause(newSet[int](nil), newSet([]int{x.bits[j]}))
		}
	}
}

func equals(theory *numTheory, x *integerS, y *integerS) {
	for j := 0; j < bitCount; j++ {
		theory.addClause(newSet([]int{x.bits[j]}), newSet([]int{y.bits[j]}))
		theory.addClause(newSet([]int{y.bits[j]}), newSet([]int{x.bits[j]}))
	}
}

type additionS struct {
	out     *integerS
	carries *integerS
}

func addition(theory *numTheory, a *integerS, b *integerS) *additionS {
	bits := integer(theory)
	carries := integer(theory)

	previous_carry := theory.var_false
	for j := 0; j < bitCount; j++ {
		current_carry := carries.bits[j]
		full_adder(theory, a.bits[j], b.bits[j], previous_carry, bits.bits[j], current_carry)
		previous_carry = current_carry
	}

	return &additionS{bits, carries}
}

func xor_gate(theory *numTheory, a int, b int, out int) {
	theory.addClause(newSet([]int{a, b}), newSet([]int{out}))
	theory.addClause(newSet([]int{a, out}), newSet([]int{b}))
	theory.addClause(newSet([]int{b, out}), newSet([]int{a}))
	theory.addClause(newSet[int](nil), newSet([]int{a, b, out}))
}

func or_gate(theory *numTheory, a int, b int, out int) {
	theory.addClause(newSet([]int{out}), newSet([]int{a}))
	theory.addClause(newSet([]int{out}), newSet([]int{b}))
	theory.addClause(newSet([]int{a, b}), newSet([]int{out}))
}

func and_gate(theory *numTheory, a int, b int, out int) {
	theory.addClause(newSet([]int{a}), newSet([]int{out}))
	theory.addClause(newSet([]int{b}), newSet([]int{out}))
	theory.addClause(newSet([]int{out}), newSet([]int{a, b}))
}

func full_adder(theory *numTheory, a int, b int, c int, out int, carry_out int) {
	fa0 := theory.addVar()
	xor_gate(theory, a, b, fa0)
	xor_gate(theory, c, fa0, out)
	fa1 := theory.addVar()
	fa2 := theory.addVar()
	and_gate(theory, a, b, fa1)
	and_gate(theory, c, fa0, fa2)
	or_gate(theory, fa1, fa2, carry_out)
}
