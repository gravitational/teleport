//go:build !windows
// +build !windows

/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
