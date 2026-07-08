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
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/safetext/shsprintf"
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

// knownSessionWrappers are the X session wrapper scripts shipped by
// major distros, in priority order.
var knownSessionWrappers = []string{
	"/etc/X11/Xsession",                // Debian/Ubuntu (x11-common)
	"/etc/X11/xinit/Xsession",          // Fedora/RHEL/CentOS (xorg-x11-xinit)
	"/etc/gdm3/Xsession",               // GDM on Debian/Ubuntu
	"/etc/gdm/Xsession",                // GDM on Fedora/RHEL
	"/usr/share/sddm/scripts/Xsession", // SDDM

	// TODO(zmb3): confirm the openSUSE/SLES wrapper path on a real system
	// (under /etc/X11/xinit/ and /etc/X11/xdm/Xsession) and add it here
}

// XSessionConfig is configuration used for starting xsession for specified user.
type XSessionConfig struct {
	Logger *slog.Logger

	// ChildLogConfig contains logger configuration for the child process.
	ChildLogConfig *srv.ChildLogConfig

	// Command is command to execute to start xsession.
	Command string

	// SessionWrapper is an optional path to an X session wrapper script used to
	// launch the session (e.g. /etc/X11/Xsession). When empty, a set of
	// well-known wrapper paths is probed.
	SessionWrapper string

	// Username is the username associated with the Teleport identity.
	Username string

	// Login is the local *nix account.
	Login string

	// Display is X11 display string (:N) to use for connection to X11 server.
	Display string
	// AuthorityFile is XAuthority file used to secure connection to X11 server.
	AuthorityFile string
}

// StartTeleportExecXSession reexecs the current Teleport binary using
// `teleport exec` and runs the provided start command.
func StartTeleportExecXSession(ctx context.Context, cfg *XSessionConfig) (*reexec.CommandExecutor, error) {
	if cfg.ChildLogConfig == nil {
		return nil, trace.BadParameter("missing parameter ChildLogConfig")
	}

	if _, err := user.Lookup(cfg.Login); err != nil {
		return nil, trace.Wrap(err)
	}

	env := envutils.SafeEnv{}
	env.AddTrusted("DISPLAY", cfg.Display)
	env.AddTrusted("XAUTHORITY", cfg.AuthorityFile)
	env.AddTrusted("XDG_SESSION_TYPE", "x11")

	cmdd, err := resolveSessionCommand(cfg.Command, discoverSessionWrapper(ctx, cfg.Logger, cfg.SessionWrapper))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if wrapped, ok := wrapWithDBusSession(cmdd, exec.LookPath); ok {
		cmdd = wrapped
	} else {
		cfg.Logger.WarnContext(ctx, "No D-Bus session launcher (dbus-run-session or dbus-launch and dbus-daemon) found; "+
			"the session may fail to start without a D-Bus session bus. Install the 'dbus' or 'dbus-x11' package. "+
			"On OpenSUSE you also need 'dbus-1-daemon' package.")
	}

	cmdmsg := &reexec.ExecCommand{
		Command:         cmdd,
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
		logger := cfg.Logger.With("xsession", cmdd)
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

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer stderrW.Close()

	cmd.Stderr = stderrW
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

// discoverSessionWrapper resolves the X session wrapper script to use.
// Prefers a wrapper explicitly-configured via file config, falling back to a
// set of well-known wrapper scripts.
// An empty string means no wrapper was found, in which case the session Exec value is run directly.
func discoverSessionWrapper(ctx context.Context, logger *slog.Logger, configured string) string {
	if configured != "" {
		if !isExecutableFile(configured) {
			logger.WarnContext(ctx, "Configured X session wrapper is not an executable file, using it anyway", "wrapper", configured)
		}
		return configured
	}

	for _, candidate := range knownSessionWrappers {
		if isExecutableFile(candidate) {
			logger.DebugContext(ctx, "Discovered X session wrapper", "wrapper", candidate)
			return candidate
		}
	}

	logger.DebugContext(ctx, "No X session wrapper found, sessions will be executed directly")
	return ""
}

// isExecutableFile reports whether path is a regular file with an executable
// permission bit set.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// resolveSessionCommand turns the raw Exec value from an xsession .desktop file
// into the command line to execute.
func resolveSessionCommand(execValue, wrapper string) (string, error) {
	execValue = strings.TrimSpace(execValue)
	if execValue == "" {
		return "", trace.BadParameter("xsession has an empty Exec value")
	}

	if wrapper != "" {
		return wrapper + " " + shsprintf.EscapeDefaultContext(execValue), nil
	}

	return execValue, nil
}

// dbusSessionLaunchers lists the tools that start a private D-Bus session bus
// for the session, in preference order.
//
// dbus-run-session is the modern tool, shipped in the base dbus package and
// present on most current distros.
// dbus-launch is the legacy tool from the dbus-x11 package, used as a fallback
// for older distros that lack dbus-run-session.
//
// Each launcher runs the rest of the command line as the program to execute on
// the new bus, so the trailing argument separator differs per tool.
var dbusSessionLaunchers = []struct {
	name         string
	args         string
	dependencies []string
}{
	{name: "dbus-run-session", args: "--"},
	{name: "dbus-launch", args: "--exit-with-session", dependencies: []string{"dbus-daemon"}},
}

// wrapWithDBusSession prefixes cmd with the first available D-Bus session
// launcher so the session runs on its own private session bus, isolated from
// any other login session of the same user.
func wrapWithDBusSession(cmd string, lookPath func(string) (string, error)) (string, bool) {
MAIN:
	for _, launcher := range dbusSessionLaunchers {
		path, err := lookPath(launcher.name)
		if err != nil {
			continue
		}
		for _, name := range launcher.dependencies {
			_, err := lookPath(name)
			if err != nil {
				continue MAIN
			}
		}
		return path + " " + launcher.args + " " + cmd, true
	}
	return cmd, false
}
