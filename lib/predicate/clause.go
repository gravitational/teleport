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

type clauseError int

const (
	clauseNoError clauseError = iota
	clauseUnsatisfiable
)

type clauseStatus int

const (
	clauseSatisfied clauseStatus = iota
	clauseUnsatisfied
	clauseUnknown
)

type clause struct {
	positive set[int]
	negative set[int]
}

func newClause(positive set[int], negative set[int]) clause {
	return clause{positive, negative}
}

func (c *clause) isUnit() bool {
	return len(c.positive)+len(c.negative) == 1
}

func (c *clause) isEmpty() bool {
	return len(c.positive)+len(c.negative) == 0
}

func (c *clause) applySubst(v int, truth bool) (clauseStatus, clauseError) {
	if truth {
		delete(c.negative, v)
		if c.isEmpty() {
			return clauseUnknown, clauseUnsatisfiable
		}

		if _, ok := c.positive[v]; ok {
			return clauseSatisfied, clauseNoError
		}

		return clauseUnsatisfied, clauseNoError
	} else {
		delete(c.positive, v)
		if c.isEmpty() {
			return clauseUnknown, clauseUnsatisfiable
		}

		if _, ok := c.negative[v]; ok {
			return clauseSatisfied, clauseNoError
		}

		return clauseUnsatisfied, clauseNoError
	}
}

func (c *clause) copy() clause {
	return newClause(copyMap(c.positive), copyMap(c.negative))
}
