/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"net"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/stretchr/testify/require"
)

func Test_websocketALPNClientConn(t *testing.T) {
	clientRawConn, serverRawConn := net.Pipe()
	t.Cleanup(func() {
		clientRawConn.Close()
		serverRawConn.Close()
	})

	clientConn := newWebSocketALPNClientConn(clientRawConn)

	t.Run("Read", func(t *testing.T) {
		wait := make(chan struct{}, 1)

		// Send a ping and some text from server.
		go func() {
			require.NoError(t, ws.WriteFrame(serverRawConn, ws.NewPingFrame([]byte("foo"))))
			frame, err := ws.ReadFrame(serverRawConn)
			require.NoError(t, err)
			require.Equal(t, ws.OpPong, frame.Header.OpCode)
			require.NoError(t, ws.WriteFrame(serverRawConn, ws.NewBinaryFrame([]byte("hello client"))))
			wait <- struct{}{}
		}()

		mustReadWebsocketALPNClientConn(t, clientConn, "hello c")
		mustReadWebsocketALPNClientConn(t, clientConn, "lient")

		<-wait
	})

	t.Run("Write", func(t *testing.T) {
		wait := make(chan struct{}, 1)
		text := "hello server"

		go func() {
			n, err := clientConn.Write([]byte(text))
			require.NoError(t, err)
			require.Equal(t, len(text), n)
			wait <- struct{}{}
		}()

		wantFrame := ws.NewBinaryFrame([]byte(text))
		wantFrame.Header.Masked = true

		actualFrame, err := ws.ReadFrame(serverRawConn)
		require.NoError(t, err)
		require.Equal(t, wantFrame, actualFrame)

		<-wait
	})
}

func mustReadWebsocketALPNClientConn(t *testing.T, conn *websocketALPNClientConn, wantText string) {
	t.Helper()

	actualTextChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		readBuff := make([]byte, len(wantText))
		_, err := conn.Read(readBuff)
		if err != nil {
			errChan <- err
		} else {
			actualTextChan <- string(readBuff)
		}
	}()

	select {
	case actualText := <-actualTextChan:
		require.Equal(t, wantText, actualText)
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for %v from Read", wantText)
	}
}
