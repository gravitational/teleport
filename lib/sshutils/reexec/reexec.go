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
	"io"
	"strings"

	"github.com/gravitational/trace"
)

const maxRead = 4096

// ReadChildError reads the child process's stderr pipe and returns it as a string.
// If the stderr pipe is empty, an empty string and nil error is returned.
func ReadChildError(stderr io.Reader) (string, error) {
	// Read the error msg from stderr.
	errMsg := new(strings.Builder)
	if _, err := io.Copy(errMsg, io.LimitReader(stderr, maxRead)); err != nil {
		return "", trace.Wrap(err, "Failed to read error message from child process")
	}

	// TODO(Joerger): Process the err msg from stderr to provide deeper insights into
	// the cause of the session failure to add to the error message.
	// e.g. user unknown because host user creation denied.

	return errMsg.String(), nil
}
