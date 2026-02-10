// Copyright 2025 Gravitational, Inc.
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

package backendtest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestRandomlyErroringBackend(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mem, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)

	var key, value = backend.NewKey("test_key_1"), []byte("test_value_1")

	randomlyErroringBackend := NewRandomlyErroringBackend(mem)
	_, err = randomlyErroringBackend.Put(ctx, backend.Item{Key: key, Value: value})
	require.NoError(t, err)

	const iterations, minBoundary, maxBoundary = 200, 50, 150

	// Get
	randomErrCnt := 0
	for range iterations {
		item, err := randomlyErroringBackend.Get(ctx, key)
		if err != nil {
			require.ErrorIs(t, err, ErrRandomBackend)
			randomErrCnt++
		} else {
			require.Equal(t, key, item.Key)
			require.Equal(t, value, item.Value)
		}
	}
	require.GreaterOrEqual(t, randomErrCnt, minBoundary)
	require.LessOrEqual(t, randomErrCnt, maxBoundary)

	// GetRange
	randomErrCnt = 0
	for range iterations {
		res, err := randomlyErroringBackend.GetRange(ctx, key, key, 100)
		if err != nil {
			require.ErrorIs(t, err, ErrRandomBackend)
			randomErrCnt++
		} else {
			require.Len(t, res.Items, 1)
			item := res.Items[0]
			require.Equal(t, key, item.Key)
			require.Equal(t, value, item.Value)
		}
	}
	require.GreaterOrEqual(t, randomErrCnt, minBoundary)
	require.LessOrEqual(t, randomErrCnt, maxBoundary)

	// Items
	randomErrCnt = 0
	for range iterations {
		for item, err := range randomlyErroringBackend.Items(ctx, backend.ItemsParams{StartKey: key, EndKey: key, Limit: 100}) {
			if err != nil {
				require.ErrorIs(t, err, ErrRandomBackend)
				randomErrCnt++
			} else {
				require.Equal(t, key, item.Key)
				require.Equal(t, value, item.Value)
			}
		}
	}
	require.GreaterOrEqual(t, randomErrCnt, minBoundary)
	require.LessOrEqual(t, randomErrCnt, maxBoundary)
}
