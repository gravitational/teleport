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
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForward(t *testing.T) {
	ctx := context.Background()

	// Create a fake client display. In practice, the display
	// set in $DISPLAY is used to connect to the client display.
	fakeClientDisplay, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, fakeClientDisplay.Close())
	})

	// Handle connections to the client XServer
	echoMsg := "msg"
	go func() {
		for {
			localConn, err := fakeClientDisplay.Accept()
			if err != nil {
				// listener is closed, test is done.
				return
			}

			go func() {
				defer localConn.Close()

				// read request and expect what was written to server
				bytes, err := io.ReadAll(localConn)
				require.NoError(t, err)
				require.Equal(t, echoMsg, string(bytes))
			}()
		}
	}()

	// Create a fake XServer proxy just like the one in sshserver.
	sl, serverDisplay, err := OpenNewXServerListener(DefaultDisplayOffset, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sl.Close())
	})

	// Handle connection to XServer proxy
	go func() {
		for {
			serverConn, err := sl.Accept()
			if err != nil {
				// listener is closed, test is done.
				return
			}

			go func() {
				defer serverConn.Close()

				clientConn, err := net.Dial("tcp", fakeClientDisplay.Addr().String())
				if err != nil {
					// fakeClientDisplay is closed, test is done.
					return
				}

				clientXConn, ok := clientConn.(*net.TCPConn)
				require.True(t, ok)
				defer clientConn.Close()

				err = Forward(ctx, clientXConn, serverConn)
				require.NoError(t, err)
			}()
		}
	}()

	// Create a fake XServer request to the XServer proxy
	xreq, err := serverDisplay.Dial()
	require.NoError(t, err)
	_, err = xreq.Write([]byte(echoMsg))
	require.NoError(t, err)

	// Create a second request simultaneously
	xreq2, err := serverDisplay.Dial()
	require.NoError(t, err)
	_, err = xreq2.Write([]byte(echoMsg))
	require.NoError(t, err)
	xreq2.Close()

	// Close XServer requests, forwarding should stop as soon as
	// the open connection has been read and forwarded fully.
	xreq.Close()
	xreq2.Close()
}
