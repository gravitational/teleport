// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resumption

import (
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/sshutils"
)

// PreDetectFixedSSHVersion returns a [multiplexer.PreDetectFunc] that sends a
// fixed SSH version identifier at connection open and wraps the connection with
// a [sshutils.SSHServerVersionOverrider] with the same version identifier.
// versionPrefix should not include the trailing CRLF.
func PreDetectFixedSSHVersion(versionPrefix string) multiplexer.PreDetectFunc {
	serverVersionCRLF := versionPrefix + "\r\n"

	return func(c net.Conn) (multiplexer.PostDetectFunc, error) {
		if _, err := c.Write([]byte(serverVersionCRLF)); err != nil {
			return nil, trace.Wrap(err)
		}

		return func(c *multiplexer.Conn) net.Conn {
			return &sshVersionSkipConn{
				Conn: c,

				serverVersion:  serverVersionCRLF[:len(serverVersionCRLF)-2],
				alreadyWritten: serverVersionCRLF,
			}
		}, nil
	}
}

// sshVersionSkipConn is used to pass the intended server version to the SSH
// server code that will handle the connection because the identification string
// was already sent over the wire.
type sshVersionSkipConn struct {
	net.Conn
	serverVersion string

	// alreadyWritten is expected to be written by the application (since it was
	// already written on the wire). Shrinks as the application writes.
	alreadyWritten string
}

var _ sshutils.SSHServerVersionOverrider = (*sshVersionSkipConn)(nil)

// SSHServerVersionOverride implements [sshutils.SSHServerVersionOverrider].
func (c *sshVersionSkipConn) SSHServerVersionOverride() string {
	return c.serverVersion
}

// NetConn returns the underlying [net.Conn].
func (c *sshVersionSkipConn) NetConn() net.Conn {
	return c.Conn
}

// Write implements [io.Writer] and [net.Conn]. Technically non-compliant since
// net.Conn should be usable concurrently, but this will almost always wrap a
// [*multiplexer.Conn] which is already non-compliant in the same way on reads.
func (c *sshVersionSkipConn) Write(p []byte) (int, error) {
	if c.alreadyWritten == "" {
		// fast path, no error wrapping
		return c.Conn.Write(p)
	}

	s := min(len(p), len(c.alreadyWritten))
	if string(p[:s]) != c.alreadyWritten[:s] {
		return 0, trace.BadParameter("new application data doesn't match already written data (this is a bug)")
	}

	// we should do the write even if it's zero-length to check that the
	// connection is still open and that we're not past the write deadline
	n, err := c.Conn.Write(p[s:])
	if n > 0 || err == nil {
		n += s
		c.alreadyWritten = c.alreadyWritten[s:]
		// this looks silly, but a zero-length slice of a string holds a
		// reference to the underlying memory, and overwriting it with an empty
		// string constant fixes that
		if c.alreadyWritten == "" {
			c.alreadyWritten = ""
		}
	}

	return n, trace.Wrap(err)
}
