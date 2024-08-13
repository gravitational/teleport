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
package regular

import (
	"context"
	"net"
	"os"
	"os/user"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestValidateListenerSocket(t *testing.T) {
	t.Parallel()

	newSocketFiles := func(t *testing.T) (*net.UnixConn, *os.File) {
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

	tests := []struct {
		name        string
		mutateFiles func(*testing.T, *net.UnixConn, *os.File) (*net.UnixConn, *os.File)
		mutateConn  func(*testing.T, *os.File)
		assert      require.ErrorAssertionFunc
	}{
		{
			name:   "ok",
			assert: require.NoError,
		},
		{
			name: "socket type not STREAM",
			mutateFiles: func(t *testing.T, conn *net.UnixConn, file *os.File) (*net.UnixConn, *os.File) {
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
			mutateConn: func(t *testing.T, file *os.File) {
				fd := file.Fd()
				err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				require.NoError(t, err)
			},
			assert: require.Error,
		},
		{
			name: "listener socket is not listening",
			mutateFiles: func(t *testing.T, conn *net.UnixConn, file *os.File) (*net.UnixConn, *os.File) {
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
			conn, listenerFD := newSocketFiles(t)
			if tc.mutateFiles != nil {
				conn, listenerFD = tc.mutateFiles(t, conn, listenerFD)
			}
			if tc.mutateConn != nil {
				tc.mutateConn(t, listenerFD)
			}
			err := validateListenerSocket(&srv.ServerContext{}, conn, listenerFD)
			tc.assert(t, err)
		})
	}
}

// BenchmarkRootExecCommand measures performance of running multiple exec requests
// over a single ssh connection. The same test is run with and without host user
// creation support to catch any performance degradation caused by user provisioning.
func BenchmarkRootExecCommand(b *testing.B) {
	utils.RequireRoot(b)

	b.ReportAllocs()

	cases := []struct {
		name       string
		createUser bool
	}{
		{
			name: "no user creation",
		},
		{
			name:       "with user creation",
			createUser: true,
		},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			var opts []ServerOption
			if test.createUser {
				opts = []ServerOption{SetCreateHostUser(true)}
			}

			f := newFixtureWithoutDiskBasedLogging(b, opts...)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				username := f.user
				if test.createUser {
					username = utils.GenerateLocalUsername(b)
					b.Cleanup(func() { _, _ = host.UserDel(username) })
				}

				_, err := newUpack(f.testSrv, username, []string{username, f.user}, wildcardAllow)
				require.NoError(b, err)

				clt := f.newSSHClient(context.Background(), b, &user.User{Username: username})

				executeCommand(b, clt, "uptime", 10)
			}
		})
	}
}

func executeCommand(tb testing.TB, clt *tracessh.Client, command string, executions int) {
	tb.Helper()

	var wg sync.WaitGroup
	for i := 0; i < executions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx := context.Background()

			se, err := clt.NewSession(ctx)
			assert.NoError(tb, err)
			defer se.Close()

			assert.NoError(tb, se.Run(ctx, command))
		}()
	}

	wg.Wait()
}
