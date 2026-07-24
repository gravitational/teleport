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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/session/sftputils"
)

func TestEnsureReqIsAllowed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "baz.txt")

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

	sep := string(filepath.Separator)
	// the path is built manually to avoid filepath.Join calling
	// filepath.Clean which would remove the relative part of the path
	convolutedPath := dir + sep + ".." + sep + filepath.Base(dir) + sep + "baz.txt"

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
		{
			name:    "dir filepath",
			allowed: &allowedOps{path: dir},
			req: &sftp.Request{
				Filepath: dir,
				Method:   sftputils.MethodGet,
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

func TestNoFollowSetstat(t *testing.T) {
	t.Parallel()

	t.Run("successful on path with no symlinks", func(t *testing.T) {
		targetFile := filepath.Join(newTempDir(t), "myfile.txt")

		fileData := []byte("foo bar baz")
		err := os.WriteFile(targetFile, []byte("foo bar baz"), 0o600)
		require.NoError(t, err)
		info, err := os.Stat(targetFile)
		require.NoError(t, err)

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

		err := setstatNoFollow(linkTarget, sftp.FileAttrFlags{Permissions: true}, &sftp.FileStat{Mode: 0o600})
		require.ErrorIs(t, err, unix.ENOTDIR)
	})

	t.Run("block symlink at end of path", func(t *testing.T) {
		tempDir := newTempDir(t)
		targetFile := filepath.Join(tempDir, "foo.txt")
		require.NoError(t, os.WriteFile(targetFile, []byte("foo"), 0o644))
		link := filepath.Join(tempDir, "link")
		require.NoError(t, os.Symlink(targetFile, link))

		err := setstatNoFollow(link, sftp.FileAttrFlags{Permissions: true}, &sftp.FileStat{Mode: 0o600})
		require.ErrorIs(t, err, unix.ELOOP)
	})
}
