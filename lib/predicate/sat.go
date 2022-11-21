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
}

func dpll_is_assigned(state *state, key string) (bool, bool) {
	for _, assignment := range state.assignments {
		if assignment.key == key {
			return true, assignment.value
		}
	}

	return false, false
}

func dpll_select_literal_clause(state *state, node node) *string {
	one_of_two := func(left, right *string) *string {
		if left != nil {
			return left
		}

		return right
	}

	switch node := node.(type) {
	case *nodeLiteral:
		return nil
	case *nodeIdentifier:
		assigned, _ := dpll_is_assigned(state, node.key)
		if assigned {
			return nil
		}

		return &node.key
	case *nodeNot:
		return dpll_select_literal_clause(state, node.left)
	case *nodeOr:
		return one_of_two(dpll_select_literal_clause(state, node.left), dpll_select_literal_clause(state, node.right))
	case *nodeAnd:
		return one_of_two(dpll_select_literal_clause(state, node.left), dpll_select_literal_clause(state, node.right))
	default:
		panic("unreachable")
	}
}

func dpll_select_literal(state *state) *string {
	for _, clause := range state.clauses {
		if literal := dpll_select_literal_clause(state, clause); literal != nil {
			return literal
		}
	}

	return nil
}

func dpll_is_clause_statisfied(state *state, node node) int {
	switch node := node.(type) {
	case *nodeLiteral:
		switch node.value {
		case true:
			return dpllSatisfied
		case false:
			return dpllUnsatisfied
		}
	case *nodeIdentifier:
		assigned, value := dpll_is_assigned(state, node.key)
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
		switch dpll_is_clause_statisfied(state, node.left) {
		case dpllSatisfied:
			return dpllUnsatisfied
		case dpllUnsatisfied:
			return dpllSatisfied
		case dpllUnknown:
			return dpllUnknown
		}
	case *nodeOr:
		left := dpll_is_clause_statisfied(state, node.left)
		right := dpll_is_clause_statisfied(state, node.right)
		if left == dpllUnknown || right == dpllUnknown {
			return dpllUnknown
		}

		if left == dpllSatisfied || right == dpllSatisfied {
			return dpllSatisfied
		}

		return dpllUnsatisfied
	case *nodeAnd:
		left := dpll_is_clause_statisfied(state, node.left)
		right := dpll_is_clause_statisfied(state, node.right)
		if left == dpllUnknown || right == dpllUnknown {
			return dpllUnknown
		}

		if left == dpllSatisfied && right == dpllSatisfied {
			return dpllSatisfied
		}

		return dpllUnsatisfied
	}

	panic(fmt.Sprintf("unknown node type: %T", node))
}

func dpll_is_satisfied(state *state) bool {
	for _, clause := range state.clauses {
		if dpll_is_clause_statisfied(state, clause) == dpllUnsatisfied {
			return false
		}
	}

	return true
}

// convert to cnf
// backtracking fix
func dpll(state *state) bool {
	for {
		i := len(state.assignments) - 1
		for !dpll_is_satisfied(state) {
			state.assignments[i].value = !state.assignments[i].value

			// formula is unsatisfiable
			if i == 0 {
				return false
			}
			i--
		}

		v := dpll_select_literal(state)
		if v == nil {
			break
		}

		// try assigning v to true
		state.assignments = append(state.assignments, assignment{key: *v, value: true})
	}

	return true
}
