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

package tdp

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/bufconn"
)

// TestTDPConnTracksLocalRemoteAddrs verifies that a TDP connection
// uses the underlying local/remote addrs when available.
func TestTDPConnTracksLocalRemoteAddrs(t *testing.T) {
	local := &net.IPAddr{IP: net.ParseIP("192.168.1.2")}
	remote := &net.IPAddr{IP: net.ParseIP("192.168.1.3")}

	for _, test := range []struct {
		desc   string
		conn   io.ReadWriteCloser
		local  net.Addr
		remote net.Addr
	}{
		{
			desc: "implements srv.TrackingConn",
			conn: fakeTrackingConn{
				local:  local,
				remote: remote,
			},
			local:  local,
			remote: remote,
		},
		{
			desc:   "does not implement srv.TrackingConn",
			conn:   &fakeConn{Buffer: &bytes.Buffer{}},
			local:  nil,
			remote: nil,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			tc := NewConn(test.conn)
			l := tc.LocalAddr()
			r := tc.RemoteAddr()
			require.Equal(t, test.local, l)
			require.Equal(t, test.remote, r)
		})
	}
}

type fakeConn struct {
	*bytes.Buffer
}

func (t *fakeConn) Close() error { return nil }

type fakeTrackingConn struct {
	*fakeConn
	local  net.Addr
	remote net.Addr
}

func (f fakeTrackingConn) LocalAddr() net.Addr {
	return f.local
}

func (f fakeTrackingConn) RemoteAddr() net.Addr {
	return f.remote
}

func newMockRWC() mockReadWriterCloser {
	return mockReadWriterCloser{
		readChan:  make(chan Message, 10),
		writeChan: make(chan Message, 10),
	}
}

type mockReadWriterCloser struct {
	closeErr   error
	readError  error
	writeError error
	readChan   chan Message
	writeChan  chan Message
	once       sync.Once
}

func (r *mockReadWriterCloser) ReadMessage() (Message, error) {
	if msg, ok := <-r.readChan; ok {
		return msg, nil
	}
	return nil, fmt.Errorf("read failed: %w", r.readError)
}

func (r *mockReadWriterCloser) WriteMessage(m Message) error {
	if r.writeError != nil {
		return fmt.Errorf("write failed: %w", r.writeError)
	}
	if r.writeChan != nil {
		r.writeChan <- m
		return nil
	}
	panic("invariant violation")
}

func (r *mockReadWriterCloser) Close() error {
	r.once.Do(func() {
		close(r.readChan)
		close(r.writeChan)
	})
	return r.closeErr
}

type mockMessage string

func (m mockMessage) Encode() ([]byte, error) {
	return []byte(string(m)), nil
}

