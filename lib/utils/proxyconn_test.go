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

package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

// TestProxyConn tests proxying the connection between client and server.
func TestProxyConn(t *testing.T) {
	ctx := context.Background()

	echoServer, err := newEchoServer()
	require.NoError(t, err)
	go echoServer.Start()
	t.Cleanup(func() { echoServer.Close() })

	echoConn, err := net.Dial("tcp", echoServer.Addr())
	require.NoError(t, err)
	// Connection will be closed below.

	// Simulate the client connection with pipe.
	clientLeft, clientRight := net.Pipe()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := ProxyConn(ctx, clientRight, echoConn)
		if err != nil && !strings.Contains(err.Error(), io.ErrClosedPipe.Error()) {
			errCh <- err
		}
	}()

	// Send message to the echo server through the proxy and expect to get
	// the same one back.
	sent := uuid.NewString()
	_, err = clientLeft.Write([]byte(sent))
	require.NoError(t, err)

	received := make([]byte, 36)
	_, err = clientLeft.Read(received)
	require.NoError(t, err)
	require.Equal(t, sent, string(received))
	fmt.Println(string(received))

	// Close the server connection and make sure the proxy loop exits.
	echoConn.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("proxy loop didn't exit after 1s")
	}
}

// TestProxyConnCancel verifies context cancellation for the proxy loop.
func TestProxyConnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	echoServer, err := newEchoServer()
	require.NoError(t, err)
	go echoServer.Start()
	t.Cleanup(func() { echoServer.Close() })

	echoConn, err := net.Dial("tcp", echoServer.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { echoConn.Close() })

	_, clientRight := net.Pipe()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		errCh <- ProxyConn(ctx, clientRight, echoConn)
	}()

	// Cancel the context and make sure the proxy loop exits.
	cancel()
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("proxy loop didn't exit after 1s")
	}
}

type echoServer struct {
	listener net.Listener
	log      *slog.Logger
}

func newEchoServer() (*echoServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &echoServer{
		listener: listener,
		log:      slog.With(teleport.ComponentKey, "echo"),
	}, nil
}

func (s *echoServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *echoServer) Close() error {
	return s.listener.Close()
}

func (s *echoServer) Start() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return trace.Wrap(err)
		}
		go s.handleConn(conn)
		// Don't close the connection to let the proxy handle it.
	}
}

func (s *echoServer) handleConn(conn net.Conn) error {
	b := make([]byte, 36) // expect to receive UUID from the test
	_, err := conn.Read(b)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.InfoContext(context.Background(), "Received message", "receieved_message", string(b))

	_, err = conn.Write(b)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
