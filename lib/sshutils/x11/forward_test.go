// Copyright 2022 Gravitational, Inc
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
		l, err := net.Listen("tcp", ":0")
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
