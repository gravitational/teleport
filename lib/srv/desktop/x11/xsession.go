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
	"bufio"
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	reexecutils "github.com/gravitational/teleport/lib/sshutils/reexec"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/session/envutils"
	"github.com/gravitational/teleport/session/reexec"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

// GetAvailableXSessions return xsessions available in the system with optional filtering
func GetAvailableXSessions(included, excluded *regexp.Regexp) (map[string]string, error) {
	path := cmp.Or(os.Getenv("TELEPORT_XSESSIONS_PATH"), "/usr/share/xsessions")
	entries := make(map[string]string)
	files, err := filepath.Glob(path + "/*.desktop")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, f := range files {
		fileName := strings.TrimSuffix(filepath.Base(f), ".desktop")
		if included != nil && !included.MatchString(fileName) {
			continue
		}
		if excluded != nil && excluded.MatchString(fileName) {
			continue
		}
		file, err := os.Open(f)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		scanner := bufio.NewScanner(file)
		var name string
		var exec string
		for scanner.Scan() {
			if s, found := strings.CutPrefix(scanner.Text(), "Name="); found {
				name = s
			} else if s, found := strings.CutPrefix(scanner.Text(), "Exec="); found {
				exec = s
			}
			if name != "" && exec != "" {
				entries[name] = exec
				break
			}
		}
		file.Close()
	}
	return entries, nil
}

// XSessionConfig is configuration used for starting xsession for specified user.
type XSessionConfig struct {
	Logger *slog.Logger

	// ChildLogConfig contains logger configuration for the child process.
	ChildLogConfig *srv.ChildLogConfig

	// Command is command to execute to start xsession.
	Command string

	// Username is the username associated with the Teleport identity.
	Username string

	// Login is the local *nix account.
	Login string

	// Display is X11 display string (:N) to use for connection to X11 server.
	Display string
	// AuthorityFile is XAuthority file used to secure connection to X11 server.
	AuthorityFile string

	// ScalePercent is the client's display scale as a percentage (100 = 1x,
	// 200 = 2x). IntegerScale rounds it for the DE's HiDPI setting; scales >= 2x
	// also bump XCURSOR_SIZE so the cursor scales with the UI.
	ScalePercent uint16

	// DesktopEnvironment identifies the DE that the selected xsession launches.
	// It selects which settings backend we write the scaling factor to.
	DesktopEnvironment DesktopEnvironment
}

// DesktopEnvironment identifies the desktop environment launched by an
// xsession Exec= command. We use it to pick the right settings backend for
// the HiDPI scaling factor.
type DesktopEnvironment int

const (
	// DesktopEnvironmentUnknown means we couldn't match the Exec command to
	// a DE we know how to configure. HiDPI scaling will not be applied.
	DesktopEnvironmentUnknown DesktopEnvironment = iota
	// DesktopEnvironmentGNOME is GNOME / gnome-session. Scaling is written
	// via gsettings to dconf, picked up by gnome-settings-daemon.
	DesktopEnvironmentGNOME
	// DesktopEnvironmentXFCE is XFCE / startxfce4. Scaling is written via
	// xfconf-query, picked up by xfsettingsd.
	DesktopEnvironmentXFCE
	// DesktopEnvironmentKDE is KDE Plasma. Scaling is written via
	// kwriteconfig (5 or 6) to kdeglobals / kcmfonts, picked up by KWin
	// on reconfigure.
	DesktopEnvironmentKDE
)

// IntegerScale converts a scale percentage to the nearest integer scale factor
// (>= 1) that the per-DE XSETTINGS Gdk/WindowScalingFactor accepts. XSETTINGS is
// integer-only, so 150% rounds to 2x and 130% to 1x. Reported DPI stays fixed at
// 96 (see pixelsToMm).
func IntegerScale(scalePercent uint16) uint16 {
	scale := (uint32(scalePercent) + 50) / 100
	if scale < 1 {
		return 1
	}
	return uint16(scale)
}

// DetectDesktopEnvironment guesses the desktop environment from an xsession
// Exec= command. Returns DesktopEnvironmentUnknown if no known DE matches.
func DetectDesktopEnvironment(exec string) DesktopEnvironment {
	e := strings.ToLower(exec)
	switch {
	case strings.Contains(e, "xfce"):
		return DesktopEnvironmentXFCE
	case strings.Contains(e, "plasma"), strings.Contains(e, "kde"):
		return DesktopEnvironmentKDE
	case strings.Contains(e, "gnome"):
		return DesktopEnvironmentGNOME
	default:
		return DesktopEnvironmentUnknown
	}
}

