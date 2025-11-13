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

package itertools_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/itertools"
)

func TestDynamicBatchSize(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		var got [][]int
		chunks := itertools.DynamicBatchSize(items, 3)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
		}
		require.Equal(t, [][]int{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
			{10},
		}, got)
	})

	t.Run("reduce every second iteration", func(t *testing.T) {
		items := sequence(62)
		var got [][]int
		i := 0
		chunks := itertools.DynamicBatchSize(items, 30)
		for chunks.Next() {
			chunk := chunks.Chunk()
			i++
			if i%2 == 0 && len(chunk) > 1 {
				require.NoError(t, chunks.ReduceSize())
				continue
			}
			got = append(got, chunk)
		}
		require.Equal(t, [][]int{
			{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30},
			{31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45},
			{46, 47, 48, 49, 50, 51, 52, 53},
			{54, 55, 56, 57},
			{58, 59},
			{60},
			{61},
			{62},
		}, got)
	})

	t.Run("empty slice", func(t *testing.T) {
		chunks := itertools.DynamicBatchSize([]int{}, 10)
		for chunks.Next() {
			require.Fail(t, "should not be called")
		}
	})

	t.Run("single item", func(t *testing.T) {
		items := []int{42}
		var got [][]int
		chunks := itertools.DynamicBatchSize(items, 10)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
		}
		require.Equal(t, [][]int{{42}}, got)
	})

	t.Run("batch size of 1", func(t *testing.T) {
		items := sequence(3)
		var got [][]int

		chunks := itertools.DynamicBatchSize(items, 1)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
		}
		require.Equal(t, [][]int{{1}, {2}, {3}}, got)
	})

	t.Run("batch size larger than items", func(t *testing.T) {
		items := sequence(5)
		var got [][]int
		chunks := itertools.DynamicBatchSize(items, 100)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
		}
		require.Equal(t, [][]int{{1, 2, 3, 4, 5}}, got)
	})

	t.Run("cannot reduce batch size of 1", func(t *testing.T) {
		items := []int{1}
		chunks := itertools.DynamicBatchSize(items, 10)
		for chunks.Next() {
			err := chunks.ReduceSize()
			require.ErrorIs(t, err, itertools.ErrCannotReduceBatchSize)
		}
	})

	t.Run("break after first batch", func(t *testing.T) {
		items := sequence(100)
		var got [][]int
		chunks := itertools.DynamicBatchSize(items, 10)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
			break
		}
		require.Equal(t, [][]int{{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}, got)
	})

	t.Run("invalid size", func(t *testing.T) {
		items := sequence(10)
		var got [][]int
		chunks := itertools.DynamicBatchSize(items, 0)
		for chunks.Next() {
			chunk := chunks.Chunk()
			got = append(got, chunk)
		}
		// When chunk size is invalid (0 or negative), it defaults to 1
		require.Equal(t, [][]int{
			{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}, {9}, {10},
		}, got)
	})
}

func sequence(n int) []int {
	items := make([]int, n)
	for i := range items {
		items[i] = i + 1
	}
	return items
}
