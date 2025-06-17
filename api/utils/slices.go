/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"slices"
	"strings"
)

// JoinStrings returns a string that is all the elements in the slice `T[]` joined by `sep`
// This being generic allows for the usage of custom string times, without having to convert
// the elements to a string to be passed into `strings.Join`.
func JoinStrings[T ~string](elems []T, sep string) T {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	n := len(sep) * (len(elems) - 1)
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(string(elems[0]))
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(string(s))
	}
	return T(b.String())
}

// Deduplicate deduplicates list of comparable values.
func Deduplicate[T comparable](in []T) []T {
	if len(in) <= 1 {
		return in
	}
	out := make([]T, 0, len(in))
	seen := make(map[T]struct{}, len(in))
	for _, val := range in {
		if _, ok := seen[val]; !ok {
			out = append(out, val)
			seen[val] = struct{}{}
		}
	}
	return out
}

// DeduplicateAny deduplicates list of any values with compare function.
func DeduplicateAny[T any](in []T, compare func(T, T) bool) []T {
	if len(in) <= 1 {
		return in
	}
	out := make([]T, 0, len(in))
	for _, val := range in {
		var seen bool
		for _, outVal := range out {
			if compare(val, outVal) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, val)
		}
	}
	return out
}

// ContainSameUniqueElements returns true if the input slices contain the same
// unique elements. Ordering and duplicates are ignored.
func ContainSameUniqueElements[S ~[]E, E comparable](s1, s2 S) bool {
	s1Dedup := Deduplicate(s1)
	s2Dedup := Deduplicate(s2)

	if len(s1Dedup) != len(s2Dedup) {
		return false
	}
	for i := range s1Dedup {
		if !slices.Contains(s2Dedup, s1Dedup[i]) {
			return false
		}
	}
	return true
}

// All checks if all elements of slice satisfy given predicate. If the slice is empty, it returns true.
func All[S ~[]E, E any](s S, predicate func(E) bool) bool {
	for _, e := range s {
		if !predicate(e) {
			return false
		}
	}
	return true
}

// CountBy counts the occurrences of each element in a slice based on a given mapper function.
func CountBy[S ~[]E, E any](elements S, mapper func(E) string) map[string]int {
	out := make(map[string]int)
	for _, elem := range elements {
		key := mapper(elem)
		out[key] += 1
	}
	return out
}
