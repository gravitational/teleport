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
			description: "match at least one role",
			condition: `
				contains_any(access_request.spec.roles, set("dev")) &&
				access_request.spec.roles.contains_any(set("dev"))`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: true,
		},
		{
			description: "does not match any role",
			condition: `
				contains_any(access_request.spec.roles, set("no-match")) ||
				access_request.spec.roles.contains_any(set("no-match"))`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: false,
		},
		{
			description: "matches all requested roles",
			condition: `
				contains_all(set("dev", "stage", "prod"), access_request.spec.roles) &&
				access_request.spec.roles.contains_all(set("dev", "stage", "prod"))`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
			},
			match: true,
		},
		{
			description: "does not match all requested roles",
			condition: `
				contains_all(set("dev"), access_request.spec.roles) ||
				set("dev").contains_all(access_request.spec.roles)`,
			env: AccessRequestExpressionEnv{
				Roles: []string{"dev", "stage", "prod"},
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
