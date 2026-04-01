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

package reexec

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

const (
	defaultPath          = "/bin:/usr/bin:/usr/local/bin:/sbin"
	defaultEnvPath       = "PATH=" + defaultPath
	defaultRootPath      = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	defaultEnvRootPath   = "PATH=" + defaultRootPath
	defaultLoginDefsPath = "/etc/login.defs"
)

// GetDefaultEnvPath returns the default value of PATH environment variable for
// new logins (prior to shell) based on login.defs. Returns a string which
// looks like "PATH=/usr/bin:/bin"
func GetDefaultEnvPath(uid string) string {
	return getDefaultEnvPathWithLoginDefs(uid, defaultLoginDefsPath)
}

func getDefaultEnvPathWithLoginDefs(uid string, loginDefsPath string) string {
	envPath := defaultEnvPath
	envRootPath := defaultEnvRootPath

	// open file, if it doesn't exist return a default path and move on
	f, err := utils.OpenFileAllowingUnsafeLinks(loginDefsPath)
	if err != nil {
		if uid == "0" {
			slog.DebugContext(context.Background(), "Unable to open login.defs, returning default su path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvRootPath)
			return defaultEnvRootPath
		}
		slog.DebugContext(context.Background(), "Unable to open login.defs, returning default path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvPath)
		return defaultEnvPath
	}
	defer f.Close()

	// read path from login.defs file (/etc/login.defs) line by line:
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip comments and empty lines:
		if line == "" || line[0] == '#' {
			continue
		}

		// look for a line that starts with ENV_PATH or ENV_SUPATH
		fields := strings.Fields(line)
		if len(fields) > 1 {
			if fields[0] == "ENV_PATH" {
				envPath = fields[1]
			}
			if fields[0] == "ENV_SUPATH" {
				envRootPath = fields[1]
			}
		}
	}

	// if any error occurs while reading the file, return the default value
	err = scanner.Err()
	if err != nil {
		if uid == "0" {
			slog.WarnContext(context.Background(), "Unable to read login.defs, returning default su path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvRootPath)
			return defaultEnvRootPath
		}
		slog.WarnContext(context.Background(), "Unable to read login.defs, returning default path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvPath)
		return defaultEnvPath
	}

	// if requesting path for uid 0 and no ENV_SUPATH is given, fallback to
	// ENV_PATH first, then the default path.
	if uid == "0" {
		return envRootPath
	}
	return envPath
}

// exitCode extracts and returns the exit code from the error.
func exitCode(err error) int {
	// If no error occurred, return 0 (success).
	if err == nil {
		return reexecconstants.RemoteCommandSuccess
	}

	var execExitErr *exec.ExitError
	switch {
	// Local execution.
	case errors.As(err, &execExitErr):
		waitStatus, ok := execExitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return reexecconstants.RemoteCommandFailure
		}
		return waitStatus.ExitStatus()
	// An error occurred, but the type is unknown, return a generic 255 code.
	default:
		slog.DebugContext(context.Background(), "Unknown error returned when executing command", "error", err)
		return reexecconstants.RemoteCommandFailure
	}
}
