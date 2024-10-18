//go:build !windows
// +build !windows

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/gravitational/trace"
)

// PercentUsed returns percentage of disk space used. The percentage of disk
// space used is calculated from (total blocks - free blocks)/total blocks.
// The value is rounded to the nearest whole integer.
func PercentUsed(path string) (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	ratio := float64(stat.Blocks-stat.Bfree) / float64(stat.Blocks)
	return Round(ratio * 100), nil
}

// FreeDiskWithReserve returns the available disk space (in bytes) on the disk at dir, minus `reservedFreeDisk`.
func FreeDiskWithReserve(dir string, reservedFreeDisk uint64) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(dir, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	//nolint:unconvert // The cast is only necessary for linux platform.
	avail := uint64(stat.Bavail) * uint64(stat.Bsize)
	if reservedFreeDisk > avail {
		return 0, trace.Errorf("no free space left")
	}
	return avail - reservedFreeDisk, nil
}

// CanUserWriteTo attempts to check if a user has write access to certain path.
// It also works around the program being run as root and tries to check
// the permissions of the user who executed the program as root.
// This should only be used for string formatting or inconsequential use cases
// as it's not bullet proof and can report wrong results.
func CanUserWriteTo(path string) (bool, error) {
	// prevent infinite loops with a max dir depth
	var fileInfo os.FileInfo
	var err error

	for i := 0; i < 20; i++ {
		fileInfo, err = os.Stat(path)
		if err == nil {
			break
		}
		if errors.Is(err, fs.ErrNotExist) {
			path = filepath.Dir(path)
			continue
		}

		return false, trace.BadParameter("failed to find path: %+v", err)

	}

	var uid int
	var gid int
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
	}

	var usr *user.User
	if ogUser := os.Getenv("SUDO_USER"); ogUser != "" {
		usr, err = user.Lookup(ogUser)
		if err != nil {
			return false, trace.NotFound("could not determine original user: %+v", err)
		}
	} else {
		usr, err = user.Current()
		if err != nil {
			return false, trace.NotFound("could not determine current user: %+v", err)
		}
	}

	perm := fileInfo.Mode().Perm()

	// file is owned by the user
	if strconv.Itoa(uid) == usr.Uid {
		// file has u+wx permissions
		if perm&syscall.S_IWUSR != 0 &&
			perm&syscall.S_IXUSR != 0 {
			return true, nil
		}
	}

	// file and user have a group in common
	groupIDs, err := usr.GroupIds()
	if err != nil {
		return false, trace.NotFound("could not determine current user group ids: %+v", err)
	}
	for _, groupID := range groupIDs {
		if strconv.Itoa(gid) == groupID {
			// file has g+wx permissions
			if perm&syscall.S_IWGRP != 0 &&
				perm&syscall.S_IXGRP != 0 {
				return true, nil
			}
			break
		}
	}

	// file has o+wx permissions
	if perm&syscall.S_IWOTH != 0 &&
		perm&syscall.S_IXOTH != 0 {
		return true, nil
	}

	return false, nil
}
