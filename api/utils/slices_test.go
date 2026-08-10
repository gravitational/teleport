/*
Copyright 2022 Gravitational, Inc.

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
	"bytes"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeduplicate(t *testing.T) {
	tests := []struct {
		name         string
		in, expected []string
	}{
		{name: "empty slice", in: []string{}, expected: []string{}},
		{name: "slice with unique elements", in: []string{"a", "b"}, expected: []string{"a", "b"}},
		{name: "slice with duplicate elements", in: []string{"a", "b", "b", "a", "c"}, expected: []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, Deduplicate(tc.in))
		})
	}
}

func TestDeduplicateAny(t *testing.T) {
	tests := []struct {
		name         string
		in, expected [][]byte
	}{
		{name: "empty slice", in: [][]byte{}, expected: [][]byte{}},
		{name: "slice with unique elements", in: [][]byte{{0}, {1}}, expected: [][]byte{{0}, {1}}},
		{name: "slice with duplicate elements", in: [][]byte{{0}, {1}, {1}, {0}, {2}}, expected: [][]byte{{0}, {1}, {2}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, DeduplicateAny(tc.in, bytes.Equal))
		})
	}
}

func TestContainSameUniqueElements(t *testing.T) {
	tests := []struct {
		name  string
		s1    []string
		s2    []string
		check require.BoolAssertionFunc
	}{
		{
			name:  "empty",
			s1:    nil,
			s2:    []string{},
			check: require.True,
		},
		{
			name:  "same",
			s1:    []string{"a", "b", "c"},
			s2:    []string{"a", "b", "c"},
			check: require.True,
		},
		{
			name:  "same with different order",
			s1:    []string{"b", "c", "a"},
			s2:    []string{"a", "b", "c"},
			check: require.True,
		},
		{
			name:  "same with duplicates",
			s1:    []string{"a", "a", "b", "c"},
			s2:    []string{"c", "c", "a", "b", "c", "c"},
			check: require.True,
		},
		{
			name:  "different",
			s1:    []string{"a", "b"},
			s2:    []string{"a", "b", "c"},
			check: require.False,
		},
		{
			name:  "different (same length)",
			s1:    []string{"d", "a", "b"},
			s2:    []string{"a", "b", "c"},
			check: require.False,
		},
		{
			name:  "different (s2 subset of s1)",
			s1:    []string{"a", "b", "c"},
			s2:    []string{"b", "a", "a"},
			check: require.False,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(t, ContainSameUniqueElements(test.s1, test.s2))
		})
	}
}

// TestContainSameUniqueElements_SameUniqueSet builds two slices over the same
// unique element set (every element at least once, plus random duplicates,
// shuffled) and asserts the result is true in both orders.
func TestContainSameUniqueElements_SameUniqueSet(t *testing.T) {
	for range 1000 {
		uniq := randDistinct()
		s1 := randMultisetOver(uniq)
		s2 := randMultisetOver(uniq)

		require.True(t, ContainSameUniqueElements(s1, s2), "s1=%v s2=%v", s1, s2)
		require.True(t, ContainSameUniqueElements(s2, s1), "s1=%v s2=%v", s1, s2)
	}
}

// TestContainSameUniqueElements_MissingElement removes one element from one
// side, so the unique sets differ by construction, and asserts the result is
// false.
func TestContainSameUniqueElements_MissingElement(t *testing.T) {
	for range 1000 {
		uniq := randDistinct()
		removed := rand.IntN(len(uniq))
		smaller := slices.Delete(slices.Clone(uniq), removed, removed+1)

		s1 := randMultisetOver(uniq)
		s2 := randMultisetOver(smaller)

		require.False(t, ContainSameUniqueElements(s1, s2), "s1=%v s2=%v", s1, s2)
		require.False(t, ContainSameUniqueElements(s2, s1), "s1=%v s2=%v", s1, s2)
	}
}

// randDistinct returns 1-8 distinct ints.
func randDistinct() []int {
	return rand.Perm(32)[:1+rand.IntN(8)]
}

// randMultisetOver returns a slice whose set of unique elements is exactly
// uniq: one copy of each element, plus random extra copies, shuffled.
func randMultisetOver(uniq []int) []int {
	out := slices.Clone(uniq)
	if len(uniq) > 0 {
		for range rand.IntN(6) {
			out = append(out, uniq[rand.IntN(len(uniq))])
		}
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func TestAll(t *testing.T) {
	tests := []struct {
		name       string
		inputSlice []int
		predicate  func(e int) bool
		expected   bool
	}{
		{
			name:       "empty slice",
			inputSlice: []int{},
			predicate:  func(e int) bool { return e > 0 },
			expected:   true,
		},
		{
			name:       "non-empty slice with all matching elements",
			inputSlice: []int{1, 2, 3},
			predicate:  func(e int) bool { return e > 0 },
			expected:   true,
		},
		{
			name:       "non-empty slice with at least one non-matching element",
			inputSlice: []int{1, 2, -3},
			predicate:  func(e int) bool { return e > 0 },
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, All(tt.inputSlice, tt.predicate))
		})
	}
}

func TestCountBy(t *testing.T) {
	type testCase struct {
		name     string
		elements []int
		mapper   func(int) string
		want     map[string]int
	}
	tests := []testCase{
		{
			name:     "empty slice",
			elements: nil,
			mapper:   nil,
			want:     make(map[string]int),
		},
		{
			name:     "identity",
			elements: []int{1, 2, 3, 4},
			mapper:   strconv.Itoa,
			want: map[string]int{
				"1": 1,
				"2": 1,
				"3": 1,
				"4": 1,
			},
		},
		{
			name:     "even-odd",
			elements: []int{1, 2, 3, 4},
			mapper: func(i int) string {
				if i%2 == 0 {
					return "even"
				}
				return "odd"
			},
			want: map[string]int{
				"odd":  2,
				"even": 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountBy(tt.elements, tt.mapper)
			require.Equal(t, tt.want, got)
		})
	}
}
