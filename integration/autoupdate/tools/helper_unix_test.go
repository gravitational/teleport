//go:build !windows

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

package tools_test

import (
	"errors"
	"syscall"

	"github.com/gravitational/trace"
)

// sendInterrupt sends a SIGINT to the process.
func sendInterrupt(pid int) error {
	err := syscall.Kill(pid, syscall.SIGINT)
	if errors.Is(err, syscall.ESRCH) {
		return trace.BadParameter("can't find the process: %v", pid)
	}
	return trace.Wrap(err)
}
