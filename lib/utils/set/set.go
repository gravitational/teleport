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

package set

import "golang.org/x/exp/maps"

// Set represents a set data structure, using the map type with an empty value.
type Set[T comparable] map[T]struct{}

// Empty returns a new, empty set.
func Empty[T comparable]() Set[T] {
	return make(map[T]struct{})
}

// Union returns a new Set that is the union of one or more other sets. A single
// argument effectively clones the set.
func Union[T comparable](sets ...Set[T]) Set[T] {
	ret := make(map[T]struct{})
	for _, set := range sets {
		for key := range set {
			ret[key] = struct{}{}
		}
	}

	return ret
}

// New returns a new Set from the given array of entries. Duplicate entries will
// be removed.
func New[T comparable](entries ...T) Set[T] {
	ret := make(map[T]struct{})

	for _, entry := range entries {
		ret[entry] = struct{}{}
	}

	return ret
}

// Union returns a new Set that is the union of this set and one or more other
// sets.
func (s Set[T]) Union(sets ...Set[T]) Set[T] {
	args := append([]Set[T]{s}, sets...)
	return Union(args...)
}

// Clone returns a clone of this set.
func (s Set[T]) Clone() Set[T] {
	return Union(s)
}

// ToArray converts a set to an array. Note that the output order is undefined.
func (s Set[T]) ToArray() []T {
	return maps.Keys(s)
}

// Equal determines if this set contains the same keys as another set.
func (s Set[T]) Equals(other Set[T]) bool {
	return maps.Equal(s, other)
}
