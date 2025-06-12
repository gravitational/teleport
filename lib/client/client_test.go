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

package client

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestHelperFunctions(t *testing.T) {
	assert.Equal(t, "one", nodeName(TargetNode{Addr: "one"}))
	assert.Equal(t, "one", nodeName(TargetNode{Addr: "one:22"}))
	assert.Equal(t, "example.com", nodeName(TargetNode{Addr: "one", Hostname: "example.com"}))
}

func TestNewSession(t *testing.T) {
	nc := &NodeClient{
		Tracer: tracing.NoopProvider().Tracer("test"),
	}

	ctx := context.Background()
	// defaults:
	ses, err := newSession(ctx, nc, nil, nil, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, ses)
	require.Equal(t, nc, ses.NodeClient())
	require.NotNil(t, ses.env)
	require.Equal(t, os.Stderr, ses.terminal.Stderr())
	require.Equal(t, os.Stdout, ses.terminal.Stdout())
	require.Equal(t, os.Stdin, ses.terminal.Stdin())

	// pass environ map
	env := map[string]string{
		sshutils.SessionEnvVar: "session-id",
	}
	ses, err = newSession(ctx, nc, nil, env, nil, nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, ses)
	// the session ID must be unset from tne environ map, if we are not joining a session:
	require.Empty(t, ses.id)
}

// TestProxyConnection verifies that client or server-side disconnect
// propagates all the way to the opposite side.
func TestProxyConnection(t *testing.T) {
	// remoteSrv mocks a remote listener, accepting port-forwarded connections
	// over SSH.
	remoteConCh := make(chan net.Conn)
	remoteErrCh := make(chan error, 3)
	remoteSrv := newTestListener(t, func(con net.Conn) {
		defer con.Close()

		remoteConCh <- con

		// Echo any data back to the sender.
		_, err := io.Copy(con, con)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		}
		remoteErrCh <- err
	})
	defer remoteSrv.Close()

	// localSrv mocks a local tsh listener, accepting local connections for
	// port-forwarding to remote SSH node.
	proxyErrCh := make(chan error, 3)
	localSrv := newTestListener(t, func(con net.Conn) {
		defer con.Close()

		proxyErrCh <- proxyConnection(context.Background(), con, remoteSrv.Addr().String(), new(net.Dialer))
	})
	defer localSrv.Close()

	// Dial localSrv. This should trigger proxyConnection and a dial to
	// remoteSrv.
	localCon, err := net.Dial("tcp", localSrv.Addr().String())
	require.NoError(t, err)
	clientErrCh := make(chan error, 3)
	go func(con net.Conn) {
		_, err := io.Copy(io.Discard, con)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		}
		clientErrCh <- err
	}(localCon)

	// Discard remoteCon to unblock the remote handler.
	<-remoteConCh

	// Simulate a client-side disconnect. All other parties (tsh proxy and
	// remove listener) should disconnect as well.
	t.Log("simulate client-side disconnect")
	err = localCon.Close()
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		select {
		case err := <-proxyErrCh:
			require.NoError(t, err)
		case err := <-remoteErrCh:
			require.NoError(t, err)
		case err := <-clientErrCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("proxyConnection, client and server didn't disconnect within 5s after client connection was closed")
		}
	}

	// Dial localSrv again. This should trigger proxyConnection and a dial to
	// remoteSrv.
	localCon, err = net.Dial("tcp", localSrv.Addr().String())
	require.NoError(t, err)
	go func(con net.Conn) {
		_, err := io.Copy(io.Discard, con)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		}
		clientErrCh <- err
	}(localCon)

	// Simulate a server-side disconnect. All other parties (tsh proxy and
	// local client) should disconnect as well.
	t.Log("simulate server-side disconnect")
	remoteCon := <-remoteConCh
	err = remoteCon.Close()
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		select {
		case err := <-proxyErrCh:
			require.NoError(t, err)
		case err := <-remoteErrCh:
			require.NoError(t, err)
		case err := <-clientErrCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("proxyConnection, client and server didn't disconnect within 5s after remote connection was closed")
		}
	}
}

