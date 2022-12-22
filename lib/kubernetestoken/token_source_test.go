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

package kubernetestoken

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
