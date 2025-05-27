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

package cache

import (
	"slices"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestResourceStore(t *testing.T) {
	store := newStore(
		func(i int) int { return i },
		map[string]func(i int) string{
			"numbers":    strconv.Itoa,
			"characters": func(i int) string { return strconv.FormatUint(uint64(i), 16) },
		})

	for i := 0; i < 100; i++ {
		require.NoError(t, store.put(i))
	}
	require.Equal(t, 100, store.len())

	zero, err := store.get("numbers", "0")
	require.NoError(t, err)
	require.Equal(t, 0, zero)

	n, err := store.get("numbers", "1000")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `no value for key "1000" in index numbers`})
	require.Equal(t, 0, n)

	v, err := store.get("characters", "1c")
	require.NoError(t, err)
	require.Equal(t, 28, v)

	out := slices.Collect(store.resources("numbers", "", ""))
	require.Len(t, out, 100)

	out = slices.Collect(store.resources("characters", "", ""))
	require.Len(t, out, 100)

	require.NoError(t, store.delete(0))
	_, err = store.get("numbers", "0")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `no value for key "0" in index numbers`})

	require.NoError(t, store.clear())

	_, err = store.get("numbers", "0")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `no value for key "0" in index numbers`})

	require.Zero(t, store.len())
}