func TestListenAndForwardCancel(t *testing.T) {
	tests := []struct {
		name    string
		testFun func(client *NodeClient, ctx context.Context, listener *wrappedListener)
	}{
		{
			name: "listenAndForward",
			testFun: func(client *NodeClient, ctx context.Context, listener *wrappedListener) {
				client.listenAndForward(ctx, listener, "localAddr", "remoteAddr")
			},
		},
		{
			name: "dynamicListenAndForward",
			testFun: func(client *NodeClient, ctx context.Context, listener *wrappedListener) {
				client.dynamicListenAndForward(ctx, listener, "localAddr")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &NodeClient{
				Client: &tracessh.Client{
					Client: &ssh.Client{
						Conn: &fakeSSHConn{},
					},
				},
				Tracer: tracing.NoopProvider().Tracer("test"),
			}

			// Create two anchors. An "accept" anchor that unblocks once the listener has
			// accepted a connection and an "unblock" anchor that unblocks when Accept
			// unblocks.
			acceptCh := make(chan struct{})
			unblockCh := make(chan struct{})

			// Create a new cancelable listener.
			ctx, cancel := context.WithCancel(context.Background())
			ln, err := newWrappedListener(acceptCh)
			require.NoError(t, err)

			// Start testFun (listenAndForward or dynamicListenAndForward)
			// and close the unblock channel once "Accept" has unblocked.
			go func() {
				tt.testFun(client, ctx, ln)
				close(unblockCh)
			}()

			// Block until "Accept" has been called. After this it is safe to assume the
			// listener is accepting.
			select {
			case <-acceptCh:
			case <-time.After(1 * time.Minute):
				t.Fatal("Timed out waiting for Accept to be called.")
			}

			// At this point, "Accept" should still be blocking.
			select {
			case <-unblockCh:
				t.Fatalf("Failed because Accept was unblocked.")
			default:
			}

			// Cancel "Accept" to unblock it.
			cancel()

			// Verify that "Accept" has unblocked.
			select {
			case <-unblockCh:
			case <-time.After(1 * time.Minute):
				t.Fatal("Timed out waiting for Accept to unblock.")
			}
		})
	}
}

func newTestListener(t *testing.T, handle func(net.Conn)) net.Listener {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	go func() {
		for {
			con, err := l.Accept()
			if err != nil {
				return
			}
			go handle(con)
		}
	}()

	return l
}

// fakeSSHConn is a NOP connection that implements the ssh.Conn interface.
// Only used in tests.
type fakeSSHConn struct {
	ssh.Conn
}

func (c *fakeSSHConn) Close() error {
	return nil
}

// wrappedListener is a listener that uses a channel to notify the caller
// when "Accept" has been called.
type wrappedListener struct {
	net.Listener
	acceptCh chan struct{}
}

func newWrappedListener(acceptCh chan struct{}) (*wrappedListener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &wrappedListener{
		Listener: ln,
		acceptCh: acceptCh,
	}, nil
}

func (l wrappedListener) Accept() (net.Conn, error) {
	close(l.acceptCh)
	return l.Listener.Accept()
}

func TestLineLabeledWriter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		inputs     []string
		lineLength int
		expected   string
	}{
		{
			name:     "typical input",
			inputs:   []string{"this is\nsome test\ninput"},
			expected: "[label] this is\n[label] some test\n[label] input\n",
		},
		{
			name:     "don't add empty line at end",
			inputs:   []string{"dangling newline\n"},
			expected: "[label] dangling newline\n",
		},
		{
			name:     "blank lines in middle",
			inputs:   []string{"this\n\nis\n\nsome input"},
			expected: "[label] this\n[label] \n[label] is\n[label] \n[label] some input\n",
		},
		{
			name:     "line break between writes",
			inputs:   []string{"line 1\n", "line 2\n"},
			expected: "[label] line 1\n[label] line 2\n",
		},
		{
			name:     "line break immediately on second write",
			inputs:   []string{"line 1", "\nline 2"},
			expected: "[label] line 1\n[label] line 2\n",
		},
		{
			name:     "line continues between writes",
			inputs:   []string{"this is all ", "one continuous line ", "until\nnow"},
			expected: "[label] this is all one continuous line until\n[label] now\n",
		},
		{
			name:       "long lines wrapped",
			inputs:     []string{"1234\nabcdefghijklmnopqrstuvwxyz\n1234"},
			lineLength: 16,
			expected:   "[label] 1234\n[label] abcdefgh\n[label] ijklmnop\n[label] qrstuvwx\n[label] yz\n[label] 1234\n",
		},
		{
			name:       "exact length lines",
			inputs:     []string{"abcdefgh", "\nijklmnop"},
			lineLength: 16,
			expected:   "[label] abcdefgh\n[label] ijklmnop\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w, err := newLineLabeledWriter(&buf, "label", tc.lineLength)
			require.NoError(t, err)

			totalBytes := 0
			expectedBytes := 0
			for _, line := range tc.inputs {
				n, err := w.Write([]byte(line))
				assert.NoError(t, err)
				totalBytes += n
				expectedBytes += len(line)
			}
			assert.NoError(t, w.Close())
			assert.Equal(t, expectedBytes, totalBytes)
			assert.Equal(t, tc.expected, buf.String())
		})
	}
}

func TestRouteToDatabaseToProto(t *testing.T) {
	input := tlsca.RouteToDatabase{
		ServiceName: "db-service",
		Database:    "db-name",
		Username:    "db-user",
		Protocol:    "db-protocol",
		Roles:       []string{"db-role1", "db-role2"},
	}
	expected := proto.RouteToDatabase{
		ServiceName: "db-service",
		Database:    "db-name",
		Username:    "db-user",
		Protocol:    "db-protocol",
		Roles:       []string{"db-role1", "db-role2"},
	}
	require.Equal(t, expected, RouteToDatabaseToProto(input))
}