// busDiscoveryPreamble is a shell snippet that finds the running xsession's
// real D-Bus session bus (dbus-run-session puts it at a temp path) and exports
// DBUS_SESSION_BUS_ADDRESS. No-op if no candidate process is found.
const busDiscoveryPreamble = `for p in $(pgrep -u "$(id -u)" -f "gnome-shell|gnome-session|xfce4-session|plasma" 2>/dev/null); do
    if [ -r "/proc/$p/environ" ]; then
        b=$(tr "\0" "\n" < "/proc/$p/environ" 2>/dev/null | grep "^DBUS_SESSION_BUS_ADDRESS=" | head -1 | cut -d= -f2-)
        if [ -n "$b" ] && [ "$b" != "unix:path=/run/user/$(id -u)/bus" ]; then
            export DBUS_SESSION_BUS_ADDRESS="$b"
            break
        fi
    fi
done; `

// scalingFactorCommand returns the shell command that sets the integer HiDPI
// scaling factor for the given DE, or "" if unsupported. It works both at
// startup (writing the per-user config before the DE daemons read it) and
// mid-session (run against the live bus so the settings daemon republishes
// Gdk/WindowScalingFactor over XSETTINGS). For KDE it also fires a best-effort
// `kwin reconfigure`, a no-op until KWin is running.
func scalingFactorCommand(de DesktopEnvironment, scalePercent uint16) string {
	scale := IntegerScale(scalePercent)
	switch de {
	case DesktopEnvironmentGNOME:
		return fmt.Sprintf("gsettings set org.gnome.desktop.interface scaling-factor %d", scale)
	case DesktopEnvironmentXFCE:
		return fmt.Sprintf("xfconf-query -c xsettings -p /Gdk/WindowScalingFactor --create -t int -s %d", scale)
	case DesktopEnvironmentKDE:
		// KDE/X11 has no single Gdk/WindowScalingFactor equivalent: write
		// per-output ScreenScaleFactors (wildcard) plus forceFontDPI for Qt
		// widget sizing. Prefer kwriteconfig6, fall back to kwriteconfig5.
		return fmt.Sprintf(
			`KW=$(command -v kwriteconfig6 || command -v kwriteconfig5) && `+
				`"$KW" --file kdeglobals --group KScreen --key ScreenScaleFactors "*=%d;" && `+
				`"$KW" --file kcmfonts --group General --key forceFontDPI %d; `+
				`dbus-send --session --type=method_call --dest=org.kde.KWin /KWin org.kde.KWin.reconfigure >/dev/null 2>&1 || true`,
			scale, 96*scale,
		)
	default:
		return ""
	}
}

