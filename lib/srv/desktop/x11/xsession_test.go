// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package x11

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/session/reexec"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	modules.SetInsecureTestMode(true)
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if reexec.IsReexec() {
		reexec.RunAndExit(os.Args[1])
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func TestGetAvailableXSessions(t *testing.T) {
	fixtureDir, err := filepath.Abs("testdata")
	require.NoError(t, err)
	t.Setenv("TELEPORT_XSESSIONS_PATH", fixtureDir)

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
	require.Empty(t, entries)
}

func TestResolveSessionCommand(t *testing.T) {
	t.Run("wrapper passes Exec as a single quoted argument", func(t *testing.T) {
		cmd, err := resolveSessionCommand("default", "/etc/X11/Xsession")
		require.NoError(t, err)
		require.Equal(t, "/etc/X11/Xsession default", cmd)
	})
	t.Run("wrapper escapes embedded single quotes", func(t *testing.T) {
		cmd, err := resolveSessionCommand("start 'it'", "/etc/X11/Xsession")
		require.NoError(t, err)
		require.Equal(t, `/etc/X11/Xsession start\ \'it\'`, cmd)
	})
	t.Run("no wrapper runs Exec directly", func(t *testing.T) {
		cmd, err := resolveSessionCommand("startxfce4", "")
		require.NoError(t, err)
		require.Equal(t, "startxfce4", cmd)
	})
	t.Run("empty Exec is rejected", func(t *testing.T) {
		_, err := resolveSessionCommand("   ", "/etc/X11/Xsession")
		require.Error(t, err)
	})
}

func TestWrapWithDBusSession(t *testing.T) {
	found := func(names ...string) func(string) (string, error) {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		return func(name string) (string, error) {
			if set[name] {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		}
	}

	t.Run("prefers dbus-run-session", func(t *testing.T) {
		cmd, ok := wrapWithDBusSession("/etc/X11/Xsession 'default'", found("dbus-run-session", "dbus-launch"))
		require.True(t, ok)
		require.Equal(t, "/usr/bin/dbus-run-session -- /etc/X11/Xsession 'default'", cmd)
	})
	t.Run("falls back to dbus-launch", func(t *testing.T) {
		cmd, ok := wrapWithDBusSession("startxfce4", found("dbus-launch", "dbus-daemon"))
		require.True(t, ok)
		require.Equal(t, "/usr/bin/dbus-launch --exit-with-session startxfce4", cmd)
	})
	t.Run("no dbus-daemon", func(t *testing.T) {
		cmd, ok := wrapWithDBusSession("startxfce4", found("dbus-launch"))
		require.False(t, ok)
		require.Equal(t, "startxfce4", cmd)
	})
	t.Run("no launcher available", func(t *testing.T) {
		cmd, ok := wrapWithDBusSession("startxfce4", found())
		require.False(t, ok)
		require.Equal(t, "startxfce4", cmd)
	})
}

func TestDiscoverSessionWrapper(t *testing.T) {
	logger := slog.Default()

	executable := filepath.Join(t.TempDir(), "Xsession")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755))

	nonExecutable := filepath.Join(t.TempDir(), "Xsession")
	require.NoError(t, os.WriteFile(nonExecutable, []byte("#!/bin/sh\n"), 0o644))

	t.Run("configured wrapper is honored verbatim", func(t *testing.T) {
		require.Equal(t, executable, discoverSessionWrapper(t.Context(), logger, executable))
	})
	t.Run("configured wrapper honored even if not executable", func(t *testing.T) {
		require.Equal(t, nonExecutable, discoverSessionWrapper(t.Context(), logger, nonExecutable))
	})
	t.Run("isExecutableFile checks regular executable file", func(t *testing.T) {
		require.True(t, isExecutableFile(executable))
		require.False(t, isExecutableFile(nonExecutable))
		require.False(t, isExecutableFile(filepath.Join(t.TempDir(), "missing")))
		require.False(t, isExecutableFile(t.TempDir()))
	})
}

func TestStartTeleportExecXSession(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)
	username := current.Username

	logger := slog.Default()
	cfg := func() *XSessionConfig {
		config := reexec.ExecLogConfig{
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
			Display:        ":0",
			SessionWrapper: "sh -c",
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
		_, err := StartTeleportExecXSession(t.Context(), config)
		require.Error(t, err)
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
