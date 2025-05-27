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
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const (
	testDirname         = "test_dir"
	testFilename        = "test_file"
	testSymlinkFilename = "test_symlink"
)

func TestNewDirectoryAccess(t *testing.T) {
	path := t.TempDir()
	filePath := filepath.Join(path, testFilename)
	err := os.WriteFile(filePath, []byte("test"), 0600)
	require.NoError(t, err)
	_, err = NewDirectoryAccess(filePath)
	require.True(t, trace.IsBadParameter(err), "%q is not a directory", filePath)
}

func setUpSharedDir(t *testing.T) (*DirectoryAccess, string) {
	t.Helper()
	path := t.TempDir()
	err := os.Mkdir(filepath.Join(path, testDirname), 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(path, testFilename), []byte("test"), 0600)
	require.NoError(t, err)
	err = os.Symlink(filepath.Join(path, testFilename), filepath.Join(path, testSymlinkFilename))
	require.NoError(t, err)
	access, err := NewDirectoryAccess(path)
	require.NoError(t, err)
	return access, path
}

func TestDirectoryAccessEscapingPaths(t *testing.T) {
	outOfRootPath := filepath.Join(testDirname, "../..")
	tests := []struct {
		name string
		call func(*DirectoryAccess) error
	}{
		{"Stat", func(a *DirectoryAccess) error { _, err := a.Stat(outOfRootPath); return err }},
		{"ReadDir", func(a *DirectoryAccess) error { _, err := a.ReadDir(outOfRootPath); return err }},
		{"Read", func(a *DirectoryAccess) error {
			buf := make([]byte, 10)
			_, err := a.Read(outOfRootPath, 0, buf)
			return err
		}},
		{"Write", func(a *DirectoryAccess) error { _, err := a.Write(outOfRootPath, 0, []byte("test")); return err }},
		{"Truncate", func(a *DirectoryAccess) error { err := a.Truncate(outOfRootPath, 100); return err }},
		{"Create", func(a *DirectoryAccess) error { err := a.Create(outOfRootPath, FileTypeDir); return err }},
		{"Delete", func(a *DirectoryAccess) error { err := a.Delete(outOfRootPath); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Escape", func(t *testing.T) {
			access, _ := setUpSharedDir(t)
			err := tt.call(access)
			require.ErrorContains(t, err, "path escapes from parent")
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
			LastModified: osStat.ModTime().Unix(),
			FileType:     FileTypeDir,
			IsEmpty:      false,
			Path:         "",
		}, info)
	})

	t.Run("ReadDir", func(t *testing.T) {
		access, path := setUpSharedDir(t)
		dir, err := access.ReadDir("")
		require.NoError(t, err)

		require.Len(t, dir, 2)
		osStat, err := os.Stat(filepath.Join(path, testDirname))
		require.NoError(t, err)
		require.Contains(t, dir, &FileOrDirInfo{
			Size:         4096,
			LastModified: osStat.ModTime().Unix(),
			FileType:     FileTypeDir,
			IsEmpty:      true,
			Path:         testDirname,
		})
		osStat, err = os.Stat(filepath.Join(path, testFilename))
		require.NoError(t, err)
		require.Contains(t, dir, &FileOrDirInfo{
			Size:         osStat.Size(),
			LastModified: osStat.ModTime().Unix(),
			FileType:     FileTypeFile,
			IsEmpty:      false,
			Path:         testFilename,
		})
	})

	t.Run("Read", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		buf := make([]byte, 4)
		read, err := access.Read(testFilename, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 4, read)
		require.Equal(t, []byte("test"), buf)
	})

	t.Run("Write", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		written, err := access.Write(testFilename, 4, []byte("_new_content"))
		require.NoError(t, err)
		require.Equal(t, 12, written)

		buf := make([]byte, 16)
		read, err := access.Read(testFilename, 0, buf)
		require.NoError(t, err)
		require.Equal(t, 16, read)
		require.Equal(t, []byte("test_new_content"), buf)
	})

	t.Run("Truncate", func(t *testing.T) {
		access, _ := setUpSharedDir(t)
		err := access.Truncate(testFilename, 100)
		require.NoError(t, err)
		stat, err := access.Stat(testFilename)
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
		err := access.Delete(testFilename)
		require.NoError(t, err)
		require.NoFileExists(t, testFilename)
	})
}
