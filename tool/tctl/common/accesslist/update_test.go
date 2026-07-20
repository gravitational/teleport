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

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestApplySpecFlags(t *testing.T) {
	tests := []struct {
		name   string
		cmd    Command
		assert func(t *testing.T, al *accesslist.AccessList)
	}{
		{
			name: "title",
			cmd:  Command{titleSet: true, title: "New Title"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, "New Title", al.Spec.Title)
			},
		},
		{
			name: "description",
			cmd:  Command{descriptionSet: true, description: "New Desc"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, "New Desc", al.Spec.Description)
			},
		},
		{
			name: "audit-frequency",
			cmd:  Command{auditFrequencySet: true, auditFrequency: 6},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, accesslist.SixMonths, al.Spec.Audit.Recurrence.Frequency)
			},
		},
		{
			name: "audit-day",
			cmd:  Command{auditDaySet: true, auditDay: 15},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, accesslist.FifteenthDayOfMonth, al.Spec.Audit.Recurrence.DayOfMonth)
			},
		},
		{
			name: "owner-grant-roles",
			cmd:  Command{ownerGrantRolesSet: true, ownerGrantRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.OwnerGrants.Roles)
			},
		},
		{
			name: "owner-grant-traits",
			cmd:  Command{ownerGrantTraitsSet: true, ownerGrantTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.OwnerGrants.Traits["team"])
			},
		},
		{
			name: "owner-required-roles",
			cmd:  Command{ownerRequiredRolesSet: true, ownerRequiredRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.OwnershipRequires.Roles)
			},
		},
		{
			name: "owner-required-traits",
			cmd:  Command{ownerRequiredTraitsSet: true, ownerRequiredTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.OwnershipRequires.Traits["team"])
			},
		},
		{
			name: "member-grant-roles",
			cmd:  Command{memberGrantRolesSet: true, memberGrantRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.Grants.Roles)
			},
		},
		{
			name: "member-grant-traits",
			cmd:  Command{memberGrantTraitsSet: true, memberGrantTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.Grants.Traits["team"])
			},
		},
		{
			name: "member-required-roles",
			cmd:  Command{memberRequiredRolesSet: true, memberRequiredRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.MembershipRequires.Roles)
			},
		},
		{
			name: "member-required-traits",
			cmd:  Command{memberRequiredTraitsSet: true, memberRequiredTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.MembershipRequires.Traits["team"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			al := &accesslist.AccessList{}
			require.NoError(t, tt.cmd.applySpecFlags(al))
			tt.assert(t, al)
		})
	}
}
