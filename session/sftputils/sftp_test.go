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

package sftputils

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
)

func TestHomeDirExpansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		expandedPath string
		errCheck     require.ErrorAssertionFunc
	}{
		{
			name:         "absolute path",
			path:         "/foo/bar",
			expandedPath: "/foo/bar",
		},
		{
			name:         "path with tilde-slash",
			path:         "~/foo/bar",
			expandedPath: "foo/bar",
		},
		{
			name:         "just tilde",
			path:         "~",
			expandedPath: ".",
		},
		{
			name:         "tilde slash",
			path:         "~/",
			expandedPath: ".",
		},
		{
			name: "~user path",
			path: "~user/foo",
			errCheck: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, PathExpansionError{path: "~user/foo"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, err := ExpandHomeDir(tt.path)
			if tt.errCheck == nil {
				require.NoError(t, err)
				require.Equal(t, tt.expandedPath, expanded)
			} else {
				tt.errCheck(t, err)
			}
		})
	}
}

type mockFile struct {
	File
	altDataSource io.Reader
}

func (m *mockFile) Read(p []byte) (int, error) {
	return m.altDataSource.Read(p)
}

type mockFS struct {
	localFS
	fileAccesses map[string]int
	altData      io.Reader
}

func (m *mockFS) Open(path string) (File, error) {
	if m.fileAccesses == nil {
		m.fileAccesses = make(map[string]int)
	}
	realPath, err := m.localFS.RealPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.fileAccesses[realPath]++
	file, err := m.localFS.Open(path)
	if err != nil || m.altData == nil {
		return file, err
	}
	return &mockFile{
		File:          file,
		altDataSource: m.altData,
	}, nil
}

type mockCmdHandlers struct {
	sftp.Handlers
}

func (m mockCmdHandlers) Filecmd(req *sftp.Request) error {
	return trace.Wrap(HandleFilecmd(req, localFS{}))
}

func TestHandleFilecmd(t *testing.T) {
	t.Parallel()
	// We're using a full client/server instead of just calling HandleFilecmd so
	// the sftp package can handle marshaling attributes.
	clientConn, serverConn := net.Pipe()
	srv := sftp.NewRequestServer(serverConn, sftp.Handlers{
		FileGet:  sftp.InMemHandler().FileGet,
		FilePut:  sftp.InMemHandler().FilePut,
		FileCmd:  mockCmdHandlers{},
		FileList: sftp.InMemHandler().FileList,
	})

	t.Cleanup(func() { require.NoError(t, srv.Close()) })
	go srv.Serve()

	clt, err := sftp.NewClientPipe(clientConn, clientConn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	t.Run("chtimes", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		originalInfo, err := os.Stat(file)
		require.NoError(t, err)
		setTime := originalInfo.ModTime().Add(time.Hour).Round(time.Second)

		assert.NoError(t, clt.Chtimes(file, setTime, setTime))
		updatedInfo, err := os.Stat(file)
		if assert.NoError(t, err) {
			assert.Equal(t, setTime, updatedInfo.ModTime())
		}
	})

	t.Run("chmod", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		assert.NoError(t, clt.Chmod(file, 0o666))
		fi, err := os.Stat(file)
		if assert.NoError(t, err) {
			assert.Equal(t, fs.FileMode(0o666), fi.Mode().Perm())
		}
	})

	t.Run("truncate", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte(strings.Repeat("a", 100)), 0o644))

		assert.NoError(t, clt.Truncate(file, 50))
		data, err := os.ReadFile(file)
		if assert.NoError(t, err) {
			assert.Len(t, data, 50)
		}
	})

	t.Run("rename", func(t *testing.T) {
		root := t.TempDir()
		initialFile := filepath.Join(root, "foo.txt")
		finalFile := filepath.Join(root, "bar.txt")
		require.NoError(t, os.WriteFile(initialFile, []byte("test"), 0o644))

		assert.NoError(t, clt.Rename(initialFile, finalFile))
		assert.NoFileExists(t, initialFile)
		assert.FileExists(t, finalFile)
	})

	t.Run("rename missing target", func(t *testing.T) {
		root := t.TempDir()
		initialFile := filepath.Join(root, "foo.txt")
		finalFile := filepath.Join(root, "bar.txt")
		assert.Error(t, clt.Rename(initialFile, finalFile))
		assert.NoFileExists(t, finalFile)
	})

	t.Run("rmdir", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "foo")
		innerFile := filepath.Join(dir, "test.txt")
		require.NoError(t, os.Mkdir(dir, defaults.DirectoryPermissions))
		require.NoError(t, os.WriteFile(innerFile, []byte("test"), 0o644))

		assert.NoError(t, clt.RemoveDirectory(dir))
		assert.NoDirExists(t, dir)
	})

	t.Run("rmdir not found", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "foo")
		assert.Error(t, clt.RemoveDirectory(dir))
	})

	t.Run("rmdir not a dir", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		assert.Error(t, clt.RemoveDirectory(file))
		assert.FileExists(t, file)
	})

	t.Run("mkdir", func(t *testing.T) {
		root := t.TempDir()
		outer := filepath.Join(root, "a")
		inner := filepath.Join(outer, "b/c")
		require.NoError(t, os.Mkdir(outer, defaults.DirectoryPermissions))

		assert.NoError(t, clt.Mkdir(inner))
		assert.DirExists(t, inner)
	})

	t.Run("link", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "foo.txt")
		target := filepath.Join(root, "bar.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		assert.NoError(t, clt.Link(target, file))
		fi, err := os.Lstat(target)
		if assert.NoError(t, err) {
			assert.Zero(t, fi.Mode()&os.ModeSymlink)
		}
	})

	t.Run("link missing target", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "foo.txt")
		target := filepath.Join(root, "bar.txt")

		assert.Error(t, clt.Link(target, file))
		assert.NoFileExists(t, target)
	})

	t.Run("link unset target", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "foo.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		assert.Error(t, clt.Link(file, ""))
	})

	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "foo.txt")
		target := filepath.Join(root, "bar.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		assert.NoError(t, clt.Symlink(target, file))
		fi, err := os.Lstat(target)
		assert.NoError(t, err)
		assert.NotZero(t, fi.Mode()&os.ModeSymlink)
	})

	t.Run("symlink unset target", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "foo.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		assert.Error(t, clt.Symlink(file, ""))
	})

	t.Run("remove", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		assert.NoError(t, clt.Remove(file))
		assert.NoFileExists(t, file)
	})

	t.Run("remove not found", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")

		assert.Error(t, clt.Remove(file))
	})

	t.Run("remove directory", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "dir")
		require.NoError(t, os.Mkdir(dir, defaults.DirectoryPermissions))

		assert.NoError(t, clt.Remove(dir))
		assert.NoDirExists(t, dir)
	})

	t.Run("unsupported operation", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("foo"), 0o644))
		req := sftp.NewRequest(MethodStat, file)
		assert.Error(t, HandleFilecmd(req, localFS{}))
	})
}

