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

package accesslist

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name        string
		cmd         Command
		errContains string
	}{
		{
			name: "valid minimal plain with user members",
			cmd:  Command{titleSet: true, ownersSet: true},
		},
		{
			name: "valid minimal plain with owner-access-lists",
			cmd:  Command{titleSet: true, ownerAccessListsSet: true},
		},
		{
			name: "valid minimal preset access list",
			cmd:  Command{accessType: accessTypeLongTerm, titleSet: true, ownersSet: true},
		},
		{
			name:        "missing title",
			cmd:         Command{ownersSet: true},
			errContains: "--title is required",
		},
		{
			name:        "missing owners",
			cmd:         Command{titleSet: true},
			errContains: "at least one of --owners or --owner-access-lists",
		},
		{
			name: "grants, traits, and requirements are valid on a plain list",
			cmd: Command{titleSet: true,
				membersSet:              true,
				memberGrantRolesSet:     true,
				memberGrantTraitsSet:    true,
				memberRequiredRolesSet:  true,
				memberRequiredTraitsSet: true,
				ownersSet:               true,
				ownerGrantRolesSet:      true,
				ownerGrantTraitsSet:     true,
				ownerRequiredRolesSet:   true,
				ownerRequiredTraitsSet:  true,
			},
		},
		{
			name: "requirements are valid on a preset access list",
			cmd: Command{accessType: accessTypeLongTerm, titleSet: true,
				membersSet:              true,
				memberRequiredRolesSet:  true,
				memberRequiredTraitsSet: true,
				ownersSet:               true,
				ownerRequiredRolesSet:   true,
				ownerRequiredTraitsSet:  true,
			},
		},
		{
			name:        "invalid use of resource flags without access-type",
			cmd:         Command{titleSet: true, ownersSet: true, nodeLabelsSet: true},
			errContains: "require --access-type",
		},
		{
			name:        "invalid access-type",
			cmd:         Command{accessType: "invalidtype", titleSet: true, ownersSet: true},
			errContains: "--access-type must be",
		},
		{
			name:        "member grant roles cannot be combined with access-type",
			cmd:         Command{accessType: accessTypeLongTerm, titleSet: true, ownersSet: true, memberGrantRolesSet: true},
			errContains: "cannot be combined with --access-type",
		},
		{
			name:        "member grant traits cannot be combined with access-type",
			cmd:         Command{accessType: accessTypeLongTerm, titleSet: true, ownersSet: true, memberGrantTraitsSet: true},
			errContains: "cannot be combined with --access-type",
		},
		{
			name:        "owner grant roles cannot be combined with access-type",
			cmd:         Command{accessType: accessTypeLongTerm, titleSet: true, ownersSet: true, ownerGrantRolesSet: true},
			errContains: "cannot be combined with --access-type",
		},
		{
			name:        "owner grant traits cannot be combined with access-type",
			cmd:         Command{accessType: accessTypeLongTerm, titleSet: true, ownersSet: true, ownerGrantTraitsSet: true},
			errContains: "cannot be combined with --access-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.validateCreate()
			if tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestBuildResourceAccessRoles(t *testing.T) {
	const awsicAssignment = "1234:arn:aws:sso:::permissionSet/test"

	tests := []struct {
		name                 string
		cmd                  Command
		wantRoleNamePrefixes []string
		assert               func(t *testing.T, roles []*types.RoleV6)
	}{
		{
			name:                 "no resource flags builds no roles",
			cmd:                  Command{},
			wantRoleNamePrefixes: nil,
		},
		{
			name:                 "standard label flag builds only the standard role",
			cmd:                  Command{nodeLabelsSet: true, nodeLabels: "env=dev"},
			wantRoleNamePrefixes: []string{standardRolePrefixName},
			assert: func(t *testing.T, roles []*types.RoleV6) {
				require.Equal(t, []string{"dev"}, []string(roles[0].Spec.Allow.NodeLabels["env"]))
			},
		},
		{
			name:                 "standard identity-only flag still builds the standard role",
			cmd:                  Command{dbNamesSet: true, dbNames: "name"},
			wantRoleNamePrefixes: []string{standardRolePrefixName},
			assert: func(t *testing.T, roles []*types.RoleV6) {
				require.Equal(t, []string{"name"}, roles[0].Spec.Allow.DatabaseNames)
				require.Empty(t, roles[0].Spec.Allow.DatabaseLabels)
			},
		},
		{
			name:                 "awsic flag builds only the awsic role",
			cmd:                  Command{awsicAssignmentsSet: true, awsicAssignments: awsicAssignment},
			wantRoleNamePrefixes: []string{awsicRolePrefixName},
			assert: func(t *testing.T, roles []*types.RoleV6) {
				require.Len(t, roles[0].Spec.Allow.AccountAssignments, 1)
				require.Equal(t, "1234", roles[0].Spec.Allow.AccountAssignments[0].Account)
			},
		},
		{
			name: "standard and awsic flags build both roles",
			cmd: Command{
				nodeLabelsSet: true, nodeLabels: "env=dev",
				awsicAssignmentsSet: true, awsicAssignments: awsicAssignment,
			},
			wantRoleNamePrefixes: []string{standardRolePrefixName, awsicRolePrefixName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles, err := tt.cmd.buildResourceAccessRoles()
			require.NoError(t, err)

			roleNames := make([]string, 0, len(roles))
			for _, r := range roles {
				roleNames = append(roleNames, r.GetName())
			}
			require.ElementsMatch(t, tt.wantRoleNamePrefixes, roleNames)

			if tt.assert != nil {
				tt.assert(t, roles)
			}
		})
	}
}

func TestBuildMembers(t *testing.T) {
	const listID = "acl-123"
	c := Command{members: "apple,banana", memberAccessLists: "some-list"}

	members, err := c.buildMembers(listID)
	require.NoError(t, err)

	gotKinds := make(map[string]string, len(members))
	for _, m := range members {
		require.Equal(t, listID, m.Spec.AccessList)
		gotKinds[m.Spec.Name] = m.Spec.MembershipKind
	}

	require.Equal(t, map[string]string{
		"apple":     accesslist.MembershipKindUser,
		"banana":    accesslist.MembershipKindUser,
		"some-list": accesslist.MembershipKindList,
	}, gotKinds)
}

func TestBuildOwners(t *testing.T) {
	c := Command{owners: "apple,banana", ownerAccessLists: "some-list"}

	owners := c.buildOwners()

	gotKinds := make(map[string]string, len(owners))
	for _, o := range owners {
		gotKinds[o.Name] = o.MembershipKind
	}

	require.Equal(t, map[string]string{
		"apple":     accesslist.MembershipKindUser,
		"banana":    accesslist.MembershipKindUser,
		"some-list": accesslist.MembershipKindList,
	}, gotKinds)
}
