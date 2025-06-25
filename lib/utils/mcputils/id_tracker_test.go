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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestIDTracker(t *testing.T) {
	tracker, err := NewIDTracker(5)
	require.NoError(t, err)
	require.Empty(t, tracker.Len())

	t.Run("request missing ID not tracked", func(t *testing.T) {
		require.False(t, tracker.PushRequest(&JSONRPCRequest{
			Method: "bad",
		}))
		require.Empty(t, tracker.Len())
	})

	t.Run("request tracked", func(t *testing.T) {
		require.True(t, tracker.PushRequest(&JSONRPCRequest{
			ID:     mcp.NewRequestId(0),
			Method: mcp.MethodToolsList,
		}))
		require.Equal(t, 1, tracker.Len())
	})

	t.Run("pop unknown id", func(t *testing.T) {
		unknownIDs := []mcp.RequestId{
			mcp.NewRequestId(5),
			mcp.NewRequestId("0"),
			mcp.NewRequestId(nil),
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
		method, ok := tracker.PopByID(mcp.NewRequestId(0))
		require.True(t, ok)
		require.Equal(t, mcp.MethodToolsList, method)
		require.Empty(t, tracker.Len())
	})

	t.Run("track last 5", func(t *testing.T) {
		for i := range 20 {
			tracker.PushRequest(&JSONRPCRequest{
				ID:     mcp.NewRequestId(i + 1),
				Method: mcp.MethodToolsCall,
			})
			require.LessOrEqual(t, tracker.Len(), 10)
		}
		for i := range 5 {
			method, ok := tracker.PopByID(mcp.NewRequestId(20 - i))
			require.True(t, ok)
			require.Equal(t, mcp.MethodToolsCall, method)
		}
		require.Empty(t, tracker.Len())
	})
}

func BenchmarkIDTracker(b *testing.B) {
	idTracker, err := NewIDTracker(100)
	require.NoError(b, err)

	for i := range 100 {
		idTracker.PushRequest(&JSONRPCRequest{
			ID:     mcp.NewRequestId(i),
			Method: mcp.MethodToolsList,
		})
	}

	// cpu: Apple M3 Pro
	// BenchmarkIDTracker-12    	12267649	        81.85 ns/op
	for b.Loop() {
		idTracker.PushRequest(&JSONRPCRequest{
			ID:     mcp.NewRequestId(2000),
			Method: mcp.MethodToolsList,
		})
		idTracker.PopByID(mcp.NewRequestId(2000))
	}
}
