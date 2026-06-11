/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accesslist

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestApplyGrantsAndRequires_AllFieldsSet(t *testing.T) {
	cmd := Command{
		ownerGrantRolesSet:      true,
		ownerGrantRoles:         "owner-grant-role",
		ownerGrantTraitsSet:     true,
		ownerGrantTraits:        "owner-grant=owner-grant-trait",
		ownerRequiredRolesSet:   true,
		ownerRequiredRoles:      "owner-required-role",
		ownerRequiredTraitsSet:  true,
		ownerRequiredTraits:     "owner-required=owner-required-trait",
		memberGrantRolesSet:     true,
		memberGrantRoles:        "member-grant-role",
		memberGrantTraitsSet:    true,
		memberGrantTraits:       "member-grant=member-grant-trait",
		memberRequiredRolesSet:  true,
		memberRequiredRoles:     "member-required-role",
		memberRequiredTraitsSet: true,
		memberRequiredTraits:    "member-required=member-required-trait",
	}

	al := &accesslist.AccessList{}
	require.NoError(t, cmd.applyGrantsAndRequires(al))

	require.Equal(t, []string{"owner-grant-role"}, al.Spec.OwnerGrants.Roles)
	require.Equal(t, []string{"owner-grant-trait"}, al.Spec.OwnerGrants.Traits["owner-grant"])
	require.Equal(t, []string{"owner-required-role"}, al.Spec.OwnershipRequires.Roles)
	require.Equal(t, []string{"owner-required-trait"}, al.Spec.OwnershipRequires.Traits["owner-required"])
	require.Equal(t, []string{"member-grant-role"}, al.Spec.Grants.Roles)
	require.Equal(t, []string{"member-grant-trait"}, al.Spec.Grants.Traits["member-grant"])
	require.Equal(t, []string{"member-required-role"}, al.Spec.MembershipRequires.Roles)
	require.Equal(t, []string{"member-required-trait"}, al.Spec.MembershipRequires.Traits["member-required"])
}

func TestApplyGrantsAndRequires_NoFlagsSet(t *testing.T) {
	al := dummyAccessList()
	want := dummyAccessList()

	require.NoError(t, (&Command{}).applyGrantsAndRequires(al))

	require.Equal(t, want.Spec.Grants, al.Spec.Grants)
	require.Equal(t, want.Spec.OwnerGrants, al.Spec.OwnerGrants)
	require.Equal(t, want.Spec.MembershipRequires, al.Spec.MembershipRequires)
	require.Equal(t, want.Spec.OwnershipRequires, al.Spec.OwnershipRequires)
}

func TestApplyGrantsAndRequires_SetToEmpty(t *testing.T) {
	cmd := Command{
		ownerGrantRolesSet:      true,
		ownerGrantTraitsSet:     true,
		ownerRequiredRolesSet:   true,
		ownerRequiredTraitsSet:  true,
		memberGrantRolesSet:     true,
		memberGrantTraitsSet:    true,
		memberRequiredRolesSet:  true,
		memberRequiredTraitsSet: true,
	}

	al := dummyAccessList()
	require.NoError(t, cmd.applyGrantsAndRequires(al))

	require.Empty(t, al.Spec.OwnerGrants.Roles)
	require.Empty(t, al.Spec.OwnerGrants.Traits)
	require.Empty(t, al.Spec.OwnershipRequires.Roles)
	require.Empty(t, al.Spec.OwnershipRequires.Traits)
	require.Empty(t, al.Spec.Grants.Roles)
	require.Empty(t, al.Spec.Grants.Traits)
	require.Empty(t, al.Spec.MembershipRequires.Roles)
	require.Empty(t, al.Spec.MembershipRequires.Traits)
}

func TestApplyGrantsAndRequires_InvalidTrait(t *testing.T) {
	tests := []struct {
		name string
		cmd  Command
	}{
		{
			name: "owner-grant-traits",
			cmd:  Command{ownerGrantTraitsSet: true, ownerGrantTraits: "invalid-trait"},
		},
		{
			name: "owner-required-traits",
			cmd:  Command{ownerRequiredTraitsSet: true, ownerRequiredTraits: "invalid-trait"},
		},
		{
			name: "member-grant-traits",
			cmd:  Command{memberGrantTraitsSet: true, memberGrantTraits: "invalid-trait"},
		},
		{
			name: "member-required-traits",
			cmd:  Command{memberRequiredTraitsSet: true, memberRequiredTraits: "invalid-trait"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.applyGrantsAndRequires(&accesslist.AccessList{})
			require.Error(t, err)
		})
	}
}

func dummyAccessList() *accesslist.AccessList {
	grants := accesslist.Grants{
		Roles:  []string{"some-role"},
		Traits: map[string][]string{"some": {"trait"}},
	}

	requires := accesslist.Requires{
		Roles:  []string{"some-role"},
		Traits: map[string][]string{"some": {"trait"}},
	}
	return &accesslist.AccessList{
		Spec: accesslist.Spec{
			Grants:             grants,
			OwnerGrants:        grants,
			MembershipRequires: requires,
			OwnershipRequires:  requires,
		},
	}
}
