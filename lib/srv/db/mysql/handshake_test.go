/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mysql

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
)

func TestSendMagicProxyReply(t *testing.T) {
	conn := &bufferedTestConn{}
	err := sendMagicProxyReply(conn)
	require.NoError(t, err)
	got, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Equal(t, []byte("\x13ready-for-handshake"), got)
}

func TestWaitForMagicProxyReply(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		desc       string
		input      []byte
		modifyConn func(t *testing.T, conn *fakeServerConn)
		timeout    time.Duration
		wantFound  bool
		wantErr    error
	}{
		{
			desc: "handles EOF when looking for length header",
			modifyConn: func(t *testing.T, conn *fakeServerConn) {
				conn.clt.Close()
			},
		},
		{
			desc:  "length header does not match",
			input: []byte("\x12ready-for-handshake"),
		},
		{
			desc:  "handles EOF when looking for payload",
			input: []byte("\x13llama-lied"),
			modifyConn: func(t *testing.T, conn *fakeServerConn) {
				conn.clt.Close()
			},
		},
		{
			desc:  "payload doesnt match",
			input: []byte("\x13llama-was-disguised"),
		},
		{
			desc:      "found match",
			input:     []byte("\x13ready-for-handshake"),
			wantFound: true,
		},
		{
			desc:    "handles timeout gracefully",
			timeout: time.Millisecond * 100,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			conn := newFakeServerConn(t, test.input)
			if test.modifyConn != nil {
				test.modifyConn(t, conn)
			}
			bufConn := protocol.NewBufferedConn(ctx, conn)
			found, err := waitForMagicProxyReply(bufConn, test.timeout)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantFound, found)
			require.NoError(t, conn.clt.Close())
			remaining, err := io.ReadAll(bufConn)
			require.NoError(t, err)
			if found {
				require.Empty(t, remaining, "should discard the matched bytes")
			} else {
				want := test.input
				if want == nil {
					want = []byte{}
				}
				require.Equal(t, want, remaining, "should not advance the buffered conn reader if the proxy reply is not found")
			}
		})
	}
}

func TestHandshakeModes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc               string
		agentHandshakeMode handshakeMode
		wantAgentError     string
		proxyHandshakeMode handshakeMode
		wantProxyError     string
		wantHandshake      bool
	}{
		{
			agentHandshakeMode: handshakeDisabled,
			proxyHandshakeMode: handshakeDisabled,
		},
		{
			agentHandshakeMode: handshakeWhenSupported,
			proxyHandshakeMode: handshakeDisabled,
		},
		{
			agentHandshakeMode: handshakeDisabled,
			proxyHandshakeMode: handshakeWhenSupported,
		},
		{
			agentHandshakeMode: handshakeWhenSupported,
			proxyHandshakeMode: handshakeWhenSupported,
			wantHandshake:      true,
		},
		{
			agentHandshakeMode: handshakeAlways,
			proxyHandshakeMode: handshakeWhenSupported,
			wantHandshake:      true,
		},
		{
			agentHandshakeMode: handshakeAlways,
			proxyHandshakeMode: handshakeAlways,
			wantHandshake:      true,
		},
		// always and disabled are not compatible
		{
			agentHandshakeMode: handshakeAlways,
			wantAgentError:     "EOF",
			proxyHandshakeMode: handshakeDisabled,
			wantProxyError:     "expected OK or ERR packet",
		},
		{
			agentHandshakeMode: handshakeDisabled,
			proxyHandshakeMode: handshakeAlways,
			wantProxyError:     "invalid protocol",
		},
		// proxy has to be upgraded last, it cant be changed to always
		// handshake until all supported agents will always handshake
		{
			agentHandshakeMode: handshakeWhenSupported,
			proxyHandshakeMode: handshakeAlways,
			wantProxyError:     "invalid protocol",
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("agent handshake %v proxy handshake %v", test.agentHandshakeMode, test.proxyHandshakeMode), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			agentProxyConn, proxyAgentConn := net.Pipe()
			var wg sync.WaitGroup
			wg.Add(2)

			// agent
			var agentMySQLServer *server.Conn
			go func() {
				defer wg.Done()
				defer agentProxyConn.Close()
				agentMySQLServer = makeProxyConn(agentProxyConn)
				log := slog.With(teleport.ComponentKey, teleport.ComponentDatabase)
				err := notifyProxy(ctx, log, test.agentHandshakeMode, agentMySQLServer)
				if test.wantAgentError != "" {
					assert.ErrorContains(t, err, test.wantAgentError)
					return
				}
				if !assert.NoError(t, err) {
					return
				}
				if test.wantHandshake {
					assert.NotEmpty(t, agentMySQLServer.Attributes())
					assert.Equal(t, "llama", agentMySQLServer.Attributes()["species"])
				} else {
					assert.Empty(t, agentMySQLServer.Attributes())
				}
				assert.Zero(t, agentMySQLServer.Sequence)
				assertEmptyConn(t, agentMySQLServer.Conn)
			}()

			// proxy
			go func() {
				defer wg.Done()
				defer proxyAgentConn.Close()
				log := slog.With(teleport.ComponentKey, teleport.ComponentProxy)
				err := waitForAgent(ctx, log, test.proxyHandshakeMode, proxyAgentConn, func(c *client.Conn) error {
					c.SetAttributes(map[string]string{"species": "llama"})
					return nil
				})
				if test.wantProxyError != "" {
					assert.ErrorContains(t, err, test.wantProxyError)
					return
				}
				if !assert.NoError(t, err) {
					return
				}
				assertEmptyConn(t, proxyAgentConn)
			}()

			wg.Wait()
		})
	}
}

func assertEmptyConn(t *testing.T, conn net.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	defer conn.SetReadDeadline(time.Time{})
	buf := make([]byte, 1)
	n, _ := conn.Read(buf)
	assert.Zero(t, n)
}

type bufferedTestConn struct {
	net.Conn

	buf bytes.Buffer
}

func (c *bufferedTestConn) Read(b []byte) (n int, err error) {
	return c.buf.Read(b)
}

func (c *bufferedTestConn) Write(b []byte) (n int, err error) {
	return c.buf.Write(b)
}

func newFakeServerConn(t *testing.T, input []byte) *fakeServerConn {
	// we use a pipe just to closely resemble a real connection and simulate timeouts
	clt, srv := net.Pipe()
	t.Cleanup(func() {
		clt.Close()
		srv.Close()
	})

	return &fakeServerConn{
		Conn: srv,
		clt:  clt,
		buf:  bytes.NewBuffer(input),
	}
}

type fakeServerConn struct {
	net.Conn
	// reads beyond the buffered input may block until this end of the pipe is closed
	clt net.Conn

	buf     *bytes.Buffer
	readErr error
}

func (c *fakeServerConn) Read(b []byte) (n int, err error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	return io.MultiReader(c.buf, c.Conn).Read(b)
}

func (c *fakeServerConn) Write(b []byte) (n int, err error) {
	return c.buf.Write(b)
}
