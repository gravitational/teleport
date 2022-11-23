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

import "fmt"

const bitCount = 8

type numTheory struct {
	counter   int
	clauses   []clause
	vars      set[int]
	var_names map[int]string
	var_true  int
	var_false int
}

func newNumTheory() *numTheory {
	t := &numTheory{
		vars:      make(set[int]),
		var_names: make(map[int]string),
	}

	t.var_true = t.addVar("true")
	t.var_false = t.addVar("false")
	return t
}

func (t *numTheory) addVar(name string) int {
	v := t.counter
	t.vars[v] = struct{}{}
	t.counter++
	t.var_names[v] = name
	return v
}

func (t *numTheory) addClause(positive, negative set[int]) {
	t.clauses = append(t.clauses, newClause(positive, negative))
}

func (t *numTheory) finish() []clause {
	return t.clauses
}

type integerS struct {
	bits []int
}

func integer(theory *numTheory, name string) *integerS {
	bits := make([]int, 0)
	for i := 0; i < bitCount; i++ {
		bits = append(bits, theory.addVar(fmt.Sprintf("%s.%d", name, i)))
	}

	return &integerS{bits}
}

func (i *integerS) assignedValue(assignments map[int]bool) int {
	value := 0
	for j := 0; j < bitCount; j++ {
		if assignments[i.bits[j]] {
			value |= 1 << j
		}
	}
	return value
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

func equals(theory *numTheory, x, y *integerS) {
	for j := 0; j < bitCount; j++ {
		theory.addClause(newSet([]int{x.bits[j]}), newSet([]int{y.bits[j]}))
		theory.addClause(newSet([]int{y.bits[j]}), newSet([]int{x.bits[j]}))
	}
}

func add(theory *numTheory, a, b *integerS) *integerS {
	bits := integer(theory, "add.out")
	carries := integer(theory, "add.carry")

	previous_carry := theory.var_false
	for j := 0; j < bitCount; j++ {
		current_carry := carries.bits[j]
		full_adder(theory, a.bits[j], b.bits[j], previous_carry, bits.bits[j], current_carry)
		previous_carry = current_carry
	}

	return bits
}

func and(theory *numTheory, a, b *integerS) *integerS {
	bits := integer(theory, "and.out")
	for j := 0; j < bitCount; j++ {
		and_gate(theory, a.bits[j], b.bits[j], bits.bits[j])
	}
	return bits
}

func mul(theory *numTheory, a, b *integerS) *integerS {
	intermediate := make([]*integerS, bitCount)
	repeat := func(bit int) *integerS {
		bits := []int{bit}
		for i := 1; i < bitCount; i++ {
			bits = append(bits, bit)
		}
		return &integerS{bits}
	}

	constant_lshift := func(v *integerS, i int) *integerS {
		bits := make([]int, 0)
		bits = append(bits, v.bits[:i]...)
		bits = append(bits, v.bits[i:]...)
		return &integerS{bits}
	}

	intermediate[0] = and(theory, repeat(a.bits[0]), b)
	for j := 1; j < bitCount; j++ {
		intermediate[j] = add(theory, intermediate[j-1], constant_lshift(and(theory, repeat(a.bits[j]), b), j))
	}

	return intermediate[bitCount-1]
}

func xor_gate(theory *numTheory, a, b, out int) {
	theory.addClause(newSet([]int{a, b}), newSet([]int{out}))
	theory.addClause(newSet([]int{a, out}), newSet([]int{b}))
	theory.addClause(newSet([]int{b, out}), newSet([]int{a}))
	theory.addClause(newSet[int](nil), newSet([]int{a, b, out}))
}

func or_gate(theory *numTheory, a, b, out int) {
	theory.addClause(newSet([]int{out}), newSet([]int{a}))
	theory.addClause(newSet([]int{out}), newSet([]int{b}))
	theory.addClause(newSet([]int{a, b}), newSet([]int{out}))
}

func and_gate(theory *numTheory, a, b, out int) {
	theory.addClause(newSet([]int{a}), newSet([]int{out}))
	theory.addClause(newSet([]int{b}), newSet([]int{out}))
	theory.addClause(newSet([]int{out}), newSet([]int{a, b}))
}

func full_adder(theory *numTheory, a, b, c, out, carry_out int) {
	fa0 := theory.addVar("fa.0")
	xor_gate(theory, a, b, fa0)
	xor_gate(theory, c, fa0, out)
	fa1 := theory.addVar("fa.1")
	fa2 := theory.addVar("fa.2")
	and_gate(theory, a, b, fa1)
	and_gate(theory, c, fa0, fa2)
	or_gate(theory, fa1, fa2, carry_out)
}
