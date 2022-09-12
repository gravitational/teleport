/*
Copyright 2022 Gravitational, Inc.

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

package integration

import (
	"os"
	"os/user"
	"runtime"
	"syscall"
	"testing"

	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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
			require.True(t, !isRoot())

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
