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

package safefile

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func newTempDir(t *testing.T) string {
	tempDir := t.TempDir()
	tempRoot, err := filepath.EvalSymlinks(tempDir)
	require.NoError(t, err)
	return tempRoot
}

func TestNoFollowOpens(t *testing.T) {
	t.Parallel()

	t.Run("successful on path with no symlinks", func(t *testing.T) {
		targetFile := filepath.Join(newTempDir(t), "myfile.txt")

		f, err := OpenFileNoFollow(targetFile, os.O_WRONLY|os.O_CREATE, 0o600)
		require.NoError(t, err)
		const fileData = "foo bar baz"
		_, err = f.WriteString(fileData)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		info, err := os.Stat(targetFile)
		require.NoError(t, err)
		require.Equal(t, int64(len(fileData)), info.Size())
		require.Equal(t, os.FileMode(0o600), info.Mode())

		f, err = OpenFileNoFollow(targetFile, os.O_RDONLY, 0)
		require.NoError(t, err)
		data, err := io.ReadAll(f)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		require.Equal(t, []byte(fileData), data)
	})

	t.Run("block symlink in parent dir", func(t *testing.T) {
		tempDir := newTempDir(t)
		targetFile := filepath.Join(tempDir, "foo.txt")
		require.NoError(t, os.WriteFile(targetFile, []byte("foo"), 0o644))
		link := filepath.Join(tempDir, "link")
		require.NoError(t, os.Symlink(tempDir, link))
		linkTarget := filepath.Join(link, "foo.txt")

		_, err := OpenFileNoFollow(linkTarget, os.O_WRONLY|os.O_CREATE, 0o644)
		require.ErrorIs(t, err, unix.ENOTDIR)
	})

	t.Run("block symlink at end of path", func(t *testing.T) {
		tempDir := newTempDir(t)
		targetFile := filepath.Join(tempDir, "foo.txt")
		require.NoError(t, os.WriteFile(targetFile, []byte("foo"), 0o644))
		link := filepath.Join(tempDir, "link")
		require.NoError(t, os.Symlink(targetFile, link))

		_, err := OpenFileNoFollow(link, os.O_WRONLY|os.O_CREATE, 0o644)
		require.ErrorIs(t, err, unix.ELOOP)
	})
}
