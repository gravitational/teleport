/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package expression

import (
	"github.com/gravitational/teleport/lib/utils"
)

// Set is a map of type string key and struct values. Set is a thin wrapper over
// the utils.Set[T] generic set type, allowing the Set to implement the
// interface(s) required for use with the expression package.
// The default value is an empty set and all methods are safe to call (even if
// the underlying map is nil).
type Set struct {
	s utils.Set[string]
}

// NewSet constructs a new set from an arbitrary collection of elements
func NewSet(values ...string) Set {
	return Set{utils.NewSet(values...)}
}

// add creates a new Set containing all values in the receiver Set and adds
// [elements].
func (s Set) add(elements ...string) Set {
	if s.s == nil {
		return NewSet(elements...)
	}
	return Set{s.s.Clone().Add(elements...)}
}

// remove creates a new Set containing all values in the receiver Set, minus
// all supplied elements. Implements expression.Remover for Set.
func (s Set) remove(elements ...string) any {
	return Set{s.s.Clone().Remove(elements...)}
}

func (s Set) contains(element string) bool {
	return s.s.Contains(element)
}

func (s Set) clone() Set {
	return Set{s.s.Clone()}
}

func (s Set) items() []string {
	return s.s.Elements()
}

// union computes the union of multiple sets
func union(sets ...Set) Set {
	result := utils.NewSet[string]()
	for _, set := range sets {
		result.Union(set.s)
	}
	return Set{result}
}
