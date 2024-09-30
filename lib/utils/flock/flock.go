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
	"context"
	"log/slog"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

// Lock implements filesystem lock for blocking another process execution until this lock is released.
func Lock(ctx context.Context, lockFile string) (func(), error) {
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
	if err != nil {
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
			slog.DebugContext(ctx, "failed invokes the control", "file", lockFile, "error", err)
		}
		if err != nil {
			slog.DebugContext(ctx, "failed to unlock file", "file", lockFile, "error", err)
		}
		if err := lf.Close(); err != nil {
			slog.DebugContext(ctx, "failed to close lock file", "file", lockFile, "error", err)
		}
	}, nil
}
