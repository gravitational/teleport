//go:build unix

// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

		sysConn, ok := c.(syscall.Conn)
		require.True(t, ok)

		rawConn, err := sysConn.SyscallConn()
		require.NoError(t, err)

		require.NoError(t, rawConn.Control(func(fd uintptr) {
			flags, err := unix.FcntlInt(fd, unix.F_GETFD, 0)
			require.NoError(t, err)

			isCloseOnExec := flags&unix.FD_CLOEXEC == unix.FD_CLOEXEC
			require.True(t, isCloseOnExec)
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
