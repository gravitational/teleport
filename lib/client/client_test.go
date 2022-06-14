/*
Copyright 2016 Gravitational, Inc.

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
	"context"
	"io"
	"net"
	"os"
	"strings"
	"time"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"
)

type ClientTestSuite struct {
	client *TeleportClient
}

var _ = check.Suite(&ClientTestSuite{})

func (s *ClientTestSuite) TestHelperFunctions(c *check.C) {
	c.Assert(nodeName("one"), check.Equals, "one")
	c.Assert(nodeName("one:22"), check.Equals, "one")
}

func (s *ClientTestSuite) SetUpSuite(c *check.C) {
	// create the client:
	config := &Config{
		KeysDir: c.MkDir(),
		Tracer:  tracing.NoopProvider().Tracer("test"),
	}
	err := config.ParseProxyHost("localhost")
	c.Assert(err, check.IsNil)
	client, err := NewClient(config)
	c.Assert(err, check.IsNil)
	c.Assert(client, check.NotNil)
	s.client = client
}

func (s *ClientTestSuite) TestNewSession(c *check.C) {
	nc := &NodeClient{
		Namespace: "blue",
		Tracer:    tracing.NoopProvider().Tracer("test"),
	}

	ctx := context.Background()
	// defaults:
	ses, err := newSession(ctx, nc, nil, nil, nil, nil, nil, true)
	c.Assert(err, check.IsNil)
	c.Assert(ses, check.NotNil)
	c.Assert(ses.NodeClient(), check.Equals, nc)
	c.Assert(ses.namespace, check.Equals, nc.Namespace)
	c.Assert(ses.env, check.NotNil)
	c.Assert(ses.terminal.Stderr(), check.Equals, os.Stderr)
	c.Assert(ses.terminal.Stdout(), check.Equals, os.Stdout)
	c.Assert(ses.terminal.Stdin(), check.Equals, os.Stdin)

	// pass environ map
	env := map[string]string{
		sshutils.SessionEnvVar: "session-id",
	}
	ses, err = newSession(ctx, nc, nil, env, nil, nil, nil, true)
	c.Assert(err, check.IsNil)
	c.Assert(ses, check.NotNil)
	c.Assert(ses.env, check.DeepEquals, env)
	// the session ID must be taken from tne environ map, if passed:
	c.Assert(string(ses.id), check.Equals, "session-id")
}

// TestProxyConnection verifies that client or server-side disconnect
// propagates all the way to the opposite side.
func (s *ClientTestSuite) TestProxyConnection(c *check.C) {
	// remoteSrv mocks a remote listener, accepting port-forwarded connections
	// over SSH.
	remoteConCh := make(chan net.Conn)
	remoteErrCh := make(chan error, 3)
	remoteSrv := newTestListener(c, func(con net.Conn) {
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
	localSrv := newTestListener(c, func(con net.Conn) {
		defer con.Close()

		proxyErrCh <- proxyConnection(context.Background(), con, remoteSrv.Addr().String(), new(net.Dialer))
	})
	defer localSrv.Close()

	// Dial localSrv. This should trigger proxyConnection and a dial to
	// remoteSrv.
	localCon, err := net.Dial("tcp", localSrv.Addr().String())
	c.Assert(err, check.IsNil)
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
	c.Log("simulate client-side disconnect")
	err = localCon.Close()
	c.Assert(err, check.IsNil)

	for i := 0; i < 3; i++ {
		select {
		case err := <-proxyErrCh:
			c.Assert(err, check.IsNil)
		case err := <-remoteErrCh:
			c.Assert(err, check.IsNil)
		case err := <-clientErrCh:
			c.Assert(err, check.IsNil)
		case <-time.After(5 * time.Second):
			c.Fatal("proxyConnection, client and server didn't disconnect within 5s after client connection was closed")
		}
	}

	// Dial localSrv again. This should trigger proxyConnection and a dial to
	// remoteSrv.
	localCon, err = net.Dial("tcp", localSrv.Addr().String())
	c.Assert(err, check.IsNil)
	go func(con net.Conn) {
		_, err := io.Copy(io.Discard, con)
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		}
		clientErrCh <- err
	}(localCon)

	// Simulate a server-side disconnect. All other parties (tsh proxy and
	// local client) should disconnect as well.
	c.Log("simulate server-side disconnect")
	remoteCon := <-remoteConCh
	err = remoteCon.Close()
	c.Assert(err, check.IsNil)

	for i := 0; i < 3; i++ {
		select {
		case err := <-proxyErrCh:
			c.Assert(err, check.IsNil)
		case err := <-remoteErrCh:
			c.Assert(err, check.IsNil)
		case err := <-clientErrCh:
			c.Assert(err, check.IsNil)
		case <-time.After(5 * time.Second):
			c.Fatal("proxyConnection, client and server didn't disconnect within 5s after remote connection was closed")
		}
	}
}

func (s *ClientTestSuite) TestListenAndForwardCancel(c *check.C) {
	client := &NodeClient{
		Client: &tracessh.Client{
			Client: &ssh.Client{
				Conn: &fakeSSHConn{},
			},
		},
		Tracer: tracing.NoopProvider().Tracer("test"),
	}

	// Create two anchors. An "accept" anchor that unblocks once the listener has
	// accepted a connection and a "unblock" anchor that unblocks when Accept
	// unblocks.
	acceptCh := make(chan struct{})
	unblockCh := make(chan struct{})

	// Create a new cancelable listener.
	ctx, cancel := context.WithCancel(context.Background())
	ln, err := newWrappedListener(acceptCh)
	c.Assert(err, check.IsNil)

	// Start listenAndForward and close the unblock channel once "Accept" has
	// unblocked.
	go func() {
		client.listenAndForward(ctx, ln, "")
		close(unblockCh)
	}()

	// Block until "Accept" has been called. After this it is safe to assume the
	// listener is accepting.
	select {
	case <-acceptCh:
	case <-time.After(1 * time.Minute):
		c.Fatal("Timed out waiting for Accept to be called.")
	}

	// At this point, "Accept" should still be blocking.
	select {
	case <-unblockCh:
		c.Fatalf("Failed because Accept was unblocked.")
	default:
	}

	// Cancel "Accept" to unblock it.
	cancel()

	// Verify that "Accept" has unblocked.
	select {
	case <-unblockCh:
	case <-time.After(1 * time.Minute):
		c.Fatal("Timed out waiting for Accept to unblock.")
	}
}

func newTestListener(c *check.C, handle func(net.Conn)) net.Listener {
	l, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, check.IsNil)

	go func() {
		for {
			con, err := l.Accept()
			if err != nil {
				c.Logf("listener error: %v", err)
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
