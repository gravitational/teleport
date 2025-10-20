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

package utils

import (
	"fmt"
	"os"
	"runtime"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// CreateLockedPIDFile creates a PID file in the path specified by pidFile
// containing the current PID, atomically swapping it in the final place and
// leaving it with an exclusive advisory lock that will get released when the
// process ends, for the benefit of "pkill -L".
func CreateLockedPIDFile(pidFile string) error {
	pending, err := renameio.NewPendingFile(pidFile, renameio.WithPermissions(0o644))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer pending.Cleanup()
	if _, err := fmt.Fprintf(pending, "%v\n", os.Getpid()); err != nil {
		return trace.ConvertSystemError(err)
	}

	const minimumDupFD = 3 // skip stdio
	locker, err := unix.FcntlInt(pending.Fd(), unix.F_DUPFD_CLOEXEC, minimumDupFD)
	runtime.KeepAlive(pending)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := unix.Flock(locker, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = unix.Close(locker)
		return trace.ConvertSystemError(err)
	}
	// deliberately leak the fd to hold the lock until the process dies

	if err := pending.CloseAtomicallyReplace(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}
