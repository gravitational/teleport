package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunForkAuthenticateChild(t *testing.T) {
	t.Parallel()

	t.Run("child disowns successfully", func(t *testing.T) {
		const script = `
		# Make sure stdin/out/err work.
		read
		echo "stdout: $REPLY"
		echo "stderr: $REPLY" >&2
		# Close signal fd.
		exec %d>&-
		# Wait to ensure the fd closure is caught before the process ends.
		sleep 1
		`
		getArgs := func(signalFd uintptr) []string {
			return []string{"-c", fmt.Sprintf(script, signalFd)}
		}
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		params := BuildForkAuthenticateCommandParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		cmd, disownSignal, err := BuildForkAuthenticateCommand(t.Context(), params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		err = RunForkAuthenticateChild(t.Context(), cmd, disownSignal)
		assert.NoError(t, err)
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("child exits with error", func(t *testing.T) {
		const script = `
		# Make sure stdin/out/err work.
		read
		echo "stdout: $REPLY"
		echo "stderr: $REPLY" >&2
		# Exit with error.
		exit 1
		`
		getArgs := func(signalFd uintptr) []string {
			return []string{"-c", script}
		}
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		params := BuildForkAuthenticateCommandParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		cmd, disownSignal, err := BuildForkAuthenticateCommand(t.Context(), params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		err = RunForkAuthenticateChild(t.Context(), cmd, disownSignal)
		var execErr *exec.ExitError
		if assert.ErrorAs(t, err, &execErr) {
			assert.Equal(t, 1, execErr.ExitCode())
		}
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("context canceled", func(t *testing.T) {
		getArgs := func(_ uintptr) []string {
			return []string{"-c", "sleep 10"}
		}
		params := BuildForkAuthenticateCommandParams{
			GetArgs: getArgs,
			Stdin:   &bytes.Buffer{},
			Stdout:  io.Discard,
			Stderr:  io.Discard,
		}
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		t.Cleanup(cancel)
		cmd, disownSignal, err := BuildForkAuthenticateCommand(ctx, params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		err = RunForkAuthenticateChild(ctx, cmd, disownSignal)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
