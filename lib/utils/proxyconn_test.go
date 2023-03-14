/*
Copyright 2022 Gravitational, Inc.

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

package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
	log      logrus.FieldLogger
}

func newEchoServer() (*echoServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &echoServer{
		listener: listener,
		log:      logrus.WithField(trace.Component, "echo"),
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
	s.log.Infof("Received message: %s.", b)

	_, err = conn.Write(b)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
