//go:build unix

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

package x11

import (
	"errors"
	"os"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

const (
	// sockDirPerm is the permission set of the directory holding XServer
	// Unix sockets, in the form the raw syscalls take. It is world writable
	// with the sticky bit set, so that sockets for multiple users can live
	// in it while only their owners can remove them.
	sockDirPerm = 0o1777
	// sockDirMkdirMode is sockDirPerm as an os.FileMode.
	sockDirMkdirMode = os.ModeSticky | 0o777
)

// EnsureSockDir creates the directory which holds XServer Unix sockets if it
// doesn't exist already (such as in CI).
func EnsureSockDir() error {
	return trace.Wrap(ensureSockDir(x11SockDir()))
}

func ensureSockDir(dir string) error {
	// Mkdir is subject to umask, so the directory may be created with narrower
	// than intended permissions. This will be fixed in the fchmod below if needed.
	if err := os.Mkdir(dir, sockDirMkdirMode); err != nil && !errors.Is(err, os.ErrExist) {
		return trace.Wrap(err)
	}

	// Operate on an open descriptor rather than on the path. The parent directory
	// is world writable, so an unprivileged user may have won a race to put a
	// symlink or a regular file here, and a chmod resolved through a path could
	// then be aimed at an arbitrary file. O_NOFOLLOW and O_DIRECTORY reject those
	// entries outright, and the fstat below cannot disagree with what the fchown
	// and fchmod that follow it act on.
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return trace.Wrap(err, "opening XServer socket directory %s", dir)
	}
	defer unix.Close(fd)

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return trace.Wrap(err, "inspecting XServer socket directory %s", dir)
	}

	// Select the 12 permission bits (setuid, setgid, sticky, rwx for user/group/other)
	// out of the mode returned by stat, which also carries file type bits above them.
	perm := stat.Mode & 0o7777

	switch {
	case stat.Uid == 0 && stat.Gid == 0 && perm == sockDirPerm:
		// Already usable by any login.
		return nil

	case os.Geteuid() == 0:
		if err := unix.Fchown(fd, 0, 0); err != nil {
			return trace.Wrap(err, "taking ownership of XServer socket directory %s", dir)
		}
		if err := unix.Fchmod(fd, sockDirPerm); err != nil {
			return trace.Wrap(err, "setting permissions on XServer socket directory %s", dir)
		}

		// The previous owner may have renamed the directory away and put another
		// in its place before the fchown, leaving the repair on a detached inode.
		var pathStat unix.Stat_t
		if err := unix.Lstat(dir, &pathStat); err != nil {
			return trace.Wrap(err, "inspecting XServer socket directory %s", dir)
		}
		if pathStat.Dev != stat.Dev || pathStat.Ino != stat.Ino {
			return trace.AccessDenied("XServer socket directory %s was replaced while it was being repaired", dir)
		}

		return nil

	case stat.Uid == uint32(os.Geteuid()):
		// Owned by the current unprivileged user; leave as is. Widening it would let
		// its owner unlink other logins' sockets, so only this login can open a display.
		// Run Teleport as root or install a proper XServer to avoid this issue
		// (primarily seen in CI tests).
		return nil

	default:
		return trace.AccessDenied("XServer socket directory %s is owned by uid %d with mode %#o and cannot be shared between logins without root", dir, stat.Uid, perm)
	}
}
