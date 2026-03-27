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

//go:build darwin

package beamsmount

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// unmountCommand returns the command and arguments for unmounting an SSHFS
// mount point on macOS.
//
// diskutil is the standard macOS tool for managing disk mounts, including
// FUSE/SSHFS volumes. It cleanly updates the mount table and notifies Finder
// of the change, which prevents stale entries in the sidebar.
func unmountCommand(path string, force bool) (string, []string) {
	if force {
		return "diskutil", []string{"unmount", "force", path}
	}
	return "diskutil", []string{"unmount", path}
}

// Unmount unmounts the SSHFS filesystem at the given path.
func Unmount(path string, force bool) error {
	cmd, args := unmountCommand(path, force)
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "unmount %s failed: %s", path, string(out))
	}
	return nil
}

// UnmountDescription returns a human-readable description of the unmount
// command that will be used, for logging/debug purposes.
func UnmountDescription(path string, force bool) string {
	cmd, args := unmountCommand(path, force)
	return fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
}
