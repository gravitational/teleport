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
	t.Parallel()
	heap := heap[entry[int]]{
		Less: entryLess[int],
	}

	now := time.Now()

	t1 := now.Add(time.Millisecond)
	heap.Push(entry[int]{tick: t1, key: 1})

	t2 := now.Add(time.Millisecond * 2)
	heap.Push(entry[int]{tick: t2, key: 2})

	require.Equal(t, entry[int]{tick: t1, key: 1}, heap.Pop())
	require.Equal(t, entry[int]{tick: t2, key: 2}, heap.Pop())

	for i := range 100 {
		ts := now.Add(time.Duration(i+1) * time.Millisecond)
		heap.Push(entry[int]{tick: ts, key: i})
	}

	root := heap.Root()
	require.NotNil(t, root)
	require.Equal(t, 0, root.key)
	root.tick = now.Add(time.Hour)
	heap.FixRoot()

	newRoot := heap.Root()
	require.NotNil(t, newRoot)
	require.Equal(t, 1, newRoot.key)

	// 43ms -> 22ms replacement should be sifted up
	heap.Slice[42].tick = now.Add(22 * time.Millisecond)
	heap.Fix(42)
	// 14ms -> 33ms replacement should be sifted down
	heap.Slice[13].tick = now.Add(33 * time.Millisecond)
	heap.Fix(13)
	var prev *entry[int]
	for range 100 {
		next := heap.Pop()
		if prev != nil {
			require.False(t, next.tick.Before(prev.tick), "prev: %v, next: %v", prev, next)
		}
		prev = &next
	}

	require.Empty(t, heap.Slice)
}