func TestConnProxy(t *testing.T) {
	// distinguished errors to use for validation
	clientReadErr := errors.New("failed client")
	serverReadErr := errors.New("failed server")
	clientCloseError := errors.New("client close error")
	serverCloseError := errors.New("server close error")
	writeError := errors.New("something went wrong")

	t.Run("error-handling", func(t *testing.T) {
		tests := []struct {
			name     string
			setupFn  func(t *testing.T, client, server *mockReadWriterCloser)
			expectFn func(t *testing.T, proxyError error)
		}{
			{
				name: "bidirectional-copy-no-errors",
				setupFn: func(t *testing.T, clientConn, serverConn *mockReadWriterCloser) {
					// Message copied from client to server
					clientConn.readChan <- mockMessage("hello server!")
					msg := <-serverConn.writeChan
					assert.Equal(t, "hello server!", string(msg.(mockMessage)))

					// Message copied from server to client
					serverConn.readChan <- mockMessage("hello client!")
					msg = <-clientConn.writeChan
					assert.Equal(t, "hello client!", string(msg.(mockMessage)))

					// Both sides return EOF, no errors will be reported
					serverConn.readError = io.EOF
					clientConn.readError = io.EOF
					_ = serverConn.Close()
				},
				expectFn: func(t *testing.T, proxyError error) {
					require.NoError(t, proxyError)
				},
			},
			{
				name: "server-write-error",
				setupFn: func(t *testing.T, clientConn, serverConn *mockReadWriterCloser) {
					serverConn.writeError = writeError
					serverConn.readError = io.EOF
					// Copy from client to server will fail
					clientConn.readChan <- mockMessage("hello server!")
					// Write error should be returned
					serverConn.Close()
				},
				expectFn: func(t *testing.T, proxyError error) {
					assert.ErrorIs(t, proxyError, writeError)
				},
			},
			{
				// Same as server-write-error, but swapped
				name: "client-write-error",
				setupFn: func(t *testing.T, clientConn, serverConn *mockReadWriterCloser) {
					clientConn.writeError = writeError
					clientConn.readError = io.EOF
					// Copy from server to client will fail
					serverConn.readChan <- mockMessage("hello client!")
					// Write error should be returned
					clientConn.Close()
				},
				expectFn: func(t *testing.T, proxyError error) {
					assert.ErrorIs(t, proxyError, writeError)
				},
			},
			{
				// Same as server and client read both fail
				name: "server-and-client-read-error",
				setupFn: func(t *testing.T, clientConn, serverConn *mockReadWriterCloser) {
					clientConn.readError = clientReadErr
					serverConn.readError = serverReadErr
					clientConn.Close()
				},
				expectFn: func(t *testing.T, proxyError error) {
					// Both errors should be found in the error chain
					assert.ErrorIs(t, proxyError, clientReadErr)
					assert.ErrorIs(t, proxyError, serverReadErr)
				},
			},
			{
				// Same as server and client read both fail
				name: "close-errors-returned",
				setupFn: func(t *testing.T, clientConn, serverConn *mockReadWriterCloser) {
					// Both sides return EOF from read, but return errors on clos
					serverConn.closeErr = serverCloseError
					clientConn.closeErr = clientCloseError
					serverConn.readError = io.EOF
					clientConn.readError = io.EOF
					_ = serverConn.Close()
				},
				expectFn: func(t *testing.T, proxyError error) {
					// Both errors should be found in the error chain
					assert.ErrorIs(t, proxyError, clientCloseError)
					assert.ErrorIs(t, proxyError, serverCloseError)
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				clientConn := newMockRWC()
				serverConn := newMockRWC()
				proxy := NewConnProxy(&clientConn, &serverConn)
				proxyError := make(chan error)
				go func() {
					proxyError <- proxy.Run()
				}()

				test.setupFn(t, &clientConn, &serverConn)
				test.expectFn(t, <-proxyError)
			})
		}
	})
}

type mockConn struct {
	c   net.Conn
	enc gob.Encoder
	dec gob.Decoder
}

func newMockConn(c net.Conn) *mockConn {
	gob.NewEncoder(c)
	return &mockConn{
		c:   c,
		dec: *gob.NewDecoder(c),
		enc: *gob.NewEncoder(c),
	}
}

func (m *mockConn) ReadMessage() (Message, error) {
	var msg mockMessage
	return msg, m.dec.Decode(&msg)
}

func (m *mockConn) WriteMessage(msg Message) error {
	return m.enc.Encode(msg)
}

func (m *mockConn) Close() error {
	return m.c.Close()
}

func newBufferedConn() (net.Conn, net.Conn, error) {
	l := bufconn.Listen(1024)
	defer l.Close()

	type result struct {
		c   net.Conn
		err error
	}
	acceptResult := make(chan result)
	go func() {
		conn, err := l.Accept()
		acceptResult <- result{conn, err}
	}()
	b, dialError := l.Dial()
	res := <-acceptResult
	return res.c, b, errors.Join(res.err, dialError)
}

