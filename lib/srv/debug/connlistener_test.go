// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package debug

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnListener_AcceptAndHandle(t *testing.T) {
	l := NewConnListener()
	defer l.Close()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go l.HandleConnection(server)

	conn, err := l.Accept()
	require.NoError(t, err)
	assert.Equal(t, server, conn)
}

func TestConnListener_AcceptAfterClose(t *testing.T) {
	l := NewConnListener()
	l.Close()

	conn, err := l.Accept()
	assert.Nil(t, conn)
	assert.Error(t, err)
}

func TestConnListener_HandleConnectionAfterClose(t *testing.T) {
	l := NewConnListener()
	l.Close()

	server, _ := net.Pipe()
	// HandleConnection on a closed listener should close the conn.
	l.HandleConnection(server)

	// Verify the connection was closed by trying to write to it.
	_, err := server.Write([]byte("test"))
	assert.Error(t, err)
}

func TestConnListener_CloseIdempotent(t *testing.T) {
	l := NewConnListener()
	require.NoError(t, l.Close())
	// Second close should not panic.
	require.NoError(t, l.Close())
}

func TestConnListener_Addr(t *testing.T) {
	l := NewConnListener()
	defer l.Close()
	addr := l.Addr()
	require.NotNil(t, addr)
	assert.Equal(t, "tcp", addr.Network())
}

func TestConnListener_AcceptBlocksUntilHandle(t *testing.T) {
	l := NewConnListener()
	defer l.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := l.Accept()
		accepted <- conn
	}()

	// Verify Accept is blocking.
	select {
	case <-accepted:
		t.Fatal("Accept should block until HandleConnection is called")
	case <-time.After(50 * time.Millisecond):
	}

	server, client := net.Pipe()
	defer client.Close()
	l.HandleConnection(server)

	select {
	case conn := <-accepted:
		assert.Equal(t, server, conn)
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not return after HandleConnection")
	}
}
