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
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/sshutils"

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
	}

	// defaults:
	ses, err := newSession(nc, nil, nil, nil, nil, nil, false, true)
	c.Assert(err, check.IsNil)
	c.Assert(ses, check.NotNil)
	c.Assert(ses.NodeClient(), check.Equals, nc)
	c.Assert(ses.namespace, check.Equals, nc.Namespace)
	c.Assert(ses.env, check.NotNil)
	c.Assert(ses.stderr, check.Equals, os.Stderr)
	c.Assert(ses.stdout, check.Equals, os.Stdout)
	c.Assert(ses.stdin, check.Equals, os.Stdin)

	// pass environ map
	env := map[string]string{
		sshutils.SessionEnvVar: "session-id",
	}
	ses, err = newSession(nc, nil, env, nil, nil, nil, false, true)
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
		_, err := io.Copy(ioutil.Discard, con)
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
		_, err := io.Copy(ioutil.Discard, con)
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
