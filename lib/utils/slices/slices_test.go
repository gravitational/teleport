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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectValues(t *testing.T) {
	for _, tt := range []struct {
		name      string
		input     []string
		collector func(string) (s string, skip bool)
		expected  []string
	}{
		{
			name:  "no elements",
			input: []string{},
			collector: func(in string) (s string, skip bool) {
				return in, false
			},
			expected: []string{},
		},
		{
			name:  "multiple strings, all match",
			input: []string{"x", "y"},
			collector: func(in string) (s string, skip bool) {
				return in, false
			},
			expected: []string{"x", "y"},
		},
		{
			name:  "deduplicates items",
			input: []string{"x", "y", "z", "x"},
			collector: func(in string) (s string, skip bool) {
				return in, false
			},
			expected: []string{"x", "y", "z"},
		},
		{
			name:  "skipped values are not returned",
			input: []string{"x", "y", "z", ""},
			collector: func(in string) (s string, skip bool) {
				return in, in == ""
			},
			expected: []string{"x", "y", "z"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectValues(tt.input, tt.collector)
			require.Equal(t, tt.expected, got)
		})
	}
}
