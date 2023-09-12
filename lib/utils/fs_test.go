/*
Copyright 2022 Gravitational, Inc.

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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenFileSymlinks(t *testing.T) {
	// symlink structure setup, this will produce the following structure under the temp directory created below:
	// dir
	// dir-s -> dir
	// dir-s-s -> dir-s
	// dir/file
	// dir/file-s -> dir/file
	// circular-s -> circular-s
	// broken-s -> nonexistent
	rootDir := t.TempDir()

	dirPath := filepath.Join(rootDir, "dir")
	err := os.Mkdir(dirPath, 0755)
	require.NoError(t, err)

	filePath := filepath.Join(dirPath, "file")
	_, err = os.Create(filePath)
	require.NoError(t, err)

	symlinkToFile := filepath.Join(dirPath, "file-s")
	err = os.Symlink(filePath, symlinkToFile)
	require.NoError(t, err)

	symlinkDir := filepath.Join(rootDir, "dir-s")
	err = os.Symlink(dirPath, symlinkDir)
	require.NoError(t, err)

	symlinkToSymlinkDir := filepath.Join(rootDir, "dir-s-s")
	err = os.Symlink(symlinkDir, symlinkToSymlinkDir)
	require.NoError(t, err)

	circularSymlink := filepath.Join(rootDir, "circular-s")
	err = os.Symlink(circularSymlink, circularSymlink)
	require.NoError(t, err)

	brokenSymlink := filepath.Join(rootDir, "broken-s")
	err = os.Symlink(filepath.Join(rootDir, "nonexistent"), brokenSymlink)
	require.NoError(t, err)

	// Tests below can not be done with t.Parallel due to the need for directory cleanup in defer
	t.Run("directFileOpenAllowed", func(t *testing.T) {
		filePath, err = filepath.EvalSymlinks(filePath)
		require.NoError(t, err)
		f, err := openFile(filePath, false)
		require.NoError(t, err)
		defer f.Close()
	})
	t.Run("symlinkFileOpenAllowed", func(t *testing.T) {
		f, err := openFile(symlinkToFile, true)
		require.NoError(t, err)
		defer f.Close()
	})
	t.Run("symlinkFileOpenDenied", func(t *testing.T) {
		_, err := openFile(symlinkToFile, false)
		require.Error(t, err)
	})
	t.Run("symlinkDirFileOpenDenied", func(t *testing.T) {
		_, err := openFile(filepath.Join(symlinkDir, "file"), false)
		require.Error(t, err)
	})
	t.Run("symlinkRecursiveDirFileOpenDenied", func(t *testing.T) {
		_, err := openFile(filepath.Join(symlinkToSymlinkDir, "file"), false)
		require.Error(t, err)
	})
	t.Run("circularSymlink", func(t *testing.T) {
		_, err := openFile(circularSymlink, false)
		require.Error(t, err)
	})
	t.Run("brokenSymlink", func(t *testing.T) {
		_, err := openFile(brokenSymlink, false)
		require.Error(t, err)
	})
}

func TestLocks(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "teleport-lock-test")
	fp := tmpFile.Name()
	t.Cleanup(func() {
		_ = os.Remove(fp)
	})
	require.NoError(t, err)

	// Can take read lock
	unlock, err := FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())

	// Can take write lock
	unlock, err = FSTryWriteLock(fp)
	require.NoError(t, err)

	// Can't take read lock while write lock is held.
	unlock2, err := FSTryReadLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	// Can't take write lock while another write lock is held.
	unlock2, err = FSTryWriteLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())

	unlock, err = FSTryReadLock(fp)
	require.NoError(t, err)

	// Can take second read lock on the same file.
	unlock2, err = FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())
	require.NoError(t, unlock2())

	// Can take read lock with timeout
	unlock, err = FSTryReadLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)
	require.NoError(t, unlock())

	// Can take write lock with timeout
	unlock, err = FSTryWriteLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)

	// Fails because timeout is exceeded, since file is already locked.
	unlock2, err = FSTryWriteLockTimeout(context.Background(), fp, time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	// Fails because context is expired while waiting for timeout.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	unlock2, err = FSTryWriteLockTimeout(ctx, fp, time.Hour*1000)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())
}

func TestOverwriteFile(t *testing.T) {
	have := []byte("Sensitive Information")
	fName := filepath.Join(t.TempDir(), "teleport-overwrite-file-test")

	require.NoError(t, os.WriteFile(fName, have, 0600))
	require.NoError(t, overwriteFile(fName))

	contents, err := os.ReadFile(fName)
	require.NoError(t, err)
	require.NotContains(t, contents, have, "File contents were not overwritten")
}

func TestRemoveSecure(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "teleport-remove-secure-test"))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, RemoveSecure(f.Name()))
}
