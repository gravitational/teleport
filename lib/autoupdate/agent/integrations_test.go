/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasParentDir(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		parent     string
		wantResult bool
	}{
		{
			name:       "Has valid parent directory",
			path:       "/opt/teleport/dir/test",
			parent:     "/opt/teleport",
			wantResult: true,
		},
		{
			name:       "Has valid parent directory with slash",
			path:       "/opt/teleport/dir/test",
			parent:     "/opt/teleport/",
			wantResult: true,
		},
		{
			name:       "Parent directory is root",
			path:       "/opt/teleport/dir",
			parent:     "/",
			wantResult: true,
		},
		{
			name:       "Parent is the same as the path",
			path:       "/opt/teleport/dir",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent the same as the path but without slash",
			path:       "/opt/teleport/dir/",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent the same as the path but with slash",
			path:       "/opt/teleport/dir",
			parent:     "/opt/teleport/dir/",
			wantResult: false,
		},
		{
			name:       "Parent is substring of the path",
			path:       "/opt/teleport/dir-place",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent is in path",
			path:       "/opt/teleport",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Empty parent",
			path:       "/opt/teleport/dir",
			parent:     "",
			wantResult: false,
		},
		{
			name:       "Empty path",
			path:       "",
			parent:     "/opt/teleport",
			wantResult: false,
		},
		{
			name:       "Both empty",
			path:       "",
			parent:     "",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hasParentDir(tt.path, tt.parent)
			require.NoError(t, err)
			require.Equal(t, tt.wantResult, result)
		})
	}
}

func TestStablePath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		resultIsFile bool
		resultIsLink bool
		result       string
		err          error
	}{
		{
			name:         "packaged path is file",
			path:         "/opt/teleport/system/bin/teleport",
			resultIsFile: true,
			result:       "/usr/local/bin/teleport",
		},
		{
			name:         "packaged path is link",
			path:         "/opt/teleport/system/bin/teleport",
			resultIsFile: true,
			resultIsLink: true,
			result:       "/usr/local/bin/teleport",
		},
		{
			name:         "packaged path is broken link",
			path:         "/opt/teleport/system/bin/teleport",
			resultIsLink: true,
			result:       "/opt/teleport/system/bin/teleport",
			err:          ErrUnstableExecutable,
		},
		{
			name:   "packaged path is missing",
			path:   "/opt/teleport/system/bin/teleport",
			result: "/opt/teleport/system/bin/teleport",
			err:    ErrUnstableExecutable,
		},
		{
			name:         "managed path is file",
			path:         "versions/version/bin/teleport",
			resultIsFile: true,
			result:       "[ns]/bin/teleport",
		},
		{
			name:         "managed path is link",
			path:         "versions/version/bin/teleport",
			resultIsFile: true,
			resultIsLink: true,
			result:       "[ns]/bin/teleport",
		},
		{
			name:         "managed path is broken link",
			path:         "versions/version/bin/teleport",
			resultIsLink: true,
			result:       "[ns]/versions/version/bin/teleport",
			err:          ErrUnstableExecutable,
		},
		{
			name:   "managed path is missing",
			path:   "versions/version/bin/teleport",
			result: "[ns]/versions/version/bin/teleport",
			err:    ErrUnstableExecutable,
		},
		{
			name:   "managed path is missing config",
			path:   "/test/versions/version/bin/teleport",
			result: "/test/versions/version/bin/teleport",
		},
		{
			name: "empty",
		},
		{
			name:   "other",
			path:   "/other",
			result: "/other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsDir := t.TempDir()
			defaultPath := t.TempDir()
			createPath := defaultPath

			if tt.path != "" && !filepath.IsAbs(tt.path) {
				tt.path = filepath.Join(nsDir, tt.path)
				createPath = filepath.Join(nsDir, "bin")
				cfgPath := filepath.Join(nsDir, updateConfigName)
				err := writeConfig(cfgPath, &UpdateConfig{
					Version: updateConfigVersion,
					Kind:    updateConfigKind,
					Spec: UpdateSpec{
						Path: createPath,
					},
				})
				require.NoError(t, err)
			}

			_, name := filepath.Split(tt.path)
			if tt.resultIsLink {
				filePath := filepath.Join(t.TempDir(), name)
				if tt.resultIsFile {
					createEmptyFile(t, filePath)
				}
				createSymlink(t, filePath, filepath.Join(createPath, name))
			} else if tt.resultIsFile {
				createEmptyFile(t, filepath.Join(createPath, name))
			}

			result, err := stablePathForBinary(tt.path, defaultPath)
			require.Equal(t, tt.result, strings.NewReplacer(
				defaultPath, "/usr/local/bin",
				nsDir, "[ns]",
			).Replace(result))
			require.Equal(t, tt.err, err)
		})
	}
}

func createEmptyFile(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	require.NoError(t, err)
	f, err := os.Create(path)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
}

func createSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(newname), os.ModePerm)
	require.NoError(t, err)
	err = os.Symlink(oldname, newname)
	require.NoError(t, err)
}
