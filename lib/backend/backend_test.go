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

package backend

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestParams(t *testing.T) {
	const (
		expectedPath  = "/usr/bin"
		expectedCount = 200
	)
	p := Params{
		"path":    expectedPath,
		"enabled": true,
		"count":   expectedCount,
	}
	path := p.GetString("path")
	if path != expectedPath {
		t.Errorf("expected 'path' to be '%v', got '%v'", expectedPath, path)
	}
}

func TestRangeEnd(t *testing.T) {
	for _, test := range []struct {
		key, expected Key
	}{
		{
			key:      NewKey("abc"),
			expected: NewKey("abd"),
		},
		{
			key:      NewKey("foo", "bar"),
			expected: NewKey("foo", "bas"),
		},
		{
			key:      NewKey("xyz"),
			expected: NewKey("xy{"),
		},
		{
			key:      NewKey("\xFF"),
			expected: Key{s: "0", components: []string{"0"}},
		},
		{
			key:      NewKey("\xFF\xFF\xFF"),
			expected: Key{s: "0", components: []string{"0"}},
		},
	} {
		t.Run(test.key.String(), func(t *testing.T) {
			end := RangeEnd(test.key)
			require.Empty(t, cmp.Diff(test.expected, end, cmp.AllowUnexported(Key{})))
		})
	}
}
