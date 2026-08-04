// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestGenerateTerraformConfigWithPresetBuilder(t *testing.T) {
	accessRole := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "accessRole1",
		},
	}

	accessRole2 := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "accessRole2",
		},
	}

	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "test-access-list"},
		accesslist.Spec{
			Title:  "Test Access List",
			Owners: []accesslist.Owner{{Name: "llama", Description: "some description"}},
			Audit: accesslist.Audit{
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FirstDayOfMonth,
				},
			},
		},
	)
	require.NoError(t, err)

	members := []*accesslist.AccessListMember{
		makeUserMember(t, accessList.GetName(), "llama", ""),
		makeUserMember(t, accessList.GetName(), "alpaca", "some reason"),
	}

	tests := []struct {
		name string
		cfg  ConfigParams
	}{
		{
			name: "long-term-preset",
			cfg: ConfigParams{
				PresetType:   preset.LongTermPresetType,
				AccessListID: accessList.GetName(),
				AccessList:   accessList,
				AccessRoles: []AccessRole{
					{Role: accessRole, BlockComment: "Some kind of block comment"},
					{Role: accessRole2, BlockComment: "Some kind of block comment for role 2"},
				},
				Members: members,
				ProviderBlock: &ProviderBlock{
					TeleportVersion: 19,
					ProxyAddr:       "proxy.example.com:3080",
				},
			},
		},
		{
			name: "short-term-preset",
			cfg: ConfigParams{
				PresetType:   preset.ShortTermPresetType,
				AccessListID: accessList.GetName(),
				AccessList:   accessList,
				AccessRoles: []AccessRole{
					{Role: accessRole},
				},
				Members: members,
				ProviderBlock: &ProviderBlock{
					TeleportVersion: 19,
					ProxyAddr:       "proxy.example.com:3080",
				},
			},
		},
		{
			// Test roles are generated with nil access list
			name: "access-roles-nil-access-list",
			cfg: ConfigParams{
				PresetType:   preset.LongTermPresetType,
				AccessListID: accessList.GetName(),
				AccessRoles: []AccessRole{
					{Role: accessRole, BlockComment: "Some kind of block comment"},
					{Role: accessRole2, BlockComment: "Some kind of block comment for role 2"},
				},
			},
		},
		{
			// Access list is generated with nil roles and members
			name: "access-list-nil-roles",
			cfg: ConfigParams{
				PresetType:   preset.LongTermPresetType,
				AccessListID: accessList.GetName(),
				AccessList:   accessList,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := GenerateConfigWithPresetBuilder(tt.cfg)
			require.NoError(t, err)

			if golden.ShouldSet() {
				golden.Set(t, []byte(out))
			}
			require.Equal(t, string(golden.Get(t)), out)
		})
	}

	// Test preset is required.
	out, err := GenerateConfigWithPresetBuilder(ConfigParams{})
	require.Equal(t, "preset type is required", err.Error())
	require.Empty(t, out)

	// Test empty params minus required preset type.
	out, err = GenerateConfigWithPresetBuilder(ConfigParams{PresetType: preset.LongTermPresetType})
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestGenerateTerraformConfig(t *testing.T) {
	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "plain-access-list"},
		accesslist.Spec{
			Title:       "Plain Access List",
			Description: "some description",
			Owners:      []accesslist.Owner{{Name: "llama", Description: "owner description"}},
			Audit: accesslist.Audit{
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FirstDayOfMonth,
				},
			},
			Grants: accesslist.Grants{
				Roles:  []string{"access", "editor"},
				Traits: map[string][]string{"logins": {"root"}},
			},
		},
	)
	require.NoError(t, err)

	members := []*accesslist.AccessListMember{
		makeUserMember(t, accessList.GetName(), "llama", ""),
		makeUserMember(t, accessList.GetName(), "alpaca", "some reason"),
	}

	tests := []struct {
		name          string
		members       []*accesslist.AccessListMember
		providerBlock *ProviderBlock
	}{
		{
			name:          "with-members",
			members:       members,
			providerBlock: &ProviderBlock{TeleportVersion: 19, ProxyAddr: "proxy.example.com:3080"},
		},
		{
			name: "nil-members",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := GenerateConfig(accessList, tt.members, tt.providerBlock)
			require.NoError(t, err)

			if golden.ShouldSet() {
				golden.Set(t, []byte(out))
			}
			require.Equal(t, string(golden.Get(t)), out)
		})
	}

	// Test empty list.
	out, err := GenerateConfig(nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, out)
}

func makeUserMember(t *testing.T, alName string, name string, reason string) *accesslist.AccessListMember {
	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: name},
		accesslist.AccessListMemberSpec{
			AccessList:     alName,
			Name:           name,
			Reason:         reason,
			MembershipKind: accesslist.MembershipKindUser,
		},
	)
	require.NoError(t, err)
	return member
}
