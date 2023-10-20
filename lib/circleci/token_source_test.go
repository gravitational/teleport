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

package circleci

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGetIDToken(t *testing.T) {
	t.Parallel()
	fakeGetEnv := func(env string) getEnvFunc {
		return func(key string) string {
			if key == "CIRCLE_OIDC_TOKEN" {
				return env
			}
			return ""
		}
	}

	tests := []struct {
		name        string
		getEnv      getEnvFunc
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "success",
			getEnv:      fakeGetEnv("foobarbizz"),
			wantString:  "foobarbizz",
			assertError: require.NoError,
		},
		{
			name:   "unset",
			getEnv: fakeGetEnv(""),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			str, err := GetIDToken(tt.getEnv)
			require.Equal(t, tt.wantString, str)
			tt.assertError(t, err)
		})
	}
}
