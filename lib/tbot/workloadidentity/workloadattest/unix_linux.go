//go:build linux

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

package workloadattest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shirou/gopsutil/v4/process"
)

var unixOS UnixOS = linux{}

type linux struct{}

func (l linux) ExePath(ctx context.Context, proc *process.Process) (string, error) {
	return proc.ExeWithContext(ctx)
}

func (l linux) OpenExe(ctx context.Context, proc *process.Process) (io.ReadCloser, error) {
	// On Linux, `/proc/<pid>/exe` is a symlink to the *inode* of the process'
	// executable rather than a simple path, this means it won't change even if
	// you replace the file on disk.
	//
	// In other words, during a rolling deployment the binary's hash won't change
	// until the process has actually been restarted - which is desirable.
	//
	// With one important caveat: network filesystems typically do not guarantee
	// inode stability, so if the process' binary is on a network mount, it's
	// possible the hash won't match the binary the process is actually running.
	return os.Open(l.procPath(strconv.Itoa(int(proc.Pid)), "exe"))
}

func (l linux) procPath(parts ...string) string {
	base := os.Getenv("HOST_PROC")
	if base == "" {
		base = "/proc"
	}
	return filepath.Join(append([]string{base}, parts...)...)
}
