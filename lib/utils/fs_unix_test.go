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
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tests contained here only function on unix systems.

func setupRecursiveChownFiles(t *testing.T) (string, string, string, string) {
	// Setup will produce the following structure under the temp directory created below:
	// dir1/
	// dir1/file
	// dir2/
	// dir2/file-s -> dir1/file
	rootDir := t.TempDir()

	dir1Path := filepath.Join(rootDir, "dir1")
	require.NoError(t, os.Mkdir(dir1Path, 0755))

	dir1FilePath := filepath.Join(dir1Path, "file")
	f, err := os.Create(dir1FilePath)
	require.NoError(t, err)
	f.Close()

	dir2Path := filepath.Join(rootDir, "dir2")
	require.NoError(t, os.Mkdir(dir2Path, 0755))

	dir2SymlinkToFile := filepath.Join(dir2Path, "file-s")
	err = os.Symlink(dir1FilePath, dir2SymlinkToFile)
	require.NoError(t, err)

	return dir1Path, dir1FilePath, dir2Path, dir2SymlinkToFile
}

func setupRecursiveChownUser(t *testing.T) (int, int, int, int, bool) {
	currentUser, err := user.Current()
	require.NoError(t, err)

	currentUID, err := strconv.Atoi(currentUser.Uid)
	require.NoError(t, err)
	currentGID, err := strconv.Atoi(currentUser.Gid)
	require.NoError(t, err)

	root := os.Geteuid() == 0
	newUid := currentUID + 1
	newGid := currentGID + 1
	if !root {
		// `root` is required to actually change ownership, if running under a normal user we will reduce the validation
		newUid = currentUID
		newGid = currentGID
	}

	return currentUID, currentGID, newUid, newGid, root
}

func verifyOwnership(t *testing.T, path string, uid, gid int) {
	fi, err := os.Lstat(path)
	require.NoError(t, err)
	fiCast := fi.Sys().(*syscall.Stat_t)
	require.Equal(t, uint32(uid), fiCast.Uid)
	require.Equal(t, uint32(gid), fiCast.Gid)
}

func TestRecursiveChown(t *testing.T) {
	t.Run("notFoundError", func(t *testing.T) {
		t.Parallel()

		require.Error(t, RecursiveChown("/invalid/path/to/nowhere", 1000, 1000))
	})
	t.Run("simpleChown", func(t *testing.T) {
		t.Parallel()
		_, _, newUid, newGid, _ := setupRecursiveChownUser(t)
		dir1Path, dir1FilePath, _, _ := setupRecursiveChownFiles(t)

		require.NoError(t, RecursiveChown(dir1Path, newUid, newGid))
		// validate ownership matches expected ids
		verifyOwnership(t, dir1Path, newUid, newGid)
		verifyOwnership(t, dir1FilePath, newUid, newGid)
	})
	t.Run("symlinkChown", func(t *testing.T) {
		t.Parallel()
		origUid, origGid, newUid, newGid, root := setupRecursiveChownUser(t)
		if !root {
			t.Skip("Skipping test, root is required")
			return
		}
		_, dir1FilePath, dir2Path, dir2SymlinkToFile := setupRecursiveChownFiles(t)

		require.NoError(t, RecursiveChown(dir2Path, newUid, newGid))
		// Validate symlink has changed
		verifyOwnership(t, dir2SymlinkToFile, newUid, newGid)
		// Validate pointed file has not changed
		verifyOwnership(t, dir1FilePath, origUid, origGid)
	})
}
