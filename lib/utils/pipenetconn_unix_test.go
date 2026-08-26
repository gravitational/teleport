//go:build unix

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

package utils

import (
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestDualPipeNetConnCloseOnExec(t *testing.T) {
	c1, c2, err := DualPipeNetConn(nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c1.Close()) })
	t.Cleanup(func() { require.NoError(t, c2.Close()) })

	for _, c := range []net.Conn{c1, c2} {
		c = unwrapNetConn(c)

		require.Implements(t, (*syscall.Conn)(nil), c)

		rawConn, err := c.(syscall.Conn).SyscallConn()
		require.NoError(t, err)

		require.NoError(t, rawConn.Control(func(fd uintptr) {
			flags, err := unix.FcntlInt(fd, unix.F_GETFD, 0)
			require.NoError(t, err)

			isCloseOnExec := flags&unix.FD_CLOEXEC == unix.FD_CLOEXEC
			require.True(t, isCloseOnExec, "expected file descriptor to have close-on-exec flag (FD_CLOEXEC, %x), got flags %x", unix.FD_CLOEXEC, flags)
		}))

	}
}

func unwrapNetConn(c net.Conn) net.Conn {
	for {
		unwrapper, ok := c.(interface{ NetConn() net.Conn })
		if !ok {
			return c
		}
		unwrapped := unwrapper.NetConn()
		if unwrapped == nil {
			return c
		}
		c = unwrapped
	}
}
