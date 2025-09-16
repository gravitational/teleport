/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package stringset

// StringSet is string container in which every string is contained at most once i.e. a set data structure.
type StringSet map[string]struct{}

// New builds a string set with elements from a given slice.
func New(elems ...string) StringSet {
	set := NewWithCap(len(elems))
	set.Add(elems...)
	return set
}

// NewWithCap builds an empty string set with a given capacity.
func NewWithCap(cap int) StringSet {
	return make(StringSet, cap)
}

// Add inserts a string to the set.
func (set StringSet) Add(elems ...string) {
	for _, str := range elems {
		set[str] = struct{}{}
	}
}

// Del removes a string from the set.
func (set StringSet) Del(str string) {
	delete(set, str)
}

// Len returns a set size.
func (set StringSet) Len() int {
	return len(set)
}

// Contains checks if the set includes a given string.
func (set StringSet) Contains(str string) bool {
	_, ok := set[str]
	return ok
}

// ToSlice returns a slice with set contents.
func (set StringSet) ToSlice() []string {
	if n := set.Len(); n > 0 {
		result := make([]string, 0, n)
		for str := range set {
			result = append(result, str)
		}
		return result
	}
	return nil
}
