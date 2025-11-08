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

package itertools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicIteration(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	var batches [][]int
	for batch := range DynamicBatchSize(items, 3) {
		batches = append(batches, append([]int(nil), batch.Items...))
	}

	require.Equal(t, [][]int{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
		{10},
	}, batches)
}

func TestReduceBatchSizeTillLimitIsReached(t *testing.T) {
	items := sequence(100)

	expectedSizes := []int{20, 10, 5, 3, 2, 1}
	var actualSizes []int

	for batch := range DynamicBatchSize(items, 20) {
		actualSizes = append(actualSizes, len(batch.Items))

		if len(batch.Items) != 1 {
			err := batch.ReduceBatchSizeByHalf()
			require.NoError(t, err)
			continue
		}

		// When we reach batch size of 1, attempt to reduce further should return an error
		// that indicates we can't reduce further.
		err := batch.ReduceBatchSizeByHalf()
		require.Error(t, err, "should not be able to reduce batch size of 1")
		break
	}

	require.Equal(t, expectedSizes, actualSizes)
}

func TestReduceSizeByHalfEvySecondIter(t *testing.T) {
	items := sequence(62)

	var batches [][]int
	i := 0

	for batch := range DynamicBatchSize(items, 30) {
		i++
		if i%2 == 0 && len(batch.Items) > 1 {
			require.NoError(t, batch.ReduceBatchSizeByHalf())
			continue
		}
		batches = append(batches, append([]int(nil), batch.Items...))
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
	}, batches)
}

func sequence(n int) []int {
	items := make([]int, n)
	for i := range items {
		items[i] = i + 1
	}
	return items
}