func TestInterceptor(t *testing.T) {
	// Example interceptor
	fooToBar := func(message Message) ([]Message, error) {
		switch string(message.(mockMessage)) {
		case "foo":
			return []Message{mockMessage("bar")}, nil
		case "many":
			return []Message{mockMessage("first"), mockMessage("last")}, nil
		case "resilience":
			return []Message{nil}, nil
		case "omit":
			return nil, nil
		}
		return []Message{message}, nil
	}

	readFooToBar := func(m *mockConn) *ReadWriteInterceptor {
		return NewReadWriteInterceptor(m, fooToBar, nil)
	}

	writeFooToBar := func(m *mockConn) *ReadWriteInterceptor {
		return NewReadWriteInterceptor(m, nil, fooToBar)
	}

	// Test both read interceptor and write interceptor functionality
	for _, wrapper := range []func(*mockConn) *ReadWriteInterceptor{
		readFooToBar,
		writeFooToBar,
	} {

		aInternal, aExternal, err := newBufferedConn()
		require.NoError(t, err)
		bInternal, bExternal, err := newBufferedConn()
		require.NoError(t, err)

		mockClientExternal := newMockConn(aExternal)
		mockClient := wrapper(newMockConn(aInternal))
		mockServer := wrapper(newMockConn(bInternal))
		mockServerExternal := newMockConn(bExternal)
		proxy := NewConnProxy(mockClient, mockServer)

		var proxyError error
		done := make(chan struct{})
		go func() {
			defer close(done)
			proxyError = proxy.Run()
		}()

		// Exercise both sides of the connection
		for _, scenario := range []struct {
			a *mockConn
			b *mockConn
		}{
			{mockClientExternal, mockServerExternal},
			{mockServerExternal, mockClientExternal},
		} {
			// Unintercepted message
			require.NoError(t, scenario.a.WriteMessage(mockMessage("noreplace")))
			msg, err := scenario.b.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, "noreplace", string(msg.(mockMessage)))

			// One message repalced with multiple
			require.NoError(t, scenario.a.WriteMessage(mockMessage("many")))
			first, err := scenario.b.ReadMessage()
			require.NoError(t, err)
			last, err := scenario.b.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, "first", string(first.(mockMessage)))
			assert.Equal(t, "last", string(last.(mockMessage)))

			// Misbehaving interceptor returns slice of nil messages
			require.NoError(t, scenario.a.WriteMessage(mockMessage("resilience")))
			require.NoError(t, scenario.a.WriteMessage(mockMessage("noreplace")))
			msg, err = scenario.b.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, "noreplace", string(msg.(mockMessage)))

			// Message swallowed by interceptor
			require.NoError(t, scenario.a.WriteMessage(mockMessage("omit")))
			require.NoError(t, scenario.a.WriteMessage(mockMessage("noreplace")))
			msg, err = scenario.b.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, "noreplace", string(msg.(mockMessage)))
		}
		mockClient.Close()
		<-done
		require.ErrorIs(t, proxyError, io.ErrClosedPipe)
	}
}

func TestRemoveNilMessages(t *testing.T) {
	tests := []struct {
		msgs        []Message
		expectedLen int
	}{
		{msgs: []Message{nil}, expectedLen: 0},
		{msgs: []Message{nil, nil}, expectedLen: 0},
		{msgs: []Message{MFA{}, nil}, expectedLen: 1},
		{msgs: []Message{nil, MFA{}}, expectedLen: 1},
		{msgs: []Message{nil, MFA{}, nil}, expectedLen: 1},
		{msgs: []Message{MFA{}, nil, MFA{}}, expectedLen: 2},
		{msgs: []Message{MFA{}, nil, nil, MFA{}}, expectedLen: 2},
		{msgs: []Message{MFA{}, nil, nil, MFA{}, nil, nil, MFA{}}, expectedLen: 3},
		{msgs: []Message{MFA{}, MFA{}, MFA{}}, expectedLen: 3},
	}

	for _, test := range tests {
		result := removeNilMessages(test.msgs)
		require.Len(t, result, test.expectedLen)
		for _, msg := range result {
			assert.NotNil(t, msg)
		}
	}
}
