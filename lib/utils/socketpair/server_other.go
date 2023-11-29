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

package socketpair

import (
	"syscall"

	"github.com/gravitational/trace"
)

// ListenerFromFD, when used with [DialerFromFD], emulates the
// behavior of a socket server over a socket pair. The dialer side creates a new socket
// pair for every dial, sending one half to the listener listener side. This provides
// a means of easily establishing multiple connections between parent/child processes
// without needing to manage the various security and lifecycle concerns associated with
// file or network sockets.
func ListenerFromFD(fd *os.File) (net.Listener, error) {
	fd.Close()
	return nil, trace.Wrap(nonUnixErr)
}

// DialerFromFD, when used with [ListenerFromFD], emulates the  behavior of a traditional
// socket server over a socket pair.  See [ListenerFromFD] for details.
func DialerFromFD(fd *os.File) (*Dailer, error) {
	fd.Close()
	return nil, trace.Wrap(nonUnixErr)
}

// Dial dials the associated listener by creating a new socketpair and passing
// one half across the main underlying socketpair.
func (d *Dialer) Dial() (net.Conn, error) {
	return nil, trace.Wrap(nonUnixErr)
}
