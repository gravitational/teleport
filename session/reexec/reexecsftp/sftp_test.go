// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package reexecsftp

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/session/sftputils"
)

func TestEnsureReqIsAllowed(t *testing.T) {
	t.Parallel()
	const filePath = "/foo/bar/baz.txt"
	passTests := []struct {
		name    string
		allowed *allowedOps
		req     *sftp.Request
	}{
		{
			name: "no restrictions",
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodGet,
			},
		},
		{
			name:    "read",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodGet,
			},
		},
		{
			name:    "write",
			allowed: &allowedOps{path: filePath, write: true},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodPut,
			},
		},
		{
			name:    "chmod",
			allowed: &allowedOps{path: filePath, write: true},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodSetStat,
			},
		},
		{
			name:    "stat in read mode",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodStat,
			},
		},
		{
			name:    "lstat in read mode",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodLstat,
			},
		},
		{
			name:    "stat in write mode",
			allowed: &allowedOps{path: filePath, write: true},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodStat,
			},
		},
		{
			name:    "lstat in write mode",
			allowed: &allowedOps{path: filePath, write: true},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodLstat,
			},
		},
	}
	for _, tc := range passTests {
		t.Run("allow "+tc.name, func(t *testing.T) {
			tc.req.Filepath = filePath
			if tc.allowed != nil {
				tc.allowed.path = filePath
			}
			handler := &sftpHandler{allowed: tc.allowed}
			require.NoError(t, handler.ensureReqIsAllowed(tc.req))
		})
	}

	const convolutedPath = "/foo/bar/../bar/baz.txt"
	failTests := []struct {
		name    string
		allowed *allowedOps
		req     *sftp.Request
	}{
		{
			name:    "uncleaned path",
			allowed: &allowedOps{path: convolutedPath},
			req: &sftp.Request{
				Filepath: convolutedPath,
				Method:   sftputils.MethodGet,
			},
		},
		{
			name:    "get in write mode",
			allowed: &allowedOps{path: filePath, write: true},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodGet,
			},
		},
		{
			name:    "write in read mode",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodPut,
			},
		},
		{
			name:    "chmod in read mode",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodSetStat,
			},
		},
		{
			name:    "unknown method",
			allowed: &allowedOps{path: filePath},
			req: &sftp.Request{
				Filepath: filePath,
				Method:   sftputils.MethodRename,
			},
		},
	}
	for _, tc := range failTests {
		t.Run("deny "+tc.name, func(t *testing.T) {
			handler := &sftpHandler{allowed: tc.allowed}
			require.Error(t, handler.ensureReqIsAllowed(tc.req))
		})
	}
}

func newTempDir(t *testing.T) string {
	tempDir := t.TempDir()
	tempRoot, err := filepath.EvalSymlinks(tempDir)
	require.NoError(t, err)
	return tempRoot
}

func TestNoFollowFileOperations(t *testing.T) {
	t.Parallel()

	t.Run("successful on path with no symlinks", func(t *testing.T) {
		targetFile := filepath.Join(newTempDir(t), "myfile.txt")

		f, err := openFileNoFollow(targetFile, os.O_WRONLY|os.O_CREATE, 0o600)
		require.NoError(t, err)
		const fileData = "foo bar baz"
		_, err = f.WriteString(fileData)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		info, err := os.Stat(targetFile)
		require.NoError(t, err)
		require.Equal(t, int64(len(fileData)), info.Size())
		require.Equal(t, os.FileMode(0o600), info.Mode())

		f, err = openFileNoFollow(targetFile, os.O_RDONLY, 0)
		require.NoError(t, err)
		data, err := io.ReadAll(f)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		require.Equal(t, []byte(fileData), data)

		updatedTime := info.ModTime().Add(time.Hour).Truncate(time.Second)
		err = setstatNoFollow(targetFile, sftp.FileAttrFlags{
			Size:        true,
			Permissions: true,
			Acmodtime:   true,
		}, &sftp.FileStat{
			Size:  uint64(len(fileData) / 2),
			Mode:  0o604,
			Atime: uint32(updatedTime.Unix()),
			Mtime: uint32(updatedTime.Unix()),
		})
		require.NoError(t, err)
		newInfo, err := os.Stat(targetFile)
		require.NoError(t, err)
		require.Equal(t, int64(len(fileData)/2), newInfo.Size())
		require.Equal(t, os.FileMode(0o604), newInfo.Mode())
		require.Equal(t, updatedTime, newInfo.ModTime())
	})

	t.Run("block symlink in parent dir", func(t *testing.T) {
		tempDir := newTempDir(t)
		targetFile := filepath.Join(tempDir, "foo.txt")
		require.NoError(t, os.WriteFile(targetFile, []byte("foo"), 0o644))
		link := filepath.Join(tempDir, "link")
		require.NoError(t, os.Symlink(tempDir, link))
		linkTarget := filepath.Join(link, "foo.txt")

		_, err := openFileNoFollow(linkTarget, os.O_WRONLY|os.O_CREATE, 0o644)
		require.ErrorIs(t, err, syscall.ENOTDIR)
		err = setstatNoFollow(linkTarget, sftp.FileAttrFlags{Permissions: true}, &sftp.FileStat{Mode: 0o600})
		require.ErrorIs(t, err, syscall.ENOTDIR)
	})

	t.Run("block symlink at end of path", func(t *testing.T) {
		tempDir := newTempDir(t)
		targetFile := filepath.Join(tempDir, "foo.txt")
		require.NoError(t, os.WriteFile(targetFile, []byte("foo"), 0o644))
		link := filepath.Join(tempDir, "link")
		require.NoError(t, os.Symlink(targetFile, link))

		_, err := openFileNoFollow(link, os.O_WRONLY|os.O_CREATE, 0o644)
		require.ErrorIs(t, err, syscall.ELOOP)
		err = setstatNoFollow(link, sftp.FileAttrFlags{Permissions: true}, &sftp.FileStat{Mode: 0o600})
		require.ErrorIs(t, err, syscall.ELOOP)
	})
}
