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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSlice tests sync pool holding slices - SliceSyncPool
func TestSlice(t *testing.T) {
	t.Parallel()

	pool := NewSliceSyncPool(1024)
	// having a loop is not a guarantee that the same slice
	// will be reused, but a good enough bet
	for range 10 {
		slice := pool.Get()
		require.Len(t, slice, 1024, "Returned slice should have zero len and values")
		for i := range slice {
			require.Equal(t, slice[i], byte(0), "Each slice element is zero byte")
		}
		copy(slice, []byte("just something to fill with"))
		pool.Put(slice)
	}
}
