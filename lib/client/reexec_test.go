package client

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestForkAuthenticateCommand(
	t *testing.T,
	ctx context.Context,
	getScript func(fd uint64) string,
	cf *CLIConf,
) (cmd *exec.Cmd, disownSignal *os.File) {
	// Set command as if we were doing tsh ssh for real.
	cmd, disownSignal, err := buildForkAuthenticateCommand(ctx, []string{"ssh"}, cf)
	require.NoError(t, err)
	// Args should be "<test binary> ssh --fork-signal-fd <fd>. Extract the fd."
	require.Len(t, cmd.Args, 4)
	fd, err := strconv.ParseUint(cmd.Args[3], 16, 64)
	require.NoError(t, err)
	// Replace Path and Args with bash.
	bash, err := exec.LookPath("bash")
	require.NoError(t, err)
	cmd.Path = bash
	cmd.Args = []string{bash, "-c", getScript(fd)}

	return cmd, disownSignal
}

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
		getScript := func(fd uint64) string {
			return fmt.Sprintf(script, fd)
		}
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cf := &CLIConf{
			overrideStdin:  bytes.NewBufferString("hello\n"),
			OverrideStdout: stdout,
			overrideStderr: stderr,
		}
		cmd, disownSignal := buildTestForkAuthenticateCommand(t, t.Context(), getScript, cf)
		err := RunForkAuthenticateChild(t.Context(), cmd, disownSignal)
		assert.NoError(t, err)
		assert.Equal(t, "stdout: hello\n", stdout.String())
		assert.Equal(t, "stderr: hello\n", stderr.String())
	})

	t.Run("child exits with error", func(t *testing.T) {})

	t.Run("context canceled", func(t *testing.T) {})
}
