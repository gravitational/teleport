// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const (
	testDirname              = "accessible_root"
	testInaccessibleFileName = "test_inaccessible_file"
	testInaccessibleDirName  = "test_inaccessible_dir"

	testSymlinkInaccessibleFileName = "test_inaccessible_symlink_file"
	testSymlinkInaccessibleDirName  = "test_inaccessible_symlink_dir"
	testSymlinkAccessibleFile       = "test_symlink_accessible_file"

	testAccessibleFileName = "test_accessible_file"
	testAccessibleDirName  = "test_accessible_dir"
)

func TestNewDirectoryAccess(t *testing.T) {
	path := t.TempDir()
	filePath := filepath.Join(path, testInaccessibleFileName)
	err := os.WriteFile(filePath, []byte("test"), 0600)
	require.NoError(t, err)
	_, err = NewDirectoryAccess(filePath)
	require.True(t, trace.IsBadParameter(err), "%q is not a directory", filePath)
}

func setUpSharedDir(t *testing.T) (*DirectoryAccess, string) {
	t.Helper()
	testRoot := t.TempDir()
	// Create a test folder like so:
	//    <random_tmp_dir_name>
	//    |-- test_inaccessible_dir/
	//    |-- test_inaccessible_file
	//    |-- accessible_root/ <- DirectoryAccess is rooted here
	//    |   |-- test_inaccessible_symlink_dir/ -> ../test_inaccessible_dir/
	//    |   |-- test_inaccessible_symlink_file -> ../test_inaccessible_file
	//    |   |-- test_accessible_file
	//    |   |-- test_accessible_dir/
	//    |   |-- test_symlink_accessible_file -> ./accessible_file
	//
	// The access folder contains a single regular file and symlinks
	// to a file and directory that should not be traversible, as well
	// as a symlink to the regular file which *should be* accessible.
	accessRoot := filepath.Join(testRoot, testDirname)
	err := os.Mkdir(accessRoot, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(accessRoot, testAccessibleFileName), []byte("test"), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testRoot, testInaccessibleFileName), []byte("data"), 0600)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(accessRoot, testAccessibleDirName), 0700)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(testRoot, testInaccessibleDirName), 0700)
	require.NoError(t, err)
	err = os.Symlink(filepath.Join(testRoot, testInaccessibleFileName), filepath.Join(accessRoot, testSymlinkInaccessibleFileName))
	require.NoError(t, err)
	err = os.Symlink(filepath.Join(accessRoot, testInaccessibleDirName), filepath.Join(accessRoot, testSymlinkInaccessibleDirName))
	require.NoError(t, err)
	// Symlink must use a relative path that remains within the shared directory.
	err = os.Symlink(testAccessibleFileName, filepath.Join(accessRoot, testSymlinkAccessibleFile))
	require.NoError(t, err)
	access, err := NewDirectoryAccess(accessRoot)
	require.NoError(t, err)
	return access, accessRoot
}

