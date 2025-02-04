/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsConditionMatched(t *testing.T) {
	tests := []struct {
		description string
		condition   string
		env         AccessRequestExpressionEnv
		match       bool
	}{
		{
			description: "match plugin name",
			condition:   `plugin.spec.name == "teleport-plugin"`,
			env: AccessRequestExpressionEnv{
				Plugin: PluginExpressionEnv{
					Name: "teleport-plugin",
				},
			},
			match: true,
		},
		{
			description: "mismatch plugin name",
			condition:   `plugin.spec.name == "teleport-plugin"`,
			env: AccessRequestExpressionEnv{
				Plugin: PluginExpressionEnv{
					Name: "teleport-plugin-mismatch",
				},
			},
			match: false,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			match, err := IsConditionMatched(test.condition, test.env)
			require.NoError(t, err)
			require.Equal(t, test.match, match)
		})
	}
}
