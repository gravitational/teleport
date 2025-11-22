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

package mcputils

import (
	"fmt"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/stretchr/testify/require"
)

func TestIDTracker(t *testing.T) {
	tracker, err := NewIDTracker(5)
	require.NoError(t, err)
	require.Empty(t, tracker.Len())

	t.Run("request missing ID not tracked", func(t *testing.T) {
		require.False(t, tracker.PushRequest(&jsonrpc.Request{
			Method: "bad",
		}))
		require.Empty(t, tracker.Len())
	})

	t.Run("request tracked", func(t *testing.T) {
		require.True(t, tracker.PushRequest(&jsonrpc.Request{
			ID:     mustMakeIntID(t, 0),
			Method: MethodToolsList,
		}))
		require.Equal(t, 1, tracker.Len())
	})

	t.Run("pop unknown id", func(t *testing.T) {
		unknownIDs := []jsonrpc.ID{
			mustMakeIntID(t, 5),
			mustMakeID(t, "0"),
			mustMakeID(t, nil),
		}
		for id := range slices.Values(unknownIDs) {
			t.Run(fmt.Sprintf("%T", id), func(t *testing.T) {
				_, ok := tracker.PopByID(id)
				require.False(t, ok)
				require.Equal(t, 1, tracker.Len())
			})
		}
	})

	t.Run("pop tracked id", func(t *testing.T) {
		method, ok := tracker.PopByID(mustMakeIntID(t, 0))
		require.True(t, ok)
		require.Equal(t, MethodToolsList, method)
		require.Empty(t, tracker.Len())
	})

	t.Run("track last 5", func(t *testing.T) {
		for i := range 20 {
			tracker.PushRequest(&jsonrpc.Request{
				ID:     mustMakeIntID(t, i+1),
				Method: MethodToolsCall,
			})
			require.LessOrEqual(t, tracker.Len(), 10)
		}
		for i := range 5 {
			method, ok := tracker.PopByID(mustMakeIntID(t, 20-i))
			require.True(t, ok)
			require.Equal(t, MethodToolsCall, method)
		}
		require.Empty(t, tracker.Len())
	})
}

func mustMakeID(t testing.TB, id any) jsonrpc.ID {
	t.Helper()
	ret, err := jsonrpc.MakeID(id)
	require.NoError(t, err)
	return ret
}

func mustMakeIntID(t testing.TB, id int) jsonrpc.ID {
	t.Helper()
	return mustMakeID(t, float64(id))
}

func BenchmarkIDTracker(b *testing.B) {
	idTracker, err := NewIDTracker(100)
	require.NoError(b, err)

	for i := range 100 {
		idTracker.PushRequest(&jsonrpc.Request{
			ID:     mustMakeIntID(b, i),
			Method: MethodToolsList,
		})
	}
	testID := mustMakeIntID(b, 2000)

	// cpu: Apple M3 Pro
	// BenchmarkIDTracker-12    	12267649	        81.85 ns/op
	for b.Loop() {
		idTracker.PushRequest(&jsonrpc.Request{
			ID:     testID,
			Method: MethodToolsList,
		})
		idTracker.PopByID(testID)
	}
}
