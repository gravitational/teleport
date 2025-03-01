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

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {

	t.Run("create", func(t *testing.T) {
		testCases := []struct {
			name     string
			set      []string
			expected []string
		}{
			{
				name: "empty",
			},
			{
				name:     "populated",
				set:      []string{"alpha", "beta", "gamma", "delta"},
				expected: []string{"alpha", "beta", "gamma", "delta"},
			},
			{
				name:     "populated with duplicates",
				set:      []string{"alpha", "beta", "gamma", "alpha", "delta", "beta"},
				expected: []string{"alpha", "beta", "gamma", "delta"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				s := NewSet(test.set...)
				require.NotNil(t, s)
				require.Len(t, s, len(test.expected))
				require.ElementsMatch(t, s.Elements(), test.expected)
			})
		}
	})

	t.Run("add", func(t *testing.T) {
		testCases := []struct {
			name     string
			augend   []string
			addends  []string
			expected []string
		}{
			{
				name:     "to empty set",
				addends:  []string{"alpha", "omega"},
				expected: []string{"alpha", "omega"},
			},
			{
				name:     "to populated set",
				augend:   []string{"alpha", "omega"},
				addends:  []string{"alpha", "beta", "gamma"},
				expected: []string{"alpha", "beta", "gamma", "omega"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				a := NewSet(test.augend...)
				a.Add(test.addends...)

				// EXPECT that the elements have been added to the target set
				require.ElementsMatch(t, a.Elements(), test.expected)
			})
		}
	})

	t.Run("remove", func(t *testing.T) {
		testCases := []struct {
			name     string
			set      []string
			remove   []string
			expected []string
		}{
			{
				name:   "from empty set",
				remove: []string{"banana", "potato"},
			},
			{
				name:     "from populated set (present)",
				set:      []string{"alpha", "beta", "gamma", "omega"},
				remove:   []string{"beta", "omega"},
				expected: []string{"alpha", "gamma"},
			},
			{
				name:     "from populated set (not present)",
				set:      []string{"alpha", "beta", "gamma", "omega"},
				remove:   []string{"banana", "potato"},
				expected: []string{"alpha", "beta", "gamma", "omega"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				s := NewSet(test.set...)
				s.Remove(test.remove...)
				require.ElementsMatch(t, test.expected, s.Elements())
			})
		}
	})

	t.Run("contains", func(t *testing.T) {
		testCases := []struct {
			name        string
			set         []string
			element     string
			expectation require.BoolAssertionFunc
		}{
			{
				name:        "on empty set",
				element:     "potato",
				expectation: require.False,
			},
			{
				name:        "populated set includes element",
				set:         []string{"alpha", "beta", "gamma", "omega"},
				element:     "gamma",
				expectation: require.True,
			},
			{
				name:        "populated set excludes element",
				set:         []string{"alpha", "beta", "gamma", "omega"},
				element:     "lambda",
				expectation: require.False,
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				s := NewSet(test.set...)
				test.expectation(t, s.Contains(test.element))
			})
		}
	})

	t.Run("union", func(t *testing.T) {
		testCases := []struct {
			name     string
			a        []string
			b        []string
			expected []string
		}{
			{
				name: "empty union empty",
			},
			{
				name:     "empty union populated",
				b:        []string{"alpha", "beta"},
				expected: []string{"alpha", "beta"},
			},
			{
				name:     "populated union empty",
				a:        []string{"alpha", "beta"},
				expected: []string{"alpha", "beta"},
			},
			{
				name:     "populated union populated",
				a:        []string{"alpha", "beta", "gamma", "delta", "epsilon"},
				b:        []string{"beta", "eta", "zeta", "epsilon"},
				expected: []string{"alpha", "beta", "gamma", "delta", "epsilon", "eta", "zeta"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				// GIVEN a pair of sets
				a := NewSet(test.a...)
				b := NewSet(test.b...)
				bItems := b.Elements()

				// WHEN I take the union of both sets
				a.Union(b)

				// EXPECT that the target set is updated with the union of both sets.
				require.ElementsMatch(t, a.Elements(), test.expected)

				// EXPECT also that the second set is unchanged
				require.ElementsMatch(t, b.Elements(), bItems)
			})
		}
	})

	t.Run("subtract", func(t *testing.T) {
		testCases := []struct {
			name       string
			minuend    []string
			subtrahend []string
			expected   []string
		}{
			{
				name: "empty minus empty",
			},
			{
				name:       "empty minus populated",
				subtrahend: []string{"alpha", "beta"},
			},
			{
				name:     "populated minus empty",
				minuend:  []string{"alpha", "beta"},
				expected: []string{"alpha", "beta"},
			},
			{
				name:       "populated minus populated",
				minuend:    []string{"alpha", "beta", "gamma", "delta", "epsilon"},
				subtrahend: []string{"beta", "eta", "zeta", "epsilon"},
				expected:   []string{"alpha", "gamma", "delta"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				// GIVEN a pair of sets
				a := NewSet(test.minuend...)
				b := NewSet(test.subtrahend...)
				bItems := b.Elements()

				// WHEN I compute the set difference on the two sets
				a.Subtract(b)

				// EXPECT that the target set has any common elements removed.
				require.ElementsMatch(t, a.Elements(), test.expected)

				// EXPECT that to subtracted set is unchanged
				require.ElementsMatch(t, b.Elements(), bItems)
			})
		}
	})

	t.Run("iteration", func(t *testing.T) {
		testCases := []struct {
			name     string
			elements []string
		}{
			{
				name:     "with empty set",
				elements: nil,
			},
			{
				name:     "with populated set",
				elements: []string{"alpha", "beta", "gamma", "delta", "omega"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				// GIVEN a set with arbitrary elements
				s := NewSet(test.elements...)

				// WHEN I iterate over the set using the Go builtin `range` loop
				var iteratedElements []string
				for element := range s {
					iteratedElements = append(iteratedElements, element)
				}

				// Expect that all elements in the set were hit
				require.ElementsMatch(t, iteratedElements, test.elements)
			})
		}
	})

	t.Run("intersection", func(t *testing.T) {
		testCases := []struct {
			name     string
			a        []string
			b        []string
			expected []string
		}{
			{
				name:     "empty intersection empty",
				expected: []string{},
			},
			{
				name:     "empty intersection populated",
				b:        []string{"alpha", "beta"},
				expected: []string{},
			},
			{
				name:     "populated intersection empty",
				a:        []string{"alpha", "beta"},
				expected: []string{},
			},
			{
				name:     "populated intersection populated",
				a:        []string{"alpha", "beta", "gamma", "delta", "epsilon"},
				b:        []string{"beta", "eta", "zeta", "epsilon"},
				expected: []string{"beta", "epsilon"},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				// GIVEN a pair of sets
				a := NewSet(test.a...)
				b := NewSet(test.b...)
				bItems := b.Elements()

				// WHEN I take the intersection of both sets
				a.Intersection(b)

				// EXPECT that the target set is updated with the intersection of both sets.
				require.ElementsMatch(t, a.Elements(), test.expected)

				// EXPECT also that the second set is unchanged
				require.ElementsMatch(t, b.Elements(), bItems)
			})
		}
	})
}

func TestSetTransform(t *testing.T) {
	src := NewSet(1, 2, 3, 4, 5, 6, 7, 8, 9, 0)
	dst := SetTransform(src, strconv.Itoa)
	require.ElementsMatch(t,
		[]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		dst.Elements(),
	)
}
