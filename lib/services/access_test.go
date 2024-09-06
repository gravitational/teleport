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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestCheckDynamicLabelsInDenyRules(t *testing.T) {
	t.Parallel()
	newRole := func(t *testing.T, spec types.RoleSpecV6) types.Role {
		role, err := types.NewRole("test-role", spec)
		require.NoError(t, err)
		return role
	}

	tests := []struct {
		name   string
		role   types.Role
		assert require.ErrorAssertionFunc
	}{
		{
			name: "ok role",
			role: newRole(t, types.RoleSpecV6{
				Deny: types.RoleConditions{
					NodeLabels: types.Labels{
						"a": {"1"},
						"b": {"2"},
						"c": {"3"},
					},
				},
			}),
			assert: require.NoError,
		},
		{
			name: "bad labels",
			role: newRole(t, types.RoleSpecV6{
				Deny: types.RoleConditions{
					NodeLabels: types.Labels{
						"a":         {"1"},
						"dynamic/b": {"2"},
						"c":         {"3"},
					},
				},
			}),
			assert: require.Error,
		},
		{
			name: "bad labels in where clause",
			role: newRole(t, types.RoleSpecV6{
				Deny: types.RoleConditions{
					NodeLabels: types.Labels{
						"a": {"1"},
						"b": {"2"},
						"c": {"3"},
					},
					ReviewRequests: &types.AccessReviewConditions{
						Where: `contains(user.spec.traits["allow-env"], labels["dynamic/env"])`,
					},
				},
			}),
			assert: require.Error,
		},
		{
			name: "bad labels in label expression",
			role: newRole(t, types.RoleSpecV6{
				Deny: types.RoleConditions{
					NodeLabels: types.Labels{
						"a": {"1"},
						"b": {"2"},
						"c": {"3"},
					},
					NodeLabelsExpression: `contains(user.spec.traits["allow-env"], labels["dynamic/env"])`,
				},
			}),
			assert: require.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, CheckDynamicLabelsInDenyRules(tc.role))
		})
	}
}
