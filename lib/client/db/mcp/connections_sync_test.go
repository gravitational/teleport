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

//go:build go1.24 && enablesynctest

package mcp

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

/*
This file uses the experimental testing/synctest package introduced with Go 1.24:

    https://go.dev/blog/synctest

When editing this file, you should set GOEXPERIMENT=synctest for your editor/LSP
to ensure that the language server doesn't fail to recognize the package.

This file is also protected by a build tag to ensure that `go test` doesn't fail
for users who haven't set the environment variable.
*/

func TestConnectionsManagerIdle(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		manager, err := NewConnectionsManager(t.Context(), &ConnectionsManagerConfig{
			MaxIdleTime: maxIdleTime,
			Logger:      utils.NewSlogLoggerForTests(),
		}, func(ctx context.Context, id string) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)
		defer manager.Close(t.Context())

		conn1, err := manager.Get(t.Context(), "first")
		require.NoError(t, err)
		require.False(t, conn1.Conn().closed.Load(), "expected connection to be active")

		// Retrieving during the active period should block and only return once
		// the other active connection is released.
		var releasedConn *fakeConn
		go func() {
			conn2, err := manager.Get(t.Context(), "first")
			assert.NoError(t, err)
			assert.False(t, conn2.Conn().closed.Load(), "expected connection to be active")
			conn2.Release()
			releasedConn = conn2.Conn()
		}()

		conn1.Release()
		synctest.Wait()

		// Each database identifier should have its own active connection
		conn3, err := manager.Get(t.Context(), "second")
		require.NoError(t, err)
		require.False(t, conn1.Conn().closed.Load(), "expected connection to be active")

		// Advancing in time should cause all released connections to be closed.
		time.Sleep(maxIdleTime * 2)
		synctest.Wait()
		require.True(t, releasedConn.closed.Load(), "expected connection to be closed")
		require.False(t, conn3.Conn().closed.Load(), "expected connection to be active") // conn3 wasn't released

		// Advance in time to get all remaining closed.
		conn3.Release()
		time.Sleep(maxIdleTime * 2)
		synctest.Wait()
		require.True(t, conn3.Conn().closed.Load(), "expected connection to be closed")

		// After the connections are closed, fetching connections should bring
		// brand new connections.
		conn4, err := manager.Get(t.Context(), "first")
		require.NoError(t, err)
		require.False(t, conn4.Conn().closed.Load(), "expected connection to be active")
		conn5, err := manager.Get(t.Context(), "second")
		require.NoError(t, err)
		require.False(t, conn5.Conn().closed.Load(), "expected connection to be active")

		// Close should wait until all connections are released.
		go func() {
			assert.NoError(t, manager.Close(t.Context()))
		}()

		conn4.Release()
		conn5.Release()
		synctest.Wait()

		_, err = manager.Get(t.Context(), "first")
		require.Error(t, err)
	})
}
