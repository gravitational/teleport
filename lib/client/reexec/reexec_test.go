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

package reexec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils"
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

func TestRunForkAuthenticate(t *testing.T) {
	t.Parallel()

	t.Run("child disowns successfully", func(t *testing.T) {
		t.Parallel()
		const script = `
		read
		# Close signal fd.
		echo x >&%d
		exec %d>&-
		# stdout/err should still work.
		echo "stdout: $REPLY"
		echo "stderr: $REPLY" >&2
		# Wait to ensure the fd closure is caught before the process ends.
		sleep 1
		`
		getArgs := func(signalFd, killFd uint64) []string {
			return []string{"-c", fmt.Sprintf(script, signalFd, signalFd)}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs:    getArgs,
			executable: "bash",
			Stdin:      bytes.NewBufferString("hello\n"),
			Stdout:     stdout,
			Stderr:     stderr,
		}

		err := RunForkAuthenticate(t.Context(), params)
		assert.NoError(t, err)
		assert.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, "stdout: hello\n", stdout.String())
			assert.Equal(t, "stderr: hello\n", stderr.String())
		}, 10*time.Second, 100*time.Millisecond)
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
		getArgs := func(signalFd, killFd uint64) []string {
			return []string{"-c", script}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs:    getArgs,
			executable: "bash",
			Stdin:      bytes.NewBufferString("hello\n"),
			Stdout:     stdout,
			Stderr:     stderr,
		}

		err := RunForkAuthenticate(t.Context(), params)
		var execErr *exec.ExitError
		if assert.ErrorAs(t, err, &execErr) {
			assert.Equal(t, 42, execErr.ExitCode())
		}
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()
		getArgs := func(_, _ uint64) []string {
			return []string{"-c", `
			# Make sure stdin/out/err work.
			read
			echo "stdout: $REPLY"
			echo "stderr: $REPLY" >&2
			# wait for cancellation
			sleep 2
			# should not be executed
			echo "extra output"
			`}
		}
		stdout := newSyncBuffer()
		stderr := newSyncBuffer()
		params := ForkAuthenticateParams{
			GetArgs:    getArgs,
			executable: "bash",
			Stdin:      bytes.NewBufferString("hello\n"),
			Stdout:     stdout,
			Stderr:     stderr,
		}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		errorCh := make(chan error, 1)
		testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
			Name: "RunForkAuthenticate",
			Task: func(ctx context.Context) error {
				errorCh <- RunForkAuthenticate(ctx, params)
				return nil
			},
		})

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, "stdout: hello\n", stdout.String())
			assert.Equal(t, "stderr: hello\n", stderr.String())
		}, 10*time.Second, 100*time.Millisecond)

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
}
