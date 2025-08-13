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

package set

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
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
			s := New(test.set...)
			require.NotNil(t, s)
			require.Len(t, s, len(test.expected))
			require.ElementsMatch(t, s.Elements(), test.expected)
		})
	}
}

func TestAdd(t *testing.T) {
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
			a := New(test.augend...)
			a.Add(test.addends...)

			require.ElementsMatch(t, a.Elements(), test.expected)
		})
	}
}

func TestRemove(t *testing.T) {
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
			s := New(test.set...)
			s.Remove(test.remove...)
			require.ElementsMatch(t, test.expected, s.Elements())
		})
	}
}

func TestContains(t *testing.T) {
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
			s := New(test.set...)
			test.expectation(t, s.Contains(test.element))
		})
	}
}

func TestUnion(t *testing.T) {
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
			a := New(test.a...)
			b := New(test.b...)
			bItems := b.Elements()

			a.Union(b)
			require.ElementsMatch(t, a.Elements(), test.expected)

			require.ElementsMatch(t, b.Elements(), bItems)
		})
	}
}

func TestSubtract(t *testing.T) {
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
			a := New(test.minuend...)
			b := New(test.subtrahend...)
			bItems := b.Elements()

			a.Subtract(b)
			require.ElementsMatch(t, a.Elements(), test.expected)

			require.ElementsMatch(t, b.Elements(), bItems)
		})
	}
}

func TestIterate(t *testing.T) {
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
			s := New(test.elements...)

			iteratedElements := make([]string, 0, len(s))
			for element := range s {
				iteratedElements = append(iteratedElements, element)
			}
			require.ElementsMatch(t, iteratedElements, test.elements)
		})
	}
}

func TestIntersection(t *testing.T) {
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
			a := New(test.a...)
			b := New(test.b...)
			bItems := b.Elements()

			a.Intersection(b)
			require.ElementsMatch(t, a.Elements(), test.expected)

			require.ElementsMatch(t, b.Elements(), bItems)
		})
	}
}

func TestTransform(t *testing.T) {
	src := New(1, 2, 3, 4, 5, 6, 7, 8, 9, 0)
	dst := Transform(src, strconv.Itoa)
	require.ElementsMatch(t,
		[]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		dst.Elements(),
	)
}
