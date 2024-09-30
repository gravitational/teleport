//go:build windows

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
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/gravitational/trace"
)

var (
	kernel = windows.NewLazyDLL("kernel32.dll")
	proc   = kernel.NewProc("CreateFileW")
)

// Lock implements filesystem lock for blocking another process execution until this lock is released.
func Lock(ctx context.Context, lockFile string) (func(), error) {
	lockPath, err := windows.UTF16PtrFromString(lockFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var lf *os.File
	for lf == nil {
		fd, _, err := proc.Call(
			uintptr(unsafe.Pointer(lockPath)),
			uintptr(windows.GENERIC_READ|windows.GENERIC_WRITE),
			// Exclusive lock, for shared must be used: uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE).
			uintptr(0),
			uintptr(0),
			uintptr(windows.OPEN_ALWAYS),
			uintptr(windows.FILE_ATTRIBUTE_NORMAL),
			0,
		)
		switch err.(windows.Errno) {
		case windows.NO_ERROR, windows.ERROR_ALREADY_EXISTS:
			lf = os.NewFile(fd, lockFile)
		case windows.ERROR_SHARING_VIOLATION:
			// if the file is locked by another process we have to wait until the next check.
			time.Sleep(time.Second)
		default:
			windows.CloseHandle(windows.Handle(fd))
			return nil, trace.Wrap(err)
		}
	}

	rc, err := lf.SyscallConn()
	if err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}
	if err := rc.Control(func(fd uintptr) {
		err = windows.SetHandleInformation(windows.Handle(fd), windows.HANDLE_FLAG_INHERIT, 1)
	}); err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}
	if err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}

	return func() {
		if err := lf.Close(); err != nil {
			slog.DebugContext(ctx, "failed to close lock file", "file", lf, "error", err)
		}
	}, nil
}
