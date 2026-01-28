// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"cmp"
	"iter"
	"slices"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

// Preconditions is a set of decisionpb.Precondition. Enforcement of preconditions should be done in the order returned
// by the All method for consistency. If multiple preconditions are not satisfied, a combined error that lists all
// unsatisfied preconditions should be returned along with any relevant details.
type Preconditions struct {
	preconditions []*decisionpb.Precondition
}

// NewPreconditions creates a new Preconditions set.
func NewPreconditions() *Preconditions {
	return &Preconditions{
		preconditions: make([]*decisionpb.Precondition, 0),
	}
}

// Add adds a precondition to the set if the kind is not already present.
func (p *Preconditions) Add(precondition *decisionpb.Precondition) {
	// Find insertion point using binary search.
	index, found := slices.BinarySearchFunc(p.preconditions, precondition, comparePreconditions)
	if !found {
		// Insert at the correct position to maintain sorted order.
		p.preconditions = slices.Insert(p.preconditions, index, precondition)
	}
}

// All returns an iterator over the preconditions in sorted kind order.
func (p *Preconditions) All() iter.Seq[*decisionpb.Precondition] {
	return func(yield func(*decisionpb.Precondition) bool) {
		for _, precondition := range p.preconditions {
			if !yield(precondition) {
				return
			}
		}
	}
}

// Contains returns true if a precondition with the given kind is in the set.
func (p *Preconditions) Contains(kind decisionpb.PreconditionKind) bool {
	_, found := slices.BinarySearchFunc(p.preconditions, &decisionpb.Precondition{Kind: kind}, comparePreconditions)
	return found
}

// Len returns the number of preconditions.
func (p *Preconditions) Len() int {
	return len(p.preconditions)
}

func comparePreconditions(a, b *decisionpb.Precondition) int {
	return cmp.Compare(a.Kind, b.Kind)
}
