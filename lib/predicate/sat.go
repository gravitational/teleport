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
	watched     map[string][]node
	enforce     []node
	uprop       []node
}

func newState(clause node) *state {
	// todo: cnf conversion
	clauses := []node{clause}
	watched := make(map[string][]node)
	enforce := make([]node, 0)
	uprop := make([]node, 0)

	for _, clause := range clauses {
		a := pickLiteral(clause, func(x string) bool { return false })
		if a != nil {
			enforce = append(enforce, clause)
			continue
		}

		b := pickLiteral(clause, func(x string) bool { return x != *a })
		if b != nil {
			uprop = append(uprop, clause)
			continue
		}

		for _, lit := range []string{*a, *b} {
			watched[lit] = append(watched[lit], clause)
		}
	}

	return &state{
		clauses: clauses,
		watched: watched,
		enforce: enforce,
		uprop:   uprop,
	}
}

func pickLiteral(node node, isExcluded func(string) bool) *string {
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
		if isExcluded(node.key) {
			return nil
		}

		return &node.key
	case *nodeNot:
		return pickLiteral(node.left, isExcluded)
	case *nodeOr:
		return one_of_two(pickLiteral(node.left, isExcluded), pickLiteral(node.right, isExcluded))
	case *nodeAnd:
		return one_of_two(pickLiteral(node.left, isExcluded), pickLiteral(node.right, isExcluded))
	default:
		panic("unreachable")
	}
}

func dpll(state *state) bool {
	return true
}
