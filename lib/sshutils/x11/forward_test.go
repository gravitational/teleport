/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package x11

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForward(t *testing.T) {
	ctx := context.Background()

	// Open a dual sided connection on a new tcp listener
	openConn := func(t *testing.T) (clientConn net.Conn, serverConn net.Conn) {
		l, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, l.Close()) })

		serverErrC := make(chan error)
		serverConnC := make(chan net.Conn)
		go func() {
			serverConn, err := l.Accept()
			if err != nil {
				serverErrC <- err
				close(serverConnC)
			}
			serverConnC <- serverConn
			close(serverErrC)
		}()

		clientConn, err = net.Dial("tcp", l.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { clientConn.Close() })

		serverConn = <-serverConnC
		require.NoError(t, <-serverErrC)
		t.Cleanup(func() { serverConn.Close() })

		return clientConn, serverConn
	}

	cConn1, sConn1 := openConn(t)
	cConn2, sConn2 := openConn(t)

	// Start forwarding between connections so that we get
	// this flow: cConn1 -> sConn1 -> cConn2 -> sConn2.
	serverConnToForward, ok := sConn1.(*net.TCPConn)
	require.True(t, ok)
	clientConnToForward, ok := cConn2.(*net.TCPConn)
	require.True(t, ok)

	forwardErrC := make(chan error, 1)
	go func() {
		forwardErrC <- Forward(ctx, serverConnToForward, clientConnToForward)
	}()

	// Write a msg to client connection 1, which should propagate to server connection 2.
	message := "msg"
	_, err := cConn1.Write([]byte(message))
	require.NoError(t, err)

	buf := make([]byte, len(message))
	_, err = sConn2.Read(buf)
	require.NoError(t, err)
	require.Equal(t, message, string(buf))

	// Fowarding should stop once the other sides of the forwarded connections are closed.
	require.NoError(t, cConn1.Close())
	require.NoError(t, sConn2.Close())
	require.NoError(t, <-forwardErrC)
}
