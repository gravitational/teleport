/*
Copyright 2023 Gravitational, Inc.

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

package pingconn

import (
	"crypto/tls"

	"github.com/gravitational/trace"
)

// NewTLS returns a ping connection wrapping the provided tls.Conn.
func NewTLS(conn *tls.Conn) *PingTLSConn {
	return &PingTLSConn{
		Conn: conn,
		ping: New(conn),
	}
}

// PingTLSConn wraps a tls.Conn and adds ping capabilities to it.
type PingTLSConn struct {
	*tls.Conn

	ping *PingConn
}

// Read reads content from the underlying connection, discarding any ping
// messages it finds.
func (c *PingTLSConn) Read(p []byte) (int, error) {
	n, err := c.ping.Read(p)
	return n, trace.Wrap(err)
}

// WritePing writes the ping packet to the connection.
func (c *PingTLSConn) WritePing() error {
	return trace.Wrap(c.ping.WritePing())
}

// Write writes provided content to the underlying connection with proper
// protocol fields.
func (c *PingTLSConn) Write(p []byte) (int, error) {
	n, err := c.ping.Write(p)
	return n, trace.Wrap(err)
}
