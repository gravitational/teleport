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

package flock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
)

// Lock implements filesystem lock for blocking another process execution until this lock is released.
func Lock(lockFile string, nonblock bool) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(lockFile), 0755); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the advisory lock using flock.
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	how := syscall.LOCK_EX
	if nonblock {
		how |= syscall.LOCK_NB
	}

	err = fdSyscall(lf, func(fd uintptr) error {
		return syscall.Flock(int(fd), how)
	})
	if errors.Is(err, syscall.EAGAIN) {
		return nil, ErrLocked
	}
	if err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}

	return lf.Close, nil
}
