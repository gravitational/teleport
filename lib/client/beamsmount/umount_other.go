/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

//go:build !darwin && !linux

package beamsmount

import (
	"fmt"

	"github.com/gravitational/trace"
)

// unmountCommand is not supported on this platform. SSHFS mounts are only
// supported on macOS and Linux.
func unmountCommand(path string, force bool) (string, []string) {
	return "", nil
}

// Unmount is not supported on this platform.
func Unmount(path string, force bool) error {
	return trace.NotImplemented("unmount is not supported on this platform")
}

// UnmountDescription returns a description of the unmount command.
func UnmountDescription(path string, force bool) string {
	return fmt.Sprintf("unmount %s (not supported on this platform)", path)
}
