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
