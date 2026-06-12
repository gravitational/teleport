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

package sftp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealpath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "a")
	require.NoError(t, os.Mkdir(dir, 0o755))
	file := filepath.Join(dir, "foo.txt")
	require.NoError(t, os.WriteFile(file, []byte("foo"), 0o644))
	symlink := filepath.Join(dir, "sym")
	require.NoError(t, os.Symlink(root, symlink))

	wd, err := os.Getwd()
	require.NoError(t, err)
	fileRelativeToHere, err := filepath.Rel(wd, file)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     file,
			expected: file,
		},
		{
			name:     "relative path",
			path:     fileRelativeToHere,
			expected: file,
		},
		{
			name:     "follow symlink",
			path:     filepath.Join(symlink, "a", "foo.txt"),
			expected: file,
		},
		{
			name:     "current dir",
			path:     ".",
			expected: wd,
		},
		{
			name:     "empty",
			path:     "",
			expected: wd,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// On Mac, temp dirs are under an extra hidden /private dir.
			expected, err := filepath.EvalSymlinks(tc.expected)
			require.NoError(t, err)
			out, err := Realpath(tc.path)
			assert.NoError(t, err)
			assert.Equal(t, expected, out)
		})
	}
}
