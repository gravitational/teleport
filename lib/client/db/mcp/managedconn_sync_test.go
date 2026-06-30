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
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
This file uses the experimental testing/synctest package introduced with Go 1.24:

    https://go.dev/blog/synctest

When editing this file, you should set GOEXPERIMENT=synctest for your editor/LSP
to ensure that the language server doesn't fail to recognize the package.

This file is also protected by a build tag to ensure that `go test` doesn't fail
for users who haven't set the environment variable.
*/

func TestManagedConnExec(t *testing.T) {
	synctest.Run(func() {
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: time.Minute}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// Regular executions, acquire a connection and return the exec results.
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)

		// Perform some "heavy work" which will keep the connection occupied.
		// go func() {
		go func() {
			_, err := Exec(t.Context(), managedConn, fakeConnExec(time.Minute))
			assert.NoError(t, err)
		}()

		synctest.Wait()

		// Any connection that happens during this should be blocked until the
		// other is released (`Exec` completes).
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)

		// If the connection is closed outside, the next execution will fail due
		// to trying to use a closed connection. It is up to the caller to deal
		// with this, as they can cause the connection interruption.
		require.NotNil(t, managedConn.active, "expected active connection")
		managedConn.active.shouldClose.Store(true)
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.Error(t, err)
	})
}

func TestManagedConnIdle(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: maxIdleTime}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// First execution will init new connection.
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)
		require.NotNil(t, managedConn.active, "expected active connection")

		// Advance in time to "expire" the connection.
		time.Sleep(maxIdleTime + 1)
		synctest.Wait()
		require.Nil(t, managedConn.active, "expected connection to be active")

		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err, "expected the execution to succeed after expired connection")
	})
}

func TestManagedConnLongExec(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: maxIdleTime}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// Have a long execution that exceeds the idle time.
		timeToComplete := 2 * maxIdleTime
		go func() {
			_, err := Exec(t.Context(), managedConn, fakeConnExec(maxIdleTime+timeToComplete))
			assert.NoError(t, err)
			assert.NotNil(t, managedConn.active, "expected active connection")
		}()

		// Advance in time to "expire" the connection.
		time.Sleep(maxIdleTime + 1)
		synctest.Wait()
		require.NotNil(t, managedConn.active, "expected connection to be active")

		time.Sleep(timeToComplete + 1)
		synctest.Wait()
		require.Nil(t, managedConn.cancelExec, "expected execution to have completed")
		require.NotNil(t, managedConn.active, "expected connection to be kept active")

		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err, "expected the execution to succeed after expired connection")
	})
}

func TestManagedConnClose(t *testing.T) {
	synctest.Run(func() {
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: time.Minute}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// First execution will init new connection.
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)

		require.NoError(t, managedConn.Close(t.Context()))
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.Error(t, err)
	})
}

// TestManagedConnCloseCancelExec given a managed connection that has a long
// running execution, when the Close context is done, it must close the
// execution context.
func TestManagedConnCloseWhileInUse(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: maxIdleTime}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// First execution will init new connection.
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)

		// Long running execution will take place.
		go func() {
			_, err := Exec(t.Context(), managedConn, fakeConnExec(time.Minute))
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
		}()

		synctest.Wait()

		// Close should cancel executions and terminate connections.
		require.NoError(t, managedConn.Close(t.Context()))
		require.NotNil(t, managedConn.active)
		require.True(t, managedConn.active.closed.Load(), "expected connection to be closed")

		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.Error(t, err, "expected executions with closed connection to return error")
	})
}

// TestManagedConnCloseBusy given a managed connection that has a long
// running execution, and another execution waiting. When the Close context is
// done, and the waiting execution should return is closed error.
func TestManagedConnCloseBusy(t *testing.T) {
	synctest.Run(func() {
		maxIdleTime := time.Minute
		managedConn, err := NewManagedConn(&ManagedConnConfig{MaxIdleTime: maxIdleTime}, func(ctx context.Context) (*fakeConn, error) {
			return &fakeConn{}, nil
		})
		require.NoError(t, err)

		// First execution will init new connection.
		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.NoError(t, err)

		// Long running execution will take place.
		go func() {
			_, err := Exec(t.Context(), managedConn, fakeConnExec(time.Minute))
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
		}()

		synctest.Wait()

		// Start another execution, this will be in waiting state (since there
		// is one execution running). We'll trigger the close before it gets
		// a chance to get executed, so we expect it to return error.
		go func() {
			_, err := Exec(t.Context(), managedConn, fakeConnExec(time.Minute))
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrConnClosed)
		}()

		synctest.Wait()

		// Close should cancel executions and terminate connections.
		require.NoError(t, managedConn.Close(t.Context()))
		require.NotNil(t, managedConn.active)
		require.True(t, managedConn.active.closed.Load(), "expected connection to be closed")

		_, err = Exec(t.Context(), managedConn, fakeConnExec(0))
		require.Error(t, err, "expected executions with closed connection to return error")
	})
}

func fakeConnExec(d time.Duration) func(context.Context, *fakeConn) (any, error) {
	return func(ctx context.Context, conn *fakeConn) (any, error) {
		if conn.Exec() {
			return nil, trace.AccessDenied("connection closed")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}

		return nil, nil
	}
}

type fakeConn struct {
	closed      atomic.Bool
	shouldClose atomic.Bool
}

func (t *fakeConn) Exec() bool {
	if t.shouldClose.Load() {
		_ = t.Close(context.TODO())
	}

	return t.closed.Load()
}

func (t *fakeConn) Close(_ context.Context) error {
	t.closed.Store(true)
	return nil
}

func (t *fakeConn) IsClosed() bool {
	return t.closed.Load()
}
