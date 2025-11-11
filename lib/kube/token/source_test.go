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

package token

import (
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetIDToken(t *testing.T) {
	t.Parallel()
	fakeGetEnv := func(env string) getEnvFunc {
		return func(key string) string {
			if key == "KUBERNETES_TOKEN_PATH" {
				return env
			}
			return ""
		}
	}

	errNoFile := func(path string) error {
		return &fs.PathError{
			Op:   "open",
			Path: path,
			Err:  syscall.ENOENT,
		}
	}

	fakeReadFile := func(content, contentPath string) readFileFunc {
		return func(path string) ([]byte, error) {
			if path == contentPath {
				return []byte(content), nil
			}
			return []byte{}, errNoFile(path)
		}
	}

	tests := []struct {
		name        string
		getEnv      getEnvFunc
		readFile    readFileFunc
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "default-token-no-var",
			getEnv:      fakeGetEnv(""),
			readFile:    fakeReadFile("foobarbizz", kubernetesDefaultTokenPath),
			wantString:  "foobarbizz",
			assertError: require.NoError,
		},
		{
			name:        "custom-token-with-var",
			getEnv:      fakeGetEnv("/custom"),
			readFile:    fakeReadFile("foobarbizz", "/custom"),
			wantString:  "foobarbizz",
			assertError: require.NoError,
		},
		{
			name:     "no-token-no-var",
			getEnv:   fakeGetEnv(""),
			readFile: fakeReadFile("foobarbizz", "/custom"),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, kubernetesDefaultTokenPath+": no such file")
			},
		},
		{
			name:     "no-token-with-var",
			getEnv:   fakeGetEnv("/custom"),
			readFile: fakeReadFile("foobarbizz", kubernetesDefaultTokenPath),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "/custom: no such file")
			},
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			str, err := GetIDToken(tt.getEnv, tt.readFile)
			require.Equal(t, tt.wantString, str)
			tt.assertError(t, err)
		})
	}
}
