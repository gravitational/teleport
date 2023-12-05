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

package integration

import (
	"os"
	"os/user"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/lib/teleagent"
)

func TestAgentSocketPermissions(t *testing.T) {
	if !isRoot() {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}

	agentServer := teleagent.NewServer(nil)

	agentServer.SetTestPermissions(func() {
		// ListenUnixSocket should not have its uid changed from root
		require.True(t, isRoot())

		done := make(chan struct{})

		// Start goroutine to attempt privilege escalation during
		// permission updates on the unix socket.
		//
		// For each step of permission updating, it should be impossible
		// for the user to unlink/remove the socket. If they can unlink
		// or remove the socket, then it could be replaced with a symlink
		// which can be used to acquire the permissions of the original socket.
		go func() {
			defer close(done)

			// Update uid to nonroot
			_, _, serr := syscall.Syscall(syscall.SYS_SETUID, 1000, 0, 0)
			require.Zero(t, serr)
			require.False(t, isRoot())

			err := unix.Unlink(agentServer.Path)
			require.Error(t, err)
			err = os.Remove(agentServer.Path)
			require.Error(t, err)
			err = os.Rename(agentServer.Path, agentServer.Path)
			require.Error(t, err)
		}()
		<-done

		// ListenUnixSocket should not have its uid changed from root
		require.True(t, isRoot())
	})

	nonRoot, err := user.LookupId("1000")
	require.NoError(t, err)

	// lock goroutine to root so that ListenUnixSocket doesn't
	// pick up the syscall in the testPermissions func
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err = agentServer.ListenUnixSocket("test", "sock.agent", nonRoot)
	require.NoError(t, err)
}
