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

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBranches(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
		checkErr require.ErrorAssertionFunc
		desc     string
	}{
		{
			input:    "branch/v7",
			expected: []string{"branch/v7"},
			checkErr: require.NoError,
			desc:     "valid-branches-input-one-branch",
		},
		{
			input:    "branch/v6,branch/v7,branch/v8",
			expected: []string{"branch/v6", "branch/v7", "branch/v8"},
			checkErr: require.NoError,
			desc:     "valid-branches-input-multiple-branches",
		},
		{
			input:    "",
			expected: nil,
			checkErr: require.Error,
			desc:     "invalid-branches-input-empty-branch",
		},

		{
			input:    ",,,branch/v7",
			expected: nil,
			checkErr: require.Error,
			desc:     "invalid-branches-input-multiple-empty-branches",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			expect, err := parseBranches(test.input)
			if test.expected != nil {
				require.ElementsMatch(t, expect, test.expected)
			}
			test.checkErr(t, err)
		})
	}
}
