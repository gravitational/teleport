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
	path, exists := os.LookupEnv("TELEPORT_XSESSIONS_PATH")
	if !exists {
		path = "/usr/share/xsessions"
	}
	entries := make(map[string]string)
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entry := range dirEntries {
		var found bool
		var fileName string
		if fileName, found = strings.CutSuffix(entry.Name(), ".desktop"); !found {
			continue
		}
		if included != nil && !included.MatchString(fileName) {
			continue
		}
		if excluded != nil && excluded.MatchString(fileName) {
			continue
		}
		file, err := os.Open(filepath.Join(path, entry.Name()))
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

	cmdmsg := &reexec.ExecCommand{
		Command:         cfg.Command,
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
