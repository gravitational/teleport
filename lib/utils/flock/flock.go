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

package flock

import (
	"errors"
	"os"

	"github.com/gravitational/trace"
)

var (
	// ErrLocked is returned when file is already locked for non-blocking lock.
	ErrLocked = errors.New("file already locked")
)

// fdSyscall should be used instead of f.Fd() when performing syscalls on fds.
// Context: https://github.com/golang/go/issues/24331
func fdSyscall(f *os.File, fn func(uintptr) error) error {
	rc, err := f.SyscallConn()
	if err != nil {
		return trace.Wrap(err)
	}
	if cErr := rc.Control(func(fd uintptr) {
		err = fn(fd)
	}); cErr != nil {
		return trace.Wrap(cErr)
	}
	return trace.Wrap(err)
}
