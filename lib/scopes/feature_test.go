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

package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFeatureEnabled verifies the expected behavior of the scope feature flag.
func TestFeatureEnabled(t *testing.T) {
	require.Error(t, AssertFeatureEnabled())
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	require.NoError(t, AssertFeatureEnabled())
}

func TestMWIFeatureEnabled(t *testing.T) {
	cases := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "both flags unset",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name: "mwi flag unset",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_SCOPES": "yes",
			},
			expected: false,
		},
		{
			name: "main scopes flag unset",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_SCOPES_MWI": "yes",
			},
			expected: false,
		},
		{
			name: "both flags set",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_SCOPES":     "yes",
				"TELEPORT_UNSTABLE_SCOPES_MWI": "yes",
			}, expected: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range tc.envVars {
				t.Setenv(key, value)
			}
			require.Equal(t, tc.expected, MWIFeatureEnabled())
			if tc.expected {
				require.NoError(t, AssertMWIFeatureEnabled())
			} else {
				require.Error(t, AssertMWIFeatureEnabled())
			}
		})
	}
}
