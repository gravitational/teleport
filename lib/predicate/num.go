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
	counter     int
	abs_counter int
	clauses     []clause
	vars        set[int]
	var_names   map[int]string
	var_true    int
	var_false   int
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

func (t *numTheory) addClause(positive set[int], negative set[int]) {
	t.clauses = append(t.clauses, newClause(positive, negative))
}

func (t *numTheory) addAbstractVar() int {
	v := t.abs_counter
	t.abs_counter++
	return v
}

func (t *numTheory) finish() []clause {
	return t.clauses
}

type integer struct {
	bits []int
}

func newInteger(theory *numTheory) *integer {
	id := theory.addAbstractVar()
	bits := make([]int, 0)
	for i := 0; i < bitCount; i++ {
		bits = append(bits, theory.addVar(fmt.Sprintf("%v_%v", id, i)))
	}

	return &integer{bits}
}

func (i *integer) constantContraint(theory *numTheory, value int) {
	for j := 0; j < bitCount; j++ {
		bit := (value >> j) & 1
		if bit == 1 {
			theory.addClause(newSet([]int{i.bits[j]}), newSet[int](nil))
		} else {
			theory.addClause(newSet[int](nil), newSet([]int{i.bits[j]}))
		}
	}
}
