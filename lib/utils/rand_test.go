// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShuffleVisit(t *testing.T) {
	const N = 1000
	s := make([]int, 0, N)
	for i := range N {
		s = append(s, i)
	}

	var out []int
	for i, v := range ShuffleVisit(s) {
		require.Equal(t, len(out), i)
		out = append(out, v)
		require.Equal(t, out, s[:len(out)])
	}
	require.Len(t, out, N)

	var fixed int
	for i, v := range s {
		if i == v {
			fixed++
		}
	}
	// this is a check with a HUGE margin of error, seeing that a perfect
	// shuffle of 1000 items would have a 1 in 99_524_607 chance of having more
	// than 10 fixed points, and the chance to have 500 or more is 1 in
	// 3.02e1134
	require.Less(t, fixed, N/2)
}
