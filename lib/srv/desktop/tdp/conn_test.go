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
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newMockRWC() rwc {
	return rwc{
		readChan:  make(chan Message, 10),
		writeChan: make(chan Message, 10),
	}
}

type rwc struct {
	closeErr   error
	readError  error
	writeError error
	readChan   chan Message
	writeChan  chan Message
	once       sync.Once
}

func (r *rwc) ReadMessage() (Message, error) {
	if msg, ok := <-r.readChan; ok {
		return msg, nil
	}
	return nil, fmt.Errorf("read failed: %w", r.readError)
}

func (r *rwc) WriteMessage(m Message) error {
	if r.writeError != nil {
		return fmt.Errorf("write failed: %w", r.writeError)
	}
	if r.writeChan != nil {
		r.writeChan <- m
		return nil
	}
	panic("invariant violation")
}

func (r *rwc) Close() error {
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
			setupFn  func(t *testing.T, client, server *rwc)
			expectFn func(t *testing.T, proxyError error)
		}{
			{
				name: "bidirectional-copy-no-errors",
				setupFn: func(t *testing.T, clientConn, serverConn *rwc) {
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
				setupFn: func(t *testing.T, clientConn, serverConn *rwc) {
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
				setupFn: func(t *testing.T, clientConn, serverConn *rwc) {
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
				setupFn: func(t *testing.T, clientConn, serverConn *rwc) {
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
				setupFn: func(t *testing.T, clientConn, serverConn *rwc) {
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

func TestInterceptor(t *testing.T) {
	testConn := newMockRWC()
	fooToBar := func(message Message) (Message, error) {
		switch string(message.(mockMessage)) {
		case "foo":
			return mockMessage("bar"), nil
		case "omit":
			return nil, nil
		}
		return message, nil
	}

	interceptedRWC := NewReadWriteInterceptor(&testConn, fooToBar, fooToBar)

	// Test write interceptor
	require.NoError(t, interceptedRWC.WriteMessage(mockMessage("noreplace")))
	msg := <-testConn.writeChan
	assert.Equal(t, "noreplace", string(msg.(mockMessage)))
	require.NoError(t, interceptedRWC.WriteMessage(mockMessage("foo")))
	msg = <-testConn.writeChan
	assert.Equal(t, "bar", string(msg.(mockMessage)))

	require.NoError(t, interceptedRWC.WriteMessage(mockMessage("omit")))
	require.NoError(t, interceptedRWC.WriteMessage(mockMessage("noreplace")))
	msg = <-testConn.writeChan
	// "omit" message should be dropped, so the next message is "noreplace"
	assert.Equal(t, "noreplace", string(msg.(mockMessage)))

	// Test read interceptor
	testConn.readChan <- mockMessage("noreplace")
	msg, err := interceptedRWC.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "noreplace", string(msg.(mockMessage)))

	testConn.readChan <- mockMessage("foo")
	msg, err = interceptedRWC.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "bar", string(msg.(mockMessage)))

	testConn.readChan <- mockMessage("omit")
	testConn.readChan <- mockMessage("noreplace")
	// "omit" message should be dropped, so the next message is "noreplace"
	msg, err = interceptedRWC.ReadMessage()
	assert.Equal(t, "noreplace", string(msg.(mockMessage)))
}
