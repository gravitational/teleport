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

//go:build linux

package reexec

import (
	"os"
	"os/exec"
	"syscall"
)

// getExecutable gets the path to the executable that should be used for re-exec.
func getExecutable() (string, error) {
	return "/proc/self/exe", nil
}

// configureReexecForOS configures the command with files to inherit and
// os-specific tweaks.
func configureReexecForOS(cmd *exec.Cmd, signal, kill *os.File) (signalFd, killFd uint64) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	cmd.ExtraFiles = []*os.File{signal, kill}
	return 3, 4
}
