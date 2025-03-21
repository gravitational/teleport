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

package delay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeapBasics(t *testing.T) {
	var heap tickHeap[int]

	t1 := time.Now()
	heap.Push(&entry[int]{tick: t1, key: 1})

	t2 := time.Now()
	heap.Push(&entry[int]{tick: t2, key: 2})

	require.Equal(t, &entry[int]{tick: t1, key: 1}, heap.Pop())
	require.Equal(t, &entry[int]{tick: t2, key: 2}, heap.Pop())

	for i := 0; i < 100; i++ {
		heap.Push(&entry[int]{tick: time.Now(), key: i})
	}

	var prev *entry[int]
	for i := 0; i < 100; i++ {
		next := heap.Pop()
		if prev != nil {
			require.True(t, prev.tick.Before(next.tick))
		}
		require.Equal(t, i, next.key)
		prev = next
	}

	require.Zero(t, heap.Len())
}
