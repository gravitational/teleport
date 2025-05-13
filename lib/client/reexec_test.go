// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

type syncBuffer struct {
	buf *bytes.Buffer
	mu  sync.Mutex
}

func newSyncBuffer() *syncBuffer {
	return &syncBuffer{
		buf: &bytes.Buffer{},
	}
}

func (rw *syncBuffer) Read(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.buf.Read(b)
}

func (rw *syncBuffer) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.buf.Write(b)
}

func (rw *syncBuffer) String() string {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.buf.String()
}

func buildBashForkCommand(t *testing.T, params ForkAuthenticateParams) *forkAuthCmd {
	cmd, err := buildForkAuthenticateCommand(params)
	require.NoError(t, err)
	bash, err := exec.LookPath("bash")
	require.NoError(t, err)
	cmd.Path = bash
	cmd.Args[0] = bash
	// Ensure that the process doesn't outlive the test.
	t.Cleanup(func() { cmd.Process.Kill() })
	return cmd
}

func TestRunForkAuthenticateChild(t *testing.T) {
	t.Parallel()

	t.Run("child disowns successfully", func(t *testing.T) {
		t.Parallel()
		const script = `
		read
		# Close signal fd.
		exec %d>&-
		# stdout/err should still work.
		echo "stdout: $REPLY"
		echo "stderr: $REPLY" >&2
		# Wait to ensure the fd closure is caught before the process ends.
		sleep 1
		`
		getArgs := func(signalFd uint64) []string {
			return []string{"-c", fmt.Sprintf(script, signalFd)}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		cmd := buildBashForkCommand(t, params)

		err := runForkAuthenticateChild(t.Context(), cmd)
		assert.NoError(t, err)
		assert.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Equal(collect, "stdout: hello\n", stdout.String())
			assert.Equal(collect, "stderr: hello\n", stderr.String())
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("child exits with error", func(t *testing.T) {
		t.Parallel()
		const script = `
		# Make sure stdin/out/err work.
		read
		echo "stdout: $REPLY"
		echo "stderr: $REPLY" >&2
		# Exit with error.
		exit 42
		`
		getArgs := func(signalFd uint64) []string {
			return []string{"-c", script}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		cmd := buildBashForkCommand(t, params)

		err := runForkAuthenticateChild(t.Context(), cmd)
		var execErr *exec.ExitError
		if assert.ErrorAs(t, err, &execErr) {
			assert.Equal(t, 42, execErr.ExitCode())
		}
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()
		getArgs := func(_ uint64) []string {
			return []string{"-c", `
			# Make sure stdin/out/err work.
			read
			echo "stdout: $REPLY"
			echo "stderr: $REPLY" >&2
			# wait for cancellation
			sleep 3
			# should not be executed
			echo "extra output"
			`}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		cmd := buildBashForkCommand(t, params)

		errorCh := make(chan error, 1)
		utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
			Name: "RunForkAuthenticateChild",
			Task: func(ctx context.Context) error {
				errorCh <- runForkAuthenticateChild(ctx, cmd)
				return nil
			},
		})

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Equal(collect, "stdout: hello\n", stdout.String())
			assert.Equal(collect, "stderr: hello\n", stderr.String())
		}, time.Second, 10*time.Millisecond)

		cancel()
		select {
		case err := <-errorCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			require.Fail(t, "timed out waiting for child to finish")
		}

		require.Never(t, func() bool {
			return strings.Contains(stdout.String(), "extra output")
		}, 3*time.Second, time.Second)
	})

	t.Run("stdin is closed after disowning", func(t *testing.T) {
		t.Parallel()
		const script = `
		# Close signal fd.
		echo x >&%d
		exec %d>&-
		echo test
		sleep 1
		# Next read should not work
		read && echo $REPLY
		`
		getArgs := func(signalFd uint64) []string {
			return []string{"-c", fmt.Sprintf(script, signalFd, signalFd)}
		}
		stdout := newSyncBuffer()
		stdinR, stdinW := io.Pipe()
		params := ForkAuthenticateParams{
			GetArgs: getArgs,
			Stdin:   stdinR,
			Stdout:  stdout,
			Stderr:  io.Discard,
		}
		cmd := buildBashForkCommand(t, params)
		err := runForkAuthenticateChild(t.Context(), cmd)
		assert.NoError(t, err)
		stdinW.Write([]byte("hello\n"))
		assert.Never(t, func() bool {
			return strings.Contains(stdout.String(), "hello")
		}, 3*time.Second, time.Second)
	})
}
