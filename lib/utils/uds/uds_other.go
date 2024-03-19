//go:build !unix

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

package uds

import (
	"errors"
	"net"
	"os"

	"github.com/gravitational/trace"
)

var errNonUnix = errors.New("uds.Conn only supported on unix")

// assert that *Conn implements net.Conn.
var _ net.Conn = (*Conn)(nil)

// Conn extends net.UnixConn with additional useful methods.
type Conn struct {
	*net.UnixConn
}

// FromFile attempts to create a [Conn] from a file. The returned conn
// is independent from the file and closing one does not close the other.
func FromFile(fd *os.File) (*Conn, error) {
	return nil, trace.Wrap(errNonUnix)
}

// WriteWithFDs performs a write that may also send associated FDs. Note that unless the
// underlying socket is a datagram socket it is not guaranteed that the exact bytes written
// will match the bytes received with the fds on the reader side.
func (c *Conn) WriteWithFDs(b []byte, fds []*os.File) (n, fdn int, err error) {
	return 0, 0, trace.Wrap(errNonUnix)
}

// ReadWithFDs reads data and associated fds. Note that the underlying socket must be a datagram socket
// if you need the bytes read to exactly match the bytes sent with the FDs.
func (c *Conn) ReadWithFDs(b []byte, fds []*os.File) (n, fdn int, err error) {
	return 0, 0, trace.Wrap(errNonUnix)
}
