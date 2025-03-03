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

package bitbucket

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIDTokenSource_GetIDToken(t *testing.T) {
	t.Run("value present", func(t *testing.T) {
		its := NewIDTokenSource(func(key string) string {
			if key == "BITBUCKET_STEP_OIDC_TOKEN" {
				return "foo"
			}
			return ""
		})
		tok, err := its.GetIDToken()
		require.NoError(t, err)
		require.Equal(t, "foo", tok)
	})

	t.Run("value missing", func(t *testing.T) {
		its := NewIDTokenSource(func(key string) string { return "" })
		tok, err := its.GetIDToken()
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.Equal(t, "", tok)
	})
}
