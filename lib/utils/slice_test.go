// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	for i := 0; i < 10; i++ {
		slice := pool.Get()
		require.Len(t, slice, 1024, "Returned slice should have zero len and values")
		for i := range slice {
			require.Equal(t, slice[i], byte(0), "Each slice element is zero byte")
		}
		copy(slice, []byte("just something to fill with"))
		pool.Put(slice)
	}
}
