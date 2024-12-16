/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sshutils

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"syscall"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
)

// ExitCodeFromExecError extracts and returns the exit code from the
// error.
func ExitCodeFromExecError(err error) int {
	// If no error occurred, return 0 (success).
	if err == nil {
		return teleport.RemoteCommandSuccess
	}

	var execExitErr *exec.ExitError
	var sshExitErr *ssh.ExitError
	switch {
	// Local execution.
	case errors.As(err, &execExitErr):
		waitStatus, ok := execExitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return teleport.RemoteCommandFailure
		}
		return waitStatus.ExitStatus()
	// Remote execution.
	case errors.As(err, &sshExitErr):
		return sshExitErr.ExitStatus()
	// An error occurred, but the type is unknown, return a generic 255 code.
	default:
		slog.DebugContext(context.Background(), "Unknown error returned when executing command", "error", err)
		return teleport.RemoteCommandFailure
	}
}
