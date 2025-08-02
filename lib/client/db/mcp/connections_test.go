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

package mcp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestConnectionsManagerCloseAll(t *testing.T) {
	maxIdleTime := time.Minute
	manager, err := NewConnectionsManager(t.Context(), &ConnectionsManagerConfig{
		MaxIdleTime: maxIdleTime,
		Logger:      utils.NewSlogLoggerForTests(),
	}, func(ctx context.Context, id string) (*fakeConn, error) {
		return &fakeConn{}, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		// It is safe to close the checker multiple times.
		assert.NoError(t, manager.Close(t.Context()))
	})

	conn1, err := manager.Get(t.Context(), "first")
	require.NoError(t, err)
	require.False(t, conn1.Conn().closed.Load(), "expected connection to be active")
	conn2, err := manager.Get(t.Context(), "second")
	require.NoError(t, err)
	require.False(t, conn2.Conn().closed.Load(), "expected connection to be active")
	conn3, err := manager.Get(t.Context(), "third")
	require.NoError(t, err)
	require.False(t, conn3.Conn().closed.Load(), "expected connection to be active")

	conn1.Release()
	conn2.Release()
	conn3.Release()
	require.NoError(t, manager.Close(t.Context()))
	require.True(t, conn1.Conn().closed.Load(), "expected connection to be closed")
	require.True(t, conn2.Conn().closed.Load(), "expected connection to be closed")
	require.True(t, conn3.Conn().closed.Load(), "expected connection to be closed")

	_, err = manager.Get(t.Context(), "first")
	require.Error(t, err)
}

type fakeConn struct {
	closed atomic.Bool
}

func (t *fakeConn) Close(_ context.Context) error {
	t.closed.Store(true)
	return nil
}
