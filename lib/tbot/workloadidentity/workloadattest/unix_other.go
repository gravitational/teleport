//go:build !linux

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

	"github.com/shirou/gopsutil/v4/process"
)

var unixOS UnixOS = nonLinux{}

// nonLinux implements the UnixOS interface for non-Linux systems.
type nonLinux struct{}

func (nonLinux) ExePath(ctx context.Context, proc *process.Process) (string, error) {
	return proc.ExeWithContext(ctx)
}

func (n nonLinux) OpenExe(ctx context.Context, proc *process.Process) (io.ReadCloser, error) {
	path, err := n.ExePath(ctx, proc)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
