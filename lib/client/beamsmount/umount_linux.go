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

//go:build linux

package beamsmount

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// unmountCommand returns the command and arguments for unmounting an SSHFS
// mount point on Linux.
//
// fusermount is preferred for FUSE filesystems like SSHFS because it can
// unmount without root privileges — the FUSE subsystem allows the mounting
// user to unmount their own mounts. The -u flag means "unmount", and -z adds
// "lazy" semantics (detach the filesystem immediately, clean up references
// when no longer busy).
//
// If fusermount is not installed (unusual for systems with FUSE/SSHFS, but
// possible in minimal containers), we fall back to the standard umount command.
// This may require elevated privileges (root or CAP_SYS_ADMIN).
func unmountCommand(path string, force bool) (string, []string) {
	// Prefer fusermount if available — it doesn't require root for FUSE mounts.
	if _, err := exec.LookPath("fusermount"); err == nil {
		if force {
			return "fusermount", []string{"-uz", path}
		}
		return "fusermount", []string{"-u", path}
	}

	// Fallback to umount. May require root privileges.
	if force {
		return "umount", []string{"-l", path}
	}
	return "umount", []string{path}
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
