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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/services"
)

func TestGetUserRoles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	client := &mockRoleClient{}
	client.On("GetRole", mock.Anything, "dev").
		Return(&types.RoleV6{
			Metadata: types.Metadata{Name: "dev"},
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"root", "{{internal.logins}}"},
				},
			},
		}, nil)

	tests := []struct {
		description string
		roleNames   []string
		traits      trait.Traits
		expected    []types.Role
	}{
		{
			description: "traits applied",
			roleNames:   []string{"dev"},
			traits: trait.Traits{
				"logins": {"foo", "bar"},
			},
			expected: []types.Role{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "dev"},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Logins: []string{"root", "foo", "bar"},
						},
					},
				},
			},
		},
		{
			description: "traits not provided",
			roleNames:   []string{"dev"},
			traits:      nil,
			expected: []types.Role{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "dev"},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Logins: []string{"root", "{{internal.logins}}"},
						},
					},
				},
			},
		},
		{
			description: "traits don't apply",
			roleNames:   []string{"dev"},
			traits: trait.Traits{
				"foo": {"bar"},
			},
			expected: []types.Role{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "dev"},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Logins: []string{"root"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			roles, err := GetUserRoles(ctx, client, tt.roleNames, tt.traits)
			require.NoError(t, err)
			require.Len(t, roles, len(tt.expected))

			for i, role := range roles {
				require.Equal(t, tt.expected[i].GetName(), role.GetName())
				require.ElementsMatch(t, tt.expected[i].GetLogins(types.Allow), role.GetLogins(types.Allow))
			}
		})
	}
}

type mockRoleClient struct {
	mock.Mock
	services.RoleGetter
}

func (m *mockRoleClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(types.Role), args.Error(1)
}
