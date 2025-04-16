// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package utils

import "maps"

// Set models a collection of unique elements. Its implemented as the classic
// Go `map[T]struct{}` pattern and has the same underlying representation. This
// has the nice property of preserving the map-like reference semantics. You can
// even iterate over the set with `range`.
type Set[T comparable] map[T]struct{}

// NewSet constructs a set from an arbitrary collection of elements
func NewSet[T comparable](elements ...T) Set[T] {
	s := NewSetWithCapacity[T](len(elements))
	s.Add(elements...)
	return s
}

// NewSetWithCapacity constructs a new, empty set with an given capacity hint.
func NewSetWithCapacity[T comparable](n int) Set[T] {
	return make(map[T]struct{}, n)
}

// Add expands the set to include the supplied elements (if not already included),
// returning a reference to the mutated set.
func (s Set[T]) Add(elements ...T) Set[T] {
	for _, element := range elements {
		s[element] = struct{}{}
	}
	return s
}

// Union updates the set to be the union of the original set and `other`
func (s Set[T]) Union(others ...Set[T]) {
	for _, other := range others {
		for element := range other {
			s[element] = struct{}{}
		}
	}
}

// Remove deletes elements from the set, returning a reference to the mutated
// set. Attempting to remove a non-existent element from the set is not
// considered an error.
func (s Set[T]) Remove(elements ...T) Set[T] {
	for _, element := range elements {
		delete(s, element)
	}
	return s
}

// Contains implements a membership test for the Set.
func (s Set[T]) Contains(element T) bool {
	_, present := s[element]
	return present
}

// Clone creates a new, independent copy of the Set.
func (s Set[T]) Clone() Set[T] {
	return maps.Clone(s)
}

// Subtract removes all elements in `other` from the set (i.e `s` becomes the
// Set Difference of `s` and `other`), returning a reference to the mutated set.
func (s Set[T]) Subtract(other Set[T]) Set[T] {
	for k := range other {
		delete(s, k)
	}
	return s
}

// Intersection updates the set to contain the similarity between the set and `other`
func (s Set[T]) Intersection(other Set[T]) {
	for b := range s {
		if !other.Contains(b) {
			s.Remove(b)
		}
	}
}

// Elements returns the elements in the set. Order of the elements is undefined.
//
// NOTE: Due to the underlying map type, a set can be naturally ranged over like
// a map, for example:
//
//	alphabet := NewSet("alpha", "beta", "gamma")
//	for l := range alphabet {
//		fmt.Printf("%s is a letter\n", l)
//	}
//
// Prefer using the natural range iteration where possible
func (s Set[T]) Elements() []T {
	elements := make([]T, 0, len(s))
	for e := range s {
		elements = append(elements, e)
	}
	return elements
}

// SetTransform maps a set into another set, using the supplied `transform`
// mapping function to convert the set elements.
func SetTransform[SrcT, DstT comparable](src Set[SrcT], transform func(SrcT) DstT) Set[DstT] {
	dst := NewSetWithCapacity[DstT](len(src))
	for element := range src {
		dst.Add(transform(element))
	}
	return dst
}
