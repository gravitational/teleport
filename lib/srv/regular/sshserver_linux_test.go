//go:build linux
// +build linux

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
package regular

import (
	"net"
	"os"
	"os/user"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestValidateListenerSocket(t *testing.T) {
	t.Parallel()

	newSocketFiles := func(t *testing.T) (*uds.Conn, *os.File) {
		left, right, err := uds.NewSocketpair(uds.SocketTypeStream)
		require.NoError(t, err)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		tcpListener := listener.(*net.TCPListener)
		listenerFD, err := tcpListener.File()
		require.NoError(t, err)

		conn, err := tcpListener.SyscallConn()
		require.NoError(t, err)
		err2 := conn.Control(func(descriptor uintptr) {
			// Disable address reuse to prevent socket replacement.
			err = syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
		})
		require.NoError(t, err2)
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, left.Close())
			require.NoError(t, right.Close())
		})
		return left, listenerFD
	}

	u, err := user.Current()
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        string
		mutateFiles func(*testing.T, *uds.Conn, *os.File) (*uds.Conn, *os.File)
		mutateConn  func(*testing.T, *os.File)
		assert      require.ErrorAssertionFunc
	}{
		{
			name:   "ok",
			user:   u.Username,
			assert: require.NoError,
		},
		{
			name:   "wrong user",
			user:   "fake-user",
			assert: require.Error,
		},
		{
			name: "socket type not STREAM",
			user: u.Username,
			mutateFiles: func(t *testing.T, conn *uds.Conn, file *os.File) (*uds.Conn, *os.File) {
				left, right, err := uds.NewSocketpair(uds.SocketTypeDatagram)
				require.NoError(t, err)
				listenerFD, err := right.File()
				require.NoError(t, err)
				require.NoError(t, right.Close())
				t.Cleanup(func() {
					require.NoError(t, left.Close())
					require.NoError(t, listenerFD.Close())
				})
				return left, listenerFD
			},
			assert: require.Error,
		},
		{
			name: "SO_REUSEADDR enabled",
			user: u.Username,
			mutateConn: func(t *testing.T, file *os.File) {
				fd := file.Fd()
				err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				require.NoError(t, err)
			},
			assert: require.Error,
		},
		{
			name: "listener socket is not listening",
			user: u.Username,
			mutateFiles: func(t *testing.T, conn *uds.Conn, file *os.File) (*uds.Conn, *os.File) {
				left, right, err := uds.NewSocketpair(uds.SocketTypeStream)
				require.NoError(t, err)
				listenerFD, err := right.File()
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, left.Close())
					require.NoError(t, listenerFD.Close())
				})
				return left, listenerFD
			},
			assert: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scx := &srv.ServerContext{
				Identity: srv.IdentityContext{
					Login: tc.user,
				},
			}
			conn, listenerFD := newSocketFiles(t)
			if tc.mutateFiles != nil {
				conn, listenerFD = tc.mutateFiles(t, conn, listenerFD)
			}
			if tc.mutateConn != nil {
				tc.mutateConn(t, listenerFD)
			}
			err := validateListenerSocket(scx, conn.UnixConn, listenerFD)
			tc.assert(t, err)
		})
	}
}
