// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build unix

package common

import (
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

func isValidForkSignalFd(fd uint64) bool {
	// Don't allow stdin, stdout, or stderr.
	return fd > 2
}

// newSignalFile creates a signaling file for --fork-after-authentication from
// a file descriptor.
func newSignalFile(fd uint64) *os.File {
	syscall.CloseOnExec(int(fd))
	return os.NewFile(uintptr(fd), "disown")
}

// replaceStdin returns a file for /dev/null that should be used from now
// on instead of stdin.
func replaceStdin() (*os.File, error) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rc, err := devNull.SyscallConn()
	if err != nil {
		_ = devNull.Close()
		return nil, trace.Wrap(err)
	}
	var dupErr error
	if ctrlErr := rc.Control(func(fd uintptr) {
		dupErr = syscall.Dup2(int(fd), syscall.Stdin)
		// stdin is not O_CLOEXEC after dup2 but thankfully the three stdio
		// file descriptors must be not O_CLOEXEC anyway, so we can avoid
		// a linux-specific implementation or syscall.ForkLock shenanigans
	}); ctrlErr != nil {
		_ = devNull.Close()
		return nil, trace.Wrap(ctrlErr)
	}
	if dupErr != nil {
		// this is the error from Dup2
		_ = devNull.Close()
		return nil, trace.Wrap(err)
	}
	return devNull, err
}
