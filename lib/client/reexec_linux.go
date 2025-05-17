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

package client

import (
	"os"
	"os/exec"
)

// getExecutable gets the path to the executable that should be used for re-exec.
func getExecutable() (string, error) {
	return "/proc/self/exe", nil
}

// addSignalFdToChild adds a file for the child process to inherit and returns
// the file descriptor of the file for the child.
func addSignalFdToChild(cmd *exec.Cmd, signal *os.File) uint64 {
	cmd.ExtraFiles = append(cmd.ExtraFiles, signal)
	return uint64(len(cmd.ExtraFiles) + 2)
}
