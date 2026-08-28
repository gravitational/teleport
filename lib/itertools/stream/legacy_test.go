/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package stream

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	legacy "github.com/gravitational/teleport/api/internalutils/stream"
)

func TestLegacyCompat(t *testing.T) {
	t.Parallel()

	var s []int
	var err error

	// basic from legacy compat & use as a standard stream
	for n, err := range FromLegacy(legacy.Slice([]int{1, 2, 3, 4})) {
		require.NoError(t, err)
		s = append(s, n)
	}
	require.Equal(t, []int{1, 2, 3, 4}, s)

	// basic into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		for i := 0; i < 4; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}))
	require.NoError(t, err)
	require.Equal(t, []int{0, 1, 2, 3}, s)

	// nested compat
	s, err = legacy.Collect(IntoLegacy(FromLegacy(legacy.Slice([]int{7, 8, 9}))))
	require.NoError(t, err)
	require.Equal(t, []int{7, 8, 9}, s)

	// single-element from legacy compat
	s = nil
	for n, err := range FromLegacy(legacy.Slice([]int{11})) {
		require.NoError(t, err)
		s = append(s, n)
	}
	require.Equal(t, []int{11}, s)

	// single-element into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		if !yield(111, nil) {
			return
		}
	}))
	require.NoError(t, err)
	require.Equal(t, []int{111}, s)

	// empty from legacy compat
	s = nil
	for n, err := range FromLegacy(legacy.Empty[int]()) {
		require.NoError(t, err)
		s = append(s, n)
	}

	// empty into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {}))
	require.NoError(t, err)
	require.Empty(t, s)

	// error from legacy compat
	err = nil
	for _, ierr := range FromLegacy(legacy.Fail[int](fmt.Errorf("unexpected error"))) {
		err = ierr
		break
	}
	require.Error(t, err)

	// error into legacy compat
	s, err = legacy.Collect(IntoLegacy(func(yield func(int, error) bool) {
		yield(0, fmt.Errorf("unexpected error"))
	}))
	require.Error(t, err)
	require.Empty(t, s)

}
