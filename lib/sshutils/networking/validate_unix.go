//go:build unix
// +build unix

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package networking

import (
	"errors"
	"os"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// validateListenerSocket checks that the socket and listener file descriptor
// sent from the networking process have the expected properties.
func validateListenerSocket(listenerFD *os.File) error {
	if err := controlSyscallConn(listenerFD, func(fd uintptr) error {
		// Verify the socket type
		if sockType, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_TYPE); err != nil {
			return trace.Wrap(err)
		} else if sockType != unix.SOCK_STREAM {
			return trace.AccessDenied("socket is not of the expected type (STREAM)")
		}

		// Verify that reuse is not enabled on the socket
		if reuseAddr, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR); err != nil {
			return trace.Wrap(err)
		} else if reuseAddr != 0 {
			return trace.AccessDenied("SO_REUSEADDR is enabled on the socket")
		}
		if reusePort, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT); err != nil {
			// Some systems may not support SO_REUSEPORT.
			if !errors.Is(err, unix.ENOPROTOOPT) {
				return trace.Wrap(err)
			}
		} else if reusePort != 0 {
			return trace.AccessDenied("SO_REUSEPORT is enabled on the socket")
		}

		// Verify that the listener is already listening.
		if acceptConn, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_ACCEPTCONN); err != nil {
			// Some systems may not support SO_ACCEPTCONN.
			if !errors.Is(err, unix.ENOPROTOOPT) {
				return trace.Wrap(err)
			}
		} else if acceptConn == 0 {
			return trace.AccessDenied("SO_ACCEPTCONN is not set, socket is not listening")
		}

		return nil
	}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func controlSyscallConn(conn syscall.Conn, f func(fd uintptr) error) error {
	syscallConn, err := conn.SyscallConn()
	if err != nil {
		return trace.Wrap(err)
	}
	if cErr := syscallConn.Control(func(fd uintptr) {
		err = f(fd)
	}); cErr != nil {
		return trace.Wrap(cErr)
	}
	return trace.Wrap(err)
}
