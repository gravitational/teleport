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

const (
	dpllSatisfied = iota
	dpllUnsatisfied
	dpllUnknown
)

type node any

type nodeLiteral struct {
	value bool
}

type nodeIdentifier struct {
	key string
}

type nodeNot struct {
	left node
}

type nodeOr struct {
	left  node
	right node
}

type nodeAnd struct {
	left  node
	right node
}

type assignment struct {
	key   string
	value bool
}

type state struct {
	clauses     []node
	assignments []assignment
	enforce     []node
}

func newState(clause node) *state {
	clauses := []node{clause}

	return &state{
		clauses: clauses,
	}
}

func pickLiteral(node node, isExcluded func(string) bool) *string {
	or := func(left, right *string) *string {
		if left != nil {
			return left
		}

		return right
	}

	switch node := node.(type) {
	case *nodeLiteral:
		return nil
	case *nodeIdentifier:
		if isExcluded(node.key) {
			return nil
		}

		return &node.key
	case *nodeNot:
		return pickLiteral(node.left, isExcluded)
	case *nodeOr:
		return or(pickLiteral(node.left, isExcluded), pickLiteral(node.right, isExcluded))
	case *nodeAnd:
		return or(pickLiteral(node.left, isExcluded), pickLiteral(node.right, isExcluded))
	default:
		panic("unreachable")
	}
}

func evalNode(state *state, node node) int {
	switch node := node.(type) {
	case *nodeLiteral:
		switch node.value {
		case true:
			return dpllSatisfied
		case false:
			return dpllUnsatisfied
		}
	case *nodeIdentifier:
		assigned, value := isAssigned(state, node.key)
		if assigned {
			switch value {
			case true:
				return dpllSatisfied
			case false:
				return dpllUnsatisfied
			}
		}

		return dpllUnknown
	case *nodeNot:
		switch evalNode(state, node.left) {
		case dpllSatisfied:
			return dpllUnsatisfied
		case dpllUnsatisfied:
			return dpllSatisfied
		case dpllUnknown:
			return dpllUnknown
		}
	case *nodeOr:
		switch evalNode(state, node.left) {
		case dpllSatisfied:
			return dpllSatisfied
		case dpllUnsatisfied:
			return evalNode(state, node.right)
		case dpllUnknown:
			switch evalNode(state, node.right) {
			case dpllSatisfied:
				return dpllSatisfied
			case dpllUnsatisfied:
				return dpllUnknown
			case dpllUnknown:
				return dpllUnknown
			}
		}
	case *nodeAnd:
		switch evalNode(state, node.left) {
		case dpllSatisfied:
			return evalNode(state, node.right)
		case dpllUnsatisfied:
			return dpllUnsatisfied
		case dpllUnknown:
			switch evalNode(state, node.right) {
			case dpllSatisfied:
				return dpllUnknown
			case dpllUnsatisfied:
				return dpllUnsatisfied
			case dpllUnknown:
				return dpllUnknown
			}
		}
	}

	panic("unreachable")
}

func isAssigned(state *state, key string) (bool, bool) {
	for _, assignment := range state.assignments {
		if assignment.key == key {
			return true, assignment.value
		}
	}

	return false, false
}

func pickUnassigned(state *state) *string {
	for _, clause := range state.clauses {
		a := pickLiteral(clause, func(x string) bool { assigned, _ := isAssigned(state, x); return assigned })
		if a != nil {
			return a
		}
	}

	return nil
}

func backtrackAdjust(state *state, rem []assignment) bool {
	satisfied := func() bool {
		for _, clause := range state.clauses {
			switch evalNode(state, clause) {
			case dpllSatisfied, dpllUnknown:
				continue
			case dpllUnsatisfied:
				return false
			}
		}

		return true
	}

	recurse := func() bool { return satisfied() || (len(rem) != 0 && backtrackAdjust(state, rem[:len(rem)-1])) }

	if recurse() {
		return true
	}

	if len(rem) > 0 {
		ass := &rem[len(rem)-1]
		fmt.Printf("repick %v = %v\n", ass.key, !ass.value)
		ass.value = !ass.value

		if recurse() {
			return true
		}
	}

	return false
}

// opt: watch-literal based unit propagation (remember cnf conversion)
func dpll(state *state) bool {
	// check that nonvariable clauses are satisfied
	for _, clause := range state.enforce {
		switch evalNode(state, clause) {
		case dpllSatisfied:
			continue
		case dpllUnsatisfied:
			// pure clause cannot be satisfied, formula is unsat
			return false
		case dpllUnknown:
			panic("unreachable")
		}
	}

	for {
		literal := pickUnassigned(state)
		if literal == nil {
			// all variables are assigned, formula is sat
			return true
		}

		state.assignments = append(state.assignments, assignment{key: *literal, value: true})
		fmt.Printf("pick %v = true\n", *literal)
		if !backtrackAdjust(state, state.assignments) {
			// backtrack failed, formula is unsat
			return false
		}
	}
}
