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

type instance struct {
	clauses     []clause
	assignments map[int]bool
}

func newInstance(clauses []clause) *instance {
	return &instance{clauses, make(map[int]bool)}
}

func (i *instance) propagate() {
	for {
		for i.unitPropagateOnce() {
		}

		if !i.pureLiteralEliminateOnce() {
			break
		}
	}
}

func (i *instance) unitPropagateOnce() bool {
	for _, clause := range i.clauses {
		if clause.isUnit() {
			if len(clause.positive) > 0 {
				for v := range clause.positive {
					i.applySubst(v, true)
				}
			} else {
				for v := range clause.negative {
					i.applySubst(v, false)
				}
			}

			return true
		}
	}

	return false
}

func (i *instance) pureLiteralEliminateOnce() bool {
	positive := make(set[int])
	negative := make(set[int])

	for _, clause := range i.clauses {
		for v := range clause.positive {
			positive[v] = struct{}{}
		}

		for v := range clause.negative {
			negative[v] = struct{}{}
		}
	}

	onlyPositive := difference(positive, negative)
	onlyNegative := difference(negative, positive)

	for v := range onlyPositive {
		i.applySubst(v, true)
	}

	for v := range onlyNegative {
		i.applySubst(v, false)
	}

	return len(onlyPositive) > 0 || len(onlyNegative) > 0
}

func (i *instance) pickBranchAssignment() (int, bool) {
	first_clause := i.clauses[0]
	vars := union(first_clause.positive, first_clause.negative)
	for v := range vars {
		return v, false
	}

	panic("unreachable")
}

func (i *instance) applySubst(v int, truth bool) clauseError {
	i.assignments[v] = truth
	j := 0
	for j < len(i.clauses) {
		isSatisfied, err := i.clauses[j].applySubst(v, truth)
		if err != clauseNoError {
			return err
		}

		if isSatisfied != clauseUnsatisfied {
			i.clauses = append(i.clauses[:j], i.clauses[j+1:]...)
			continue
		}

		j++
	}

	return clauseNoError
}

type pick struct {
	v     int
	truth bool
}

func (i *instance) solveInner() ([]pick, clauseError) {
	i.propagate()

	if len(i.clauses) == 0 {
		picks := make([]pick, 0)
		for v, truth := range i.assignments {
			picks = append(picks, pick{v, truth})
		}
		return picks, clauseNoError
	}

	for _, c := range i.clauses {
		if c.isEmpty() {
			return nil, clauseUnsatisfiable
		}
	}

	v, truth := i.pickBranchAssignment()
	copy := i.copy()
	err := i.applySubst(v, truth)
	if err != clauseNoError {
		return nil, err
	}

	err = copy.applySubst(v, !truth)
	if err != clauseNoError {
		return nil, err
	}

	picks, err := i.solveInner()
	if err != clauseNoError {
		return nil, err
	}

	copyPicks, err := copy.solveInner()
	if err != clauseNoError {
		return nil, err
	}

	return append(picks, copyPicks...), clauseNoError
}

func (i *instance) solve() (map[int]bool, clauseError) {
	state := i.copy()
	state.clauses = nil

	// eliminate trivial input clauses
	for _, clause := range i.clauses {
		if len(intersect(clause.positive, clause.negative)) > 0 {
			continue
		}

		state.clauses = append(state.clauses, clause)
	}

	assignments, err := state.solveInner()
	if err != clauseNoError {
		return nil, err
	}

	filtered := make(map[int]bool)
	for _, assignment := range assignments {
		filtered[assignment.v] = assignment.truth
	}

	return filtered, clauseNoError
}

func (i *instance) copy() *instance {
	clauses := make([]clause, len(i.clauses))
	for j, clause := range i.clauses {
		clauses[j] = clause.copy()
	}

	return &instance{
		clauses:     clauses,
		assignments: copyMap(i.assignments),
	}
}
