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

package listener

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryListenerClient(t *testing.T) {
	var wg sync.WaitGroup
	expectedMessage := "hello from server"

	listener := NewInMemoryListener()
	t.Cleanup(func() { listener.Close() })

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		t.Cleanup(func() { conn.Close() })

		_, _ = conn.Write([]byte(expectedMessage))
	}()

	// To avoid blocking in case the server is not working correctly, wrap
	// the client connection into a eventually loop.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		conn, err := listener.DialContext(t.Context(), "", "")
		require.NoError(collect, err)

		buf := make([]byte, len(expectedMessage))
		n, err := conn.Read(buf[0:])
		require.NoError(collect, err)
		require.Equal(collect, len(expectedMessage), n)
		require.Equal(collect, expectedMessage, string(buf[:n]))
	}, 50*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		wg.Wait()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond)
}

func TestMemoryListenerServer(t *testing.T) {
	var wg sync.WaitGroup
	expectedMessage := "hello from client"

	listener := NewInMemoryListener()
	t.Cleanup(func() { listener.Close() })

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.DialContext(t.Context(), "", "")
		if err != nil {
			return
		}

		_, _ = conn.Write([]byte(expectedMessage))
	}()

	// To avoid blocking in case the client is not working correctly, wrap
	// the server accept connection into a eventually loop.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		conn, err := listener.Accept()
		require.NoError(collect, err)

		buf := make([]byte, len(expectedMessage))
		n, err := conn.Read(buf[0:])
		require.NoError(collect, err)
		require.Equal(collect, len(expectedMessage), n)
		require.Equal(collect, expectedMessage, string(buf[:n]))
	}, 50*time.Millisecond, 10*time.Millisecond)

	// Close the listener and expect subsequent accept calls to return error.
	listener.Close()
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := listener.Accept()
		require.Error(collect, err)
		require.ErrorIs(collect, err, io.EOF)
	}, 50*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		wg.Wait()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond)
}

func TestMemoryListenerDialTimeout(t *testing.T) {
	listener := NewInMemoryListener()
	t.Cleanup(func() { listener.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := listener.DialContext(ctx, "", "")
		require.Error(collect, err)
		require.ErrorIs(collect, err, context.DeadlineExceeded)
	}, 100*time.Millisecond, 10*time.Millisecond)

	require.Never(t, func() bool {
		_, _ = listener.Accept()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond, "expected server to not have received connections")
}
