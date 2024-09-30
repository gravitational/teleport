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

package update

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// lock implements filesystem lock for blocking another process execution until this lock is released.
func lock(dir string) (func(), error) {
	ctx := context.Background()
	// Build the path to the lock file that will be used by flock.
	lockFile := filepath.Join(dir, ".lock")

	// Create the advisory lock using flock.
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := lf.SyscallConn()
	if err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}
	if err := rc.Control(func(fd uintptr) {
		err = syscall.Flock(int(fd), syscall.LOCK_EX)
	}); err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}

	return func() {
		rc, err := lf.SyscallConn()
		if err != nil {
			_ = lf.Close()
			slog.DebugContext(ctx, "failed to acquire syscall connection", "error", err)
			return
		}
		if err := rc.Control(func(fd uintptr) {
			err = syscall.Flock(int(fd), syscall.LOCK_UN)
		}); err != nil {
			slog.DebugContext(ctx, "failed to unlock file", "file", lockFile, "error", err)
		}
		if err := lf.Close(); err != nil {
			slog.DebugContext(ctx, "failed to close lock file", "file", lockFile, "error", err)
		}
	}, nil
}

// freeDiskWithReserve returns the available disk space.
func freeDiskWithReserve(dir string) (uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if stat.Bsize < 0 {
		return 0, trace.Errorf("invalid size")
	}
	avail := stat.Bavail * uint64(stat.Bsize)
	if reservedFreeDisk > avail {
		return 0, trace.Errorf("no free space left")
	}
	return avail - reservedFreeDisk, nil
}
