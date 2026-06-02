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
	"fmt"
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

func TestDetectDesktopEnvironment(t *testing.T) {
	tests := []struct {
		exec string
		want DesktopEnvironment
	}{
		{"gnome-session", DesktopEnvironmentGNOME},
		{"/usr/bin/gnome-session --session=gnome", DesktopEnvironmentGNOME},
		{"GNOME-SESSION", DesktopEnvironmentGNOME},
		{"startxfce4", DesktopEnvironmentXFCE},
		{"/usr/bin/startxfce4", DesktopEnvironmentXFCE},
		{"start-plasma", DesktopEnvironmentKDE},
		{"/usr/bin/startplasma-x11", DesktopEnvironmentKDE},
		{"startkde", DesktopEnvironmentKDE},
		{"", DesktopEnvironmentUnknown},
		{"i3", DesktopEnvironmentUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.exec, func(t *testing.T) {
			require.Equal(t, tt.want, DetectDesktopEnvironment(tt.exec))
		})
	}
}

func TestIntegerScale(t *testing.T) {
	tests := []struct {
		percent uint16
		want    uint16
	}{
		{0, 1},
		{49, 1},
		{50, 1},
		{100, 1},
		{124, 1},
		{125, 1}, // 1.25 rounds to 1 with the standard ".50 rounds up" rule
		{149, 1}, // 1.49 rounds to 1
		{150, 2}, // 1.50 rounds to 2 (main fix)
		{175, 2}, // 1.75 rounds to 2
		{200, 2},
		{249, 2},
		{250, 3},
		{300, 3},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.percent), func(t *testing.T) {
			require.Equal(t, tt.want, IntegerScale(tt.percent))
		})
	}
}

func TestScalingFactorCommand(t *testing.T) {
	tests := []struct {
		name  string
		de    DesktopEnvironment
		scale uint16
		want  string
	}{
		{"gnome 2x", DesktopEnvironmentGNOME, 200, "gsettings set org.gnome.desktop.interface scaling-factor 2"},
		{"gnome 1x", DesktopEnvironmentGNOME, 100, "gsettings set org.gnome.desktop.interface scaling-factor 1"},
		{"gnome clamped to 1", DesktopEnvironmentGNOME, 0, "gsettings set org.gnome.desktop.interface scaling-factor 1"},
		{"gnome 150% rounds up", DesktopEnvironmentGNOME, 150, "gsettings set org.gnome.desktop.interface scaling-factor 2"},
		{"gnome 175% rounds up", DesktopEnvironmentGNOME, 175, "gsettings set org.gnome.desktop.interface scaling-factor 2"},
		{"xfce 2x", DesktopEnvironmentXFCE, 200, "xfconf-query -c xsettings -p /Gdk/WindowScalingFactor --create -t int -s 2"},
		{"xfce 1x", DesktopEnvironmentXFCE, 100, "xfconf-query -c xsettings -p /Gdk/WindowScalingFactor --create -t int -s 1"},
		{"kde 2x", DesktopEnvironmentKDE, 200, `KW=$(command -v kwriteconfig6 || command -v kwriteconfig5) && "$KW" --file kdeglobals --group KScreen --key ScreenScaleFactors "*=2;" && "$KW" --file kcmfonts --group General --key forceFontDPI 192; dbus-send --session --type=method_call --dest=org.kde.KWin /KWin org.kde.KWin.reconfigure >/dev/null 2>&1 || true`},
		{"unknown empty", DesktopEnvironmentUnknown, 200, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, scalingFactorCommand(tt.de, tt.scale))
		})
	}
}
