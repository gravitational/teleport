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

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		description string
		condition   string
		env         AccessRequestExpressionEnv
		match       bool
	}{
		{
			description: "does not match user 'level' trait",
			condition:   `contains_any(user.traits["level"], set("L1"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level": {"L2"},
				},
			},
			match: false,
		},
		{
			description: "matches one of user 'level' trait",
			condition:   `contains_any(user.traits["level"], set("L1", "L2"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level": {"L1"},
				},
			},
			match: true,
		},
		{
			description: "matches all user traits",
			condition: `
				contains_any(user.traits["level"], set("L1", "L2")) &&
				contains_any(user.traits["team"], set("Cloud")) &&
				contains_any(user.traits["location"], set("Seattle"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level":    {"L1"},
					"team":     {"Cloud"},
					"location": {"Seattle"},
				},
			},
			match: true,
		},
		{
			description: "matches some user traits",
			condition: `
				contains_any(user.traits["level"], set("L1", "L2")) &&
				contains_any(user.traits["team"], set("Cloud")) &&
				contains_any(user.traits["location"], set("Seattle"))`,
			env: AccessRequestExpressionEnv{
				UserTraits: map[string][]string{
					"level":    {"L1"},
					"team":     {"Tools"},
					"location": {"Seattle"},
				},
			},
			match: false,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			match, err := EvaluateCondition(test.condition, test.env)
			require.NoError(t, err)
			require.Equal(t, test.match, match)
		})
	}
}
