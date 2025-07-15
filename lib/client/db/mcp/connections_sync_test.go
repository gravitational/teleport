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

func TestIdleConnectionsCheckerIdle(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		checker, err := NewIdleConnectionsChecker(t.Context(), &IdleConnectionsCheckerConfig{
			MaxIdleTime: maxIdleTime,
			Logger:      utils.NewSlogLoggerForTests(),
		}, func(ctx context.Context, id string) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)
		defer checker.Close(t.Context())

		conn1, err := checker.Get(t.Context(), "first")
		require.NoError(t, err)
		require.False(t, conn1.closed.Load(), "expected connection to be active")

		// Retrieving during the active period should return the same connection.
		conn2, err := checker.Get(t.Context(), "first")
		require.NoError(t, err)
		require.False(t, conn1.closed.Load(), "expected connection to be active")
		require.Same(t, conn1, conn2, "expected to be the exact same connection but got different")

		// Each database identifier should have its own active connection
		conn3, err := checker.Get(t.Context(), "second")
		require.NoError(t, err)
		require.False(t, conn1.closed.Load(), "expected connection to be active")
		require.NotSame(t, conn3, conn1, "expected to be be different connection but got the same")

		// Advancing in time should cause all inactive connections to be closed.
		time.Sleep(maxIdleTime + 1)
		synctest.Wait()
		require.True(t, conn1.closed.Load(), "expected connection to be closed")
		require.True(t, conn2.closed.Load(), "expected connection to be closed")
		require.True(t, conn3.closed.Load(), "expected connection to be closed")

		// After the connections are closed, fetching connections should bring
		// brand new connections.
		conn4, err := checker.Get(t.Context(), "first")
		require.NoError(t, err)
		require.False(t, conn4.closed.Load(), "expected connection to be active")
		require.NotSame(t, conn4, conn1)
		conn5, err := checker.Get(t.Context(), "second")
		require.NoError(t, err)
		require.False(t, conn5.closed.Load(), "expected connection to be active")
		require.NotSame(t, conn5, conn3)
	})
}
