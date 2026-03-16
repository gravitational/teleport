package x11

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/stretchr/testify/require"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	modules.SetInsecureTestMode(true)
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if srv.IsReexec() {
		srv.RunAndExit(os.Args[1])
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func TestGetAvailableXSessions(t *testing.T) {
	_, helperFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(helperFile), "testdata")
	require.NoError(t, os.Setenv("TELEPORT_XSESSIONS_PATH", fixtureDir))

	entries, err := GetAvailableXSessions()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, entries["Xfce Session"], "startxfce4")
	require.Equal(t, entries["KDE Plasma"], "start-plasma")
}

func TestStartTeleportExecXSession(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)
	username := current.Username

	logger := slog.Default()
	cfg := func() *XSessionConfig {
		return &XSessionConfig{
			Logger:   logger,
			Username: username,
			Login:    username,
			LogConfig: &srv.ChildLogConfig{
				ExecLogConfig: srv.ExecLogConfig{
					Level: &slog.LevelVar{},
				},
				Writer: io.Discard,
			},
			Display: ":0",
		}
	}
	t.Run("valid command", func(t *testing.T) {
		config := cfg()
		config.Command = "sh -c 'echo a'"
		cmd, err := StartTeleportExecXSession(t.Context(), config)
		require.NoError(t, err)
		err = cmd.Wait()
		require.NoError(t, err)
	})
	t.Run("invalid command", func(t *testing.T) {
		config := cfg()
		config.Command = "invalid-command"
		cmd, err := StartTeleportExecXSession(t.Context(), config)
		require.NoError(t, err)
		err = cmd.Wait()
		require.Error(t, err)
		var exitError *exec.ExitError
		ok := errors.As(err, &exitError)
		require.True(t, ok)
	})
	t.Run("invalid user", func(t *testing.T) {
		config := cfg()
		config.Command = "sh -c 'echo a'"
		config.Login = "invalid-username"
		cmd, err := StartTeleportExecXSession(t.Context(), config)
		require.NoError(t, err)
		err = cmd.Wait()
		require.Error(t, err)
		var exitError *exec.ExitError
		ok := errors.As(err, &exitError)
		require.True(t, ok)
	})
	t.Run("correct DISPLAY", func(t *testing.T) {
		config := cfg()
		config.Command = "sh -c '[ \"$DISPLAY\" = \":0\" ]'"
		cmd, err := StartTeleportExecXSession(t.Context(), config)
		require.NoError(t, err)
		err = cmd.Wait()
		require.NoError(t, err)
	})
}
