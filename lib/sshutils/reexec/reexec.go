/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package reexec contains a common implementation for teleport reexec commands.
package reexec

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

// TODO(Joerger): Isolate reexec logic scattered throughout lib/srv to this package,
// with additional packages for each of the reexec types (sftp, exec, networking, etc).

// ReadChildError reads the child process's stderr pipe for an error.
// If stderr is empty, a nil childErr is returned. If stderr is non-empty and
// looks like "Failed to launch: <internal-error-message>", it is returned as childErr,
// potentially with additional error context gathered from the given
// server context. Otherwise, err is returned.
func ReadChildError(ctx context.Context, stderr io.Reader) (childErr error, err error) {
	// Read the error msg from stderr.
	errMsg := new(strings.Builder)
	if _, err := io.Copy(errMsg, stderr); err != nil {
		return nil, trace.Wrap(err, "Failed to read error message from child process")
	}

	if errMsg.Len() == 0 {
		return nil, nil
	}

	// It should be empty or include an error message like "Failed to launch: ..."
	if !strings.HasPrefix(errMsg.String(), "Failed to launch: ") {
		return nil, trace.Wrap(err, "Unexpected error message from child process: %s", errMsg.String())
	}

	// TODO(Joerger): Process the err msg from stderr to provide deeper insights into
	// the cause of the session failure to add to the error message.
	// e.g. user unknown because host user creation denied.

	return errors.New(errMsg.String()), nil
}
