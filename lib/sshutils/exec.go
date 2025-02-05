/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"syscall"

	"github.com/gravitational/teleport"
)

// errorWithExitStatus defines an interface that provides an ExitStatus
// function to get the exit code of the process execution.
//
// This interface is introduced so ssh.ExitError can be mocked in unit test.
type errorWithExitStatus interface {
	ExitStatus() int
}

// execExitError defines an interface that provides a Sys function to get exit
// status from the process execution.
//
// This interface is introduced so exec.ExitError can be mocked in unit test.
type execExitError interface {
	Sys() any
}

// ExitCodeFromExecError extracts and returns the exit code from the
// error.
func ExitCodeFromExecError(err error) int {
	// If no error occurred, return 0 (success).
	if err == nil {
		return teleport.RemoteCommandSuccess
	}

	var execExitErr execExitError
	var exitErr errorWithExitStatus
	switch {
	case errors.As(err, &execExitErr):
		waitStatus, ok := execExitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return teleport.RemoteCommandFailure
		}
		return waitStatus.ExitStatus()
	case errors.As(err, &exitErr):
		return exitErr.ExitStatus()
	// An error occurred, but the type is unknown, return a generic 255 code.
	default:
		slog.DebugContext(context.Background(), "Unknown error returned when executing command", "error", err)
		return teleport.RemoteCommandFailure
	}
}
