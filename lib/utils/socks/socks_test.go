/*
Copyright 2018 Gravitational, Inc.

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

package socks

import (
	"fmt"
	"io"
	"net"
	"testing"

	"golang.org/x/net/proxy"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type SOCKSSuite struct{}

var _ = check.Suite(&SOCKSSuite{})
var _ = fmt.Printf

func (s *SOCKSSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *SOCKSSuite) TearDownSuite(c *check.C) {}
func (s *SOCKSSuite) SetUpTest(c *check.C)     {}
func (s *SOCKSSuite) TearDownTest(c *check.C)  {}

func (s *SOCKSSuite) TestHandshake(c *check.C) {
	remoteAddr := "example.com:443"

	// Create and start a debug SOCKS5 server that calls socks.Handshake().
	socksServer, err := newDebugServer()
	c.Assert(err, check.IsNil)
	go socksServer.Serve()

	// Create a proxy dialer that can perform a SOCKS5 handshake.
	proxy, err := proxy.SOCKS5("tcp", socksServer.Addr().String(), nil, nil)
	c.Assert(err, check.IsNil)

	// Connect to the SOCKS5 server, this is where the handshake function is called.
	conn, err := proxy.Dial("tcp", remoteAddr)
	c.Assert(err, check.IsNil)

	// Read in what was written on the connection. With the debug server it's
	// always the addres requested.
	buf := make([]byte, len(remoteAddr))
	_, err = io.ReadFull(conn, buf)
	c.Assert(err, check.IsNil)
	c.Assert(string(buf), check.Equals, remoteAddr)

	// Close and cleanup.
	err = conn.Close()
	c.Assert(err, check.IsNil)
}

// debugServer is a debug SOCKS5 server that performs a SOCKS5 handshake
// then writes the remote address and closes the connection.
type debugServer struct {
	ln net.Listener
}

// newDebugServer creates a new debug server on a random port.
func newDebugServer() (*debugServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &debugServer{
		ln: ln,
	}, nil
}

// Addr returns the address the debug server is running on.
func (d *debugServer) Addr() net.Addr {
	return d.ln.Addr()
}

// Serve accepts and handles the connection.
func (d *debugServer) Serve() {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			log.Debugf("Failed to accept connection: %v.", err)
			break
		}

		go d.handle(conn)
	}
}

// handle performs the SOCKS5 handshake then writes the remote address to
// the net.Conn and closes it.
func (d *debugServer) handle(conn net.Conn) {
	defer conn.Close()

	remoteAddr, err := Handshake(conn)
	if err != nil {
		log.Debugf("Handshake failed: %v.", err)
		return
	}

	n, err := conn.Write([]byte(remoteAddr))
	if err != nil {
		log.Debugf("Failed to write to connection: %v.", err)
		return
	}
	if n != len(remoteAddr) {
		log.Debugf("Short write, wrote %v wanted %v.", n, len(remoteAddr))
		return
	}
}