type fileInfo struct {
	name string
	mode fs.FileMode
	size int64
}

func (fi fileInfo) Name() string {
	return fi.name
}

func (fi fileInfo) Size() int64 {
	return fi.size
}

func (fi fileInfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi fileInfo) ModTime() time.Time {
	return time.Time{}
}

func (fi fileInfo) IsDir() bool {
	return false
}

func (fi fileInfo) Sys() any {
	return nil
}

func TestHandleFilelist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	statMap := make(map[string]fs.FileInfo, 10)
	for i := range 5 {
		fileName := fmt.Sprintf("file-%d", i)
		file := filepath.Join(root, fileName)
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		statMap[fileName] = fileInfo{
			name: fileName,
			mode: 0o644,
			size: 4,
		}
		symlinkName := fmt.Sprintf("file-%d", i+5)
		symlink := filepath.Join(root, symlinkName)
		require.NoError(t, os.Symlink(file, symlink))
		statMap[symlinkName] = fileInfo{
			name: symlinkName,
			mode: 0o644,
			size: 4,
		}
	}

	// Add a broken symlink.
	brokenSymlinkName := "broken-symlink"
	brokenSymlink := filepath.Join(root, brokenSymlinkName)
	brokenTarget := filepath.Join(root, "this-file-does-not-exist")
	require.NoError(t, os.Symlink(brokenTarget, brokenSymlink))
	symlinkStat, err := os.Lstat(brokenSymlink)
	require.NoError(t, err)
	statMap[brokenSymlinkName] = fileInfo{
		name: brokenSymlinkName,
		mode: symlinkStat.Mode(),
		size: int64(len(brokenTarget)),
	}

	tests := []struct {
		name           string
		req            *sftp.Request
		assert         assert.ErrorAssertionFunc
		expectedOutput map[string]fs.FileInfo
	}{
		{
			name:           "list",
			req:            sftp.NewRequest(MethodList, root),
			assert:         assert.NoError,
			expectedOutput: statMap,
		},
		{
			name:   "stat",
			req:    sftp.NewRequest(MethodStat, root+"/file-0"),
			assert: assert.NoError,
			expectedOutput: map[string]fs.FileInfo{
				"file-0": fileInfo{
					name: "file-0",
					mode: 0o644,
					size: 4,
				},
			},
		},
		{
			name:   "readlink",
			req:    sftp.NewRequest(MethodReadlink, root+"/file-5"),
			assert: assert.NoError,
			expectedOutput: map[string]fs.FileInfo{
				root + "/file-0": fileName(root + "/file-0"),
			},
		},
		{
			name:   "unsupported operation",
			req:    sftp.NewRequest(MethodRemove, root),
			assert: assert.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lister, err := HandleFilelist(tc.req, localFS{})
			tc.assert(t, err)
			if tc.expectedOutput == nil {
				assert.Nil(t, lister)
				return
			}
			assert.NotNil(t, lister)

			list := make([]fs.FileInfo, len(tc.expectedOutput))
			n, err := lister.ListAt(list, 0)
			assert.NoError(t, err)
			assert.Equal(t, len(tc.expectedOutput), n)
			for _, fi := range list {
				entry, ok := tc.expectedOutput[fi.Name()]
				if assert.True(t, ok, "unexpected file %q", fi.Name()) {
					assert.Equal(t, entry.Name(), fi.Name())
					assert.Equal(t, entry.Size(), fi.Size(), fi.Name())
					assert.Equal(t, entry.Mode(), fi.Mode(), "%s: expected mode 0o%o, got mode 0o%o", fi.Name(), entry.Mode(), fi.Mode())
				}
			}
		})
	}
}
