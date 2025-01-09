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

package slices

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type aType struct {
	fieldA string
}

func TestFilterMapUnique(t *testing.T) {
	for _, tt := range []struct {
		name      string
		input     []string
		collector func(string) (s string, include bool)
		expected  []string
	}{
		{
			name:  "no elements",
			input: []string{},
			collector: func(in string) (s string, include bool) {
				return in, true
			},
			expected: []string{},
		},
		{
			name:  "multiple strings, all match",
			input: []string{"x", "y"},
			collector: func(in string) (s string, include bool) {
				return in, true
			},
			expected: []string{"x", "y"},
		},
		{
			name:  "deduplicates items",
			input: []string{"x", "y", "z", "x"},
			collector: func(in string) (s string, include bool) {
				return in, true
			},
			expected: []string{"x", "y", "z"},
		},
		{
			name:  "not included values are not returned",
			input: []string{"x", "y", "z", ""},
			collector: func(in string) (s string, include bool) {
				return in, in != ""
			},
			expected: []string{"x", "y", "z"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterMapUnique(tt.input, tt.collector)
			require.Equal(t, tt.expected, got)
		})
	}

	t.Run("structs", func(t *testing.T) {
		input := []aType{
			{"+a"},
			{"+b"},
			{"+b"},
			{"b"},
			{"z"},
		}
		withPlusPrefix := func(a aType) (string, bool) {
			return a.fieldA, strings.HasPrefix(a.fieldA, "+")
		}
		got := FilterMapUnique(input, withPlusPrefix)

		expected := []string{"+a", "+b"}
		require.Equal(t, expected, got)
	})
}
