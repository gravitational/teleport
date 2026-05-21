//go:build unix

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/gravitational/teleport/session/uds"
)

func TestWaitReady(t *testing.T) {
	t.Run("signal ready", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		cmd := exec.Command(os.Args[0], "-test.run=^TestReexecHelperProcess$")
		cmd.Env = append(
			os.Environ(),
			reexecWaitHelperEnv+"="+reexecWaitHelperEnvSignalReady,
		)

		proc := &Process{
			cmd:  cmd,
			done: make(chan struct{}),
		}
		require.NoError(t, proc.start(t.Context()))

		childErr, err := proc.waitReady(t.Context())
		require.NoError(t, err)
		require.Empty(t, childErr)
		<-proc.done
	})

	t.Run("process error", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		proc := &Process{
			cmd:  exec.Command("sh", "-c", "exit 255"),
			done: make(chan struct{}),
		}
		require.NoError(t, proc.start(t.Context()))

		childErr, err := proc.waitReady(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "networking process exited before signaling ready")
		require.Empty(t, childErr)
		<-proc.done
	})

	t.Run("process exited with stderr", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		proc := &Process{
			cmd:  exec.Command("sh", "-c", "printf 'Failed to launch' >&2 && exit 255"),
			done: make(chan struct{}),
		}
		require.NoError(t, proc.start(t.Context()))

		childErr, err := proc.waitReady(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "networking process exited before signaling ready")
		require.Equal(t, "Failed to launch", childErr)
		<-proc.done
	})

	t.Run("process exited with stderr unbounded", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		// Write 256KiB to overload the stderr pipe. If the stderr isn't being actively drained,
		// this would lead to the process failing to finish printing and thus never exiting.
		proc := &Process{
			cmd:  exec.Command("bash", "-c", "printf 'x%.0s' {1..262144} >&2 && exit 255"),
			done: make(chan struct{}),
		}
		require.NoError(t, proc.start(t.Context()))

		childErr, err := proc.waitReady(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "networking process exited before signaling ready")
		expectStderr := strings.Repeat("x", stderrMaxRead)
		require.Equal(t, expectStderr, childErr)
		<-proc.done
	})

	t.Run("context cancellation", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		cmd := exec.Command(os.Args[0], "-test.run=^TestReexecHelperProcess$")
		cmd.Env = append(
			os.Environ(),
			reexecWaitHelperEnv+"="+reexecWaitHelperEnvWaitClose,
		)

		proc := &Process{
			cmd:  cmd,
			done: make(chan struct{}),
		}
		require.NoError(t, proc.start(t.Context()))

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		childErr, err := proc.waitReady(ctx)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, childErr)
		<-proc.done
	})
}

const (
	reexecWaitHelperEnv            = "TELEPORT_REEXEC_WAIT_HELPER_ERROR"
	reexecWaitHelperEnvWaitClose   = "waitClose"
	reexecWaitHelperEnvSignalReady = "sendReady"
)

func TestReexecHelperProcess(t *testing.T) {
	if os.Getenv(reexecWaitHelperEnv) == "" {
		return
	}

	ffd := os.NewFile(3, "listener")
	if ffd == nil {
		os.Exit(1)
	}

	switch os.Getenv(reexecWaitHelperEnv) {
	case reexecWaitHelperEnvWaitClose:
	case reexecWaitHelperEnvSignalReady:
		// Signal ready.
		parentConn, err := uds.FromFile(ffd)
		_ = ffd.Close()
		if err != nil {
			os.Exit(1)
		}
		if _, err := parentConn.Write(make([]byte, 1)); err != nil {
			os.Exit(1)
		}
	}

	// Wait for the other side of the connection to close and exit immediately.
	ffd.Read(make([]byte, 1))
	os.Exit(1)
}
