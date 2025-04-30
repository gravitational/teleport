package client

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunForkAuthenticateChild(t *testing.T) {
	t.Parallel()

	t.Run("child disowns successfully", func(t *testing.T) {
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
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		params := BuildForkAuthenticateCommandParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		cmd, err := BuildForkAuthenticateCommand(params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		err = RunForkAuthenticateChild(t.Context(), cmd)
		assert.NoError(t, err)
		assert.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Equal(collect, "stdout: hello\n", stdout.String())
			assert.Equal(collect, "stderr: hello\n", stderr.String())
		}, time.Second, 10*time.Millisecond)
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
		getArgs := func(signalFd uint64) []string {
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
		cmd, err := BuildForkAuthenticateCommand(params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		err = RunForkAuthenticateChild(t.Context(), cmd)
		var execErr *exec.ExitError
		if assert.ErrorAs(t, err, &execErr) {
			assert.Equal(t, 1, execErr.ExitCode())
		}
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("context canceled", func(t *testing.T) {
		getArgs := func(_ uint64) []string {
			return []string{"-c", `
			# Make sure stdin/out/err work.
			read
			echo "stdout: $REPLY"
			echo "stderr: $REPLY" >&2
			# wait for cancellation
			sleep 3
			echo "extra output"
			`}
		}
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		params := BuildForkAuthenticateCommandParams{
			GetArgs: getArgs,
			Stdin:   bytes.NewBufferString("hello\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		cmd, err := BuildForkAuthenticateCommand(params)
		require.NoError(t, err)
		bash, err := exec.LookPath("bash")
		require.NoError(t, err)
		cmd.Path = bash
		cmd.Args[0] = bash

		errorCh := make(chan error, 1)
		utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
			Name: "RunForkAuthenticateChild",
			Task: func(ctx context.Context) error {
				errorCh <- RunForkAuthenticateChild(ctx, cmd)
				return nil
			},
		})

		assert.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Equal(collect, "stdout: hello\n", stdout.String())
			assert.Equal(collect, "stderr: hello\n", stderr.String())
		}, time.Second, 10*time.Millisecond)

		cancel()
		select {
		case err := <-errorCh:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			fmt.Println(stdout.String())
			assert.Fail(t, "timed out waiting for child to finish")
		}
	})
}