func TestDirectoryAccessEscapingPaths(t *testing.T) {
	operations := map[string]func(a *DirectoryAccess, path string) error{
		"Stat":    func(a *DirectoryAccess, path string) error { _, err := a.Stat(path); return err },
		"ReadDir": func(a *DirectoryAccess, path string) error { _, err := a.ReadDir(path); return err },
		"Read": func(a *DirectoryAccess, path string) error {
			buf := make([]byte, 10)
			_, err := a.Read(path, 0, buf)
			return err
		},
		"Write":    func(a *DirectoryAccess, path string) error { _, err := a.Write(path, 0, []byte("test")); return err },
		"Truncate": func(a *DirectoryAccess, path string) error { err := a.Truncate(path, 100); return err },
		"Create":   func(a *DirectoryAccess, path string) error { err := a.Create(path, FileTypeDir); return err },
		"Delete":   func(a *DirectoryAccess, path string) error { err := a.Delete(path); return err },
	}

	// Try escape paths on all operations
	paths := []string{"..", "../../"}
	for opName, op := range operations {
		for _, path := range paths {
			t.Run(fmt.Sprintf("%s_Escape/%s", opName, path), func(t *testing.T) {
				access, _ := setUpSharedDir(t)
				err := op(access, path)
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			})
		}
	}

	// Try operations (except create/delete) on symlink to a file outside of the root.
	symlinkOperations := []string{"Stat", "Read", "ReadDir", "Write", "Truncate"}
	for _, symlinkOp := range symlinkOperations {
		t.Run(fmt.Sprintf("symlink_traversal/%s", symlinkOp), func(t *testing.T) {
			access, path := setUpSharedDir(t)
			fmt.Println(path)
			err := operations[symlinkOp](access, testSymlinkInaccessibleFileName)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	}

	// Try create/delete operations following a symlink to a directory outside
	// of the root.
	symlinkDirOperations := []string{"Create", "Delete"}
	for _, symlinkOp := range symlinkDirOperations {
		t.Run(fmt.Sprintf("symlink_dir_traversal/%s", symlinkOp), func(t *testing.T) {
			access, _ := setUpSharedDir(t)
			err := operations[symlinkOp](access, filepath.Join(testSymlinkInaccessibleDirName, "somefile"))
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	}
}

func TestDirectoryAccessSuccessOperations(t *testing.T) {
	t.Run("Stat", func(t *testing.T) {
		access, path := setUpSharedDir(t)
		info, err := access.Stat("")
		require.NoError(t, err)

		osStat, err := os.Stat(path)
		require.NoError(t, err)

		require.Equal(t, &FileOrDirInfo{
			Size:         4096,
			LastModified: osStat.ModTime().UnixMilli(),
			FileType:     FileTypeDir,
			IsEmpty:      false,
			Path:         "",
		}, info)
	})

	t.Run("ReadDir", func(t *testing.T) {
		access, path := setUpSharedDir(t)
		dir, err := access.ReadDir("")
		require.NoError(t, err)

		// Although there are 4 entries in the shared directory, our 'ReadDir'
		// method only shows 2 because it skips symlinks.
		require.Len(t, dir, 2)
		osStat, err := os.Stat(filepath.Join(path, testAccessibleDirName))
		// Find the directory entry
		require.NoError(t, err)
		require.Contains(t, dir, &FileOrDirInfo{
			Size:         4096,
			LastModified: osStat.ModTime().UnixMilli(),
			FileType:     FileTypeDir,
			IsEmpty:      true,
			Path:         testAccessibleDirName,
		})
		osStat, err = os.Stat(filepath.Join(path, testAccessibleFileName))
		require.NoError(t, err)
		require.Contains(t, dir, &FileOrDirInfo{
			Size:         osStat.Size(),
			LastModified: osStat.ModTime().UnixMilli(),
			FileType:     FileTypeFile,
			IsEmpty:      false,
			Path:         testAccessibleFileName,
		})
	})

	t.Run("Read", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		buf := make([]byte, 4)
		read, err := access.Read(testAccessibleFileName, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 4, read)
		require.Equal(t, []byte("test"), buf)
	})

	// Although we hide symlinks during ReadDir operations,
	// a tech savvy user could still attempt I/O operations against
	// them. This "test" is more of a demonstration that relative
	// symlinks that stay within the bounds of the root are permitted.
	t.Run("Read Accessible Symlink", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		buf := make([]byte, 4)
		read, err := access.Read(testSymlinkAccessibleFile, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 4, read)
		require.Equal(t, []byte("test"), buf)
	})

	t.Run("Write", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		written, err := access.Write(testAccessibleFileName, 4, []byte("_new_content"))
		require.NoError(t, err)
		require.Equal(t, 12, written)

		buf := make([]byte, 16)
		read, err := access.Read(testAccessibleFileName, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 16, read)
		require.Equal(t, []byte("test_new_content"), buf)
	})

	t.Run("Write Accessible Symlink", func(t *testing.T) {
		access, path := setUpSharedDir(t)
		fmt.Println(path)
		written, err := access.Write(testSymlinkAccessibleFile, 4, []byte("_symlink"))
		require.NoError(t, err)
		require.Equal(t, 8, written)

		buf := make([]byte, 12)
		read, err := access.Read(testSymlinkAccessibleFile, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 12, read)
		require.Equal(t, []byte("test_symlink"), buf)
	})

	t.Run("Truncate", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		err := access.Truncate(testAccessibleFileName, 100)
		require.NoError(t, err)
		stat, err := access.Stat(testAccessibleFileName)
		require.NoError(t, err)
		require.Equal(t, int64(100), stat.Size)
	})

	t.Run("Create", func(t *testing.T) {
		access, _ := setUpSharedDir(t)

		err := access.Create("new_file", FileTypeFile)
		require.NoError(t, err)
		createdFile, err := access.Stat("new_file")
		require.NoError(t, err)
		require.Equal(t, FileTypeFile, createdFile.FileType)

		err = access.Create("new_dir", FileTypeDir)
		require.NoError(t, err)
		createdDir, err := access.Stat("new_dir")
		require.NoError(t, err)
		require.Equal(t, FileTypeDir, createdDir.FileType)
	})

	t.Run("Delete", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		err := access.Delete(testInaccessibleFileName)
		require.NoError(t, err)
		require.NoFileExists(t, testInaccessibleFileName)
	})
}
