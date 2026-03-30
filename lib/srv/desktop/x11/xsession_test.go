package x11

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
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

	entries, err := GetAvailableXSessions(nil, nil)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "startxfce4", entries["Xfce Session"])
	require.Equal(t, "start-plasma", entries["KDE Plasma"])

	included, err := regexp.Compile(`xf`)
	require.NoError(t, err)
	entries, err = GetAvailableXSessions(included, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "startxfce4", entries["Xfce Session"])

	excluded, err := regexp.Compile(`ce`)
	require.NoError(t, err)
	entries, err = GetAvailableXSessions(nil, excluded)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "start-plasma", entries["KDE Plasma"])

	require.NoError(t, err)
	entries, err = GetAvailableXSessions(included, excluded)
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

func TestStartTeleportExecXSession(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)
	username := current.Username

	logger := slog.Default()
	cfg := func() *XSessionConfig {
		config := srv.ExecLogConfig{
			Level: slog.LevelDebug,
		}
		return &XSessionConfig{
			Logger:   logger,
			Username: username,
			Login:    username,
			ChildLogConfig: &srv.ChildLogConfig{
				ExecLogConfig: config,
				Writer:        os.Stderr,
			},
			Display: ":0",
		}
	}
	t.Run("valid command", func(t *testing.T) {
		config := cfg()
		config.Command = "echo a"
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
		config.Command = "echo a"
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
		config.Command = "[ \"$DISPLAY\" = \":0\" ]"
		cmd, err := StartTeleportExecXSession(t.Context(), config)
		require.NoError(t, err)
		err = cmd.Wait()
		require.NoError(t, err)
	})
}
