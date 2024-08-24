/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package terraformcloud

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIDTokenSource_GetIDToken(t *testing.T) {
	t.Parallel()

	t.Run("tagged", func(t *testing.T) {
		getEnv := func(key string) string {
			if key == "TFC_WORKLOAD_IDENTITY_TOKEN_FOO" {
				return "bar"
			}

			return ""
		}

		t.Run("value present", func(t *testing.T) {
			its := &IDTokenSource{
				getEnv:      getEnv,
				audienceTag: "foo",
			}
			tok, err := its.GetIDToken()
			require.NoError(t, err)
			require.Equal(t, "bar", tok)
		})

		t.Run("value missing", func(t *testing.T) {
			its := &IDTokenSource{
				getEnv:      getEnv,
				audienceTag: "baz",
			}

			tok, err := its.GetIDToken()
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err))
			require.Equal(t, "", tok)
		})
	})

	t.Run("untagged", func(t *testing.T) {
		t.Run("value present", func(t *testing.T) {
			its := &IDTokenSource{
				getEnv: func(key string) string {
					if key == "TFC_WORKLOAD_IDENTITY_TOKEN" {
						return "bar"
					}

					return ""
				},
			}
			tok, err := its.GetIDToken()
			require.NoError(t, err)
			require.Equal(t, "bar", tok)
		})

		t.Run("value missing", func(t *testing.T) {
			its := &IDTokenSource{
				getEnv: func(key string) string {
					return ""
				},
			}

			tok, err := its.GetIDToken()
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err))
			require.Equal(t, "", tok)
		})

	})

}
