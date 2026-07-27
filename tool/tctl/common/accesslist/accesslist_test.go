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
	"github.com/gravitational/teleport/api/types/trait"
)

func TestApplyGrantsAndRequirements(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		al      *accesslist.AccessList
		wantErr bool
		wantAl  *accesslist.AccessList
	}{
		{
			name: "flag values gets applied",
			al:   &accesslist.AccessList{},
			cmd: Command{
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
			},
			wantAl: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Grants: accesslist.Grants{
						Roles:  []string{"member-grant-role"},
						Traits: map[string][]string{"member-grant": {"member-grant-trait"}},
					},
					OwnerGrants: accesslist.Grants{
						Roles:  []string{"owner-grant-role"},
						Traits: map[string][]string{"owner-grant": {"owner-grant-trait"}},
					},
					MembershipRequires: accesslist.Requires{
						Roles:  []string{"member-required-role"},
						Traits: map[string][]string{"member-required": {"member-required-trait"}},
					},
					OwnershipRequires: accesslist.Requires{
						Roles:  []string{"owner-required-role"},
						Traits: map[string][]string{"owner-required": {"owner-required-trait"}},
					},
				},
			},
		},
		{
			name:   "no flags set, leaves existing values alone",
			al:     dummyAccessList(),
			cmd:    Command{},
			wantAl: dummyAccessList(),
		},
		{
			name: "set flags to clear existing values",
			al:   dummyAccessList(),
			cmd: Command{
				ownerGrantRolesSet:      true,
				ownerGrantTraitsSet:     true,
				ownerRequiredRolesSet:   true,
				ownerRequiredTraitsSet:  true,
				memberGrantRolesSet:     true,
				memberGrantTraitsSet:    true,
				memberRequiredRolesSet:  true,
				memberRequiredTraitsSet: true,
			},
			wantAl: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Grants:             accesslist.Grants{Roles: []string{}, Traits: trait.Traits{}},
					OwnerGrants:        accesslist.Grants{Roles: []string{}, Traits: trait.Traits{}},
					MembershipRequires: accesslist.Requires{Roles: []string{}, Traits: trait.Traits{}},
					OwnershipRequires:  accesslist.Requires{Roles: []string{}, Traits: trait.Traits{}},
				},
			}},
		{
			name:    "invalid owner grant traits",
			al:      &accesslist.AccessList{},
			cmd:     Command{ownerGrantTraitsSet: true, ownerGrantTraits: "invalid-trait"},
			wantErr: true,
		},
		{
			name:    "invalid owner required traits",
			al:      &accesslist.AccessList{},
			cmd:     Command{ownerRequiredTraitsSet: true, ownerRequiredTraits: "invalid-trait"},
			wantErr: true,
		},
		{
			name:    "invalid member grant traits",
			al:      &accesslist.AccessList{},
			cmd:     Command{memberGrantTraitsSet: true, memberGrantTraits: "invalid-trait"},
			wantErr: true,
		},
		{
			name:    "invalid member required traits",
			al:      &accesslist.AccessList{},
			cmd:     Command{memberRequiredTraitsSet: true, memberRequiredTraits: "invalid-trait"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.applyGrantsAndRequirements(tt.al)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantAl, tt.al)
			}
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