// StartTeleportExecXSession reexecs the current Teleport binary using
// `teleport exec` and runs the provided start command.
func StartTeleportExecXSession(ctx context.Context, cfg *XSessionConfig) (*reexec.CommandExecutor, error) {
	if cfg.ChildLogConfig == nil {
		return nil, trace.BadParameter("missing parameter ChildLogConfig")
	}

	u, err := user.Lookup(cfg.Login)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimeDir := fmt.Sprintf("/run/user/%s", u.Uid)

	env := envutils.SafeEnv{}
	env.AddTrusted("DISPLAY", cfg.Display)
	env.AddTrusted("XAUTHORITY", cfg.AuthorityFile)
	env.AddTrusted("XDG_RUNTIME_DIR", runtimeDir)
	env.AddTrusted("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s/bus", runtimeDir))
	env.AddTrusted("XDG_SESSION_TYPE", "x11")

	// Persist the DE's scaling-factor setting before launching the session,
	// wrapped in a short-lived dbus-run-session since the DE daemons aren't up
	// yet. They read it on startup and publish Gdk/WindowScalingFactor over
	// XSETTINGS, which GTK and Qt both honor. We avoid GDK_SCALE: it compounds
	// with the XSETTINGS factor and double-scales GTK windows.
	scale := IntegerScale(cfg.ScalePercent)
	command := cfg.Command
	if scaleCmd := scalingFactorCommand(cfg.DesktopEnvironment, cfg.ScalePercent); scaleCmd != "" {
		command = fmt.Sprintf("dbus-run-session -- %s; %s", scaleCmd, cfg.Command)
	}
	if scale >= 2 {
		env.AddTrusted("XCURSOR_SIZE", fmt.Sprintf("%d", 24*scale))
	}

	cmdmsg := &reexec.ExecCommand{
		Command:         command,
		ForceLoginShell: true,
		RequestType:     sshutils.ExecRequest,
		Login:           cfg.Login,
		Username:        cfg.Username,
		Environment:     env,
		LogConfig:       cfg.ChildLogConfig.ExecLogConfig,
	}

	inr, inw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	inw.Close()
	defer inr.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer outw.Close()

	go func() {
		logger := cfg.Logger.With("xsession", cfg.Command)
		scanner := bufio.NewScanner(outr)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Log(ctx, logutils.TraceLevel, "xsession output", "line", line)
		}
		outr.Close()
	}()

	cmd, err := reexec.ConfigureCommand(ctx, cfg.Logger, cfg.ChildLogConfig.Writer, cmdmsg, reexecconstants.ExecSubCommand, map[reexec.FileFD]*os.File{
		reexec.StdinFile:  inr,
		reexec.StdoutFile: outw,
		reexec.StderrFile: outw,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		childErr, err := reexecutils.ReadChildErrorWithContext(stderrR, nil)
		if err != nil {
			cfg.Logger.WarnContext(ctx, "Failed to read child process stderr", "error", err)
			return
		}
		if childErr != "" {
			cfg.Logger.WarnContext(ctx, "Child process returned error", "error", childErr)
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

// UpdateXSessionScale pushes the DE's scaling-factor setting to a running
// xsession by re-execing the per-DE command on the user's live session bus, so
// the settings daemon republishes Gdk/WindowScalingFactor over XSETTINGS. Only
// new GTK/Qt windows pick up the change; toolkits cache the scale at window
// creation. Returns (nil, nil) for unsupported DEs.
func UpdateXSessionScale(ctx context.Context, cfg *XSessionConfig) (*reexec.CommandExecutor, error) {
	if cfg.ChildLogConfig == nil {
		return nil, trace.BadParameter("missing parameter ChildLogConfig")
	}

	scaleCmd := scalingFactorCommand(cfg.DesktopEnvironment, cfg.ScalePercent)
	if scaleCmd == "" {
		return nil, nil
	}
	// The reexec inherits DBUS_SESSION_BUS_ADDRESS=/run/user/$UID/bus (the logind
	// location), but without logind that socket doesn't exist: the real bus is the
	// dbus-run-session one at /tmp/dbus-XXXXXX. Re-discover it from a session
	// process's environ first, or gsettings can't reach the settings daemon.
	scaleCmd = busDiscoveryPreamble + scaleCmd

	u, err := user.Lookup(cfg.Login)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimeDir := fmt.Sprintf("/run/user/%s", u.Uid)

	env := envutils.SafeEnv{}
	env.AddTrusted("DISPLAY", cfg.Display)
	env.AddTrusted("XAUTHORITY", cfg.AuthorityFile)
	env.AddTrusted("XDG_RUNTIME_DIR", runtimeDir)
	env.AddTrusted("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s/bus", runtimeDir))
	env.AddTrusted("XDG_SESSION_TYPE", "x11")

	cmdmsg := &reexec.ExecCommand{
		Command:         scaleCmd,
		ForceLoginShell: true,
		RequestType:     sshutils.ExecRequest,
		Login:           cfg.Login,
		Username:        cfg.Username,
		Environment:     env,
		LogConfig:       cfg.ChildLogConfig.ExecLogConfig,
	}

	inr, inw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	inw.Close()
	defer inr.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer outw.Close()

	go func() {
		logger := cfg.Logger.With("xsession", cmdmsg.Command)
		scanner := bufio.NewScanner(outr)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Log(ctx, logutils.TraceLevel, "xsession scale update output", "line", line)
		}
		outr.Close()
	}()

	cmd, err := reexec.ConfigureCommand(ctx, cfg.Logger, cfg.ChildLogConfig.Writer, cmdmsg, reexecconstants.ExecSubCommand, map[reexec.FileFD]*os.File{
		reexec.StdinFile:  inr,
		reexec.StdoutFile: outw,
		reexec.StderrFile: outw,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		childErr, err := reexecutils.ReadChildErrorWithContext(stderrR, nil)
		if err != nil {
			cfg.Logger.WarnContext(ctx, "Failed to read child process stderr", "error", err)
			return
		}
		if childErr != "" {
			cfg.Logger.WarnContext(ctx, "Child process returned error", "error", childErr)
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}
