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

package gitlab

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIDTokenSource_GetIDToken(t *testing.T) {
	makeConfig := func(
		t *testing.T,
		configuredEnvVar string,
		wantKey string,
		returnValue string,
	) IDTokenSourceConfig {
		return IDTokenSourceConfig{
			EnvVarName: configuredEnvVar,
			EnvGetter: func(gotKey string) string {
				if gotKey == wantKey {
					return returnValue
				}
				t.Errorf("Expected key %q, got %q", wantKey, gotKey)
				return ""
			},
		}
	}
	t.Run("value present", func(t *testing.T) {
		cfg := makeConfig(
			t,
			"",
			"TBOT_GITLAB_JWT",
			"foo",
		)
		its := NewIDTokenSource(cfg)
		tok, err := its.GetIDToken()
		require.NoError(t, err)
		require.Equal(t, "foo", tok)
	})

	t.Run("value missing", func(t *testing.T) {
		cfg := makeConfig(
			t,
			"",
			"TBOT_GITLAB_JWT",
			"",
		)
		its := NewIDTokenSource(cfg)
		tok, err := its.GetIDToken()
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.Empty(t, tok)
	})

	t.Run("overridden env value present", func(t *testing.T) {
		cfg := makeConfig(
			t,
			"OVERRIDDEN",
			"OVERRIDDEN",
			"foo",
		)
		its := NewIDTokenSource(cfg)
		tok, err := its.GetIDToken()
		require.NoError(t, err)
		require.Equal(t, "foo", tok)
	})
}
