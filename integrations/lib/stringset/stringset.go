// Copyright 2023 Gravitational, Inc
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
