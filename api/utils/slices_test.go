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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(t, ContainSameUniqueElements(test.s1, test.s2))
		})
	}
}
