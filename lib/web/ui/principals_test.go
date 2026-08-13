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

package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMakeResourcePrincipalSets(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   []types.ResourcePrincipalSet
		want []ResourcePrincipalSet
	}{
		"no dimensions yields nil": {
			in:   nil,
			want: nil,
		},
		"values are sorted and byRole omitted when absent": {
			in: []types.ResourcePrincipalSet{{
				PrincipalType: types.PrincipalTypeLogins,
				Granted:       []string{"ubuntu", "admin"},
				Requestable:   []string{"root", "ec2-user"},
			}},
			want: []ResourcePrincipalSet{{
				PrincipalType: types.PrincipalTypeLogins,
				Granted:       []string{"admin", "ubuntu"},
				Requestable:   []string{"ec2-user", "root"},
			}},
		},
		"empty dimension keeps its type": {
			in:   []types.ResourcePrincipalSet{{PrincipalType: types.PrincipalTypeRoleARNs}},
			want: []ResourcePrincipalSet{{PrincipalType: types.PrincipalTypeRoleARNs}},
		},
		"byRole attribution is carried through": {
			in: []types.ResourcePrincipalSet{{
				PrincipalType: "db_users",
				Granted:       []string{"reader"},
				Requestable:   []string{"writer"},
				ByRole: []types.RolePrincipalValues{
					{Role: "held", Values: []string{"reader"}},
					{Role: "requestable", RequiresRequest: true, Values: []string{"writer"}},
				},
			}},
			want: []ResourcePrincipalSet{{
				PrincipalType: "db_users",
				Granted:       []string{"reader"},
				Requestable:   []string{"writer"},
				ByRole: []RolePrincipalValues{
					{Role: "held", Values: []string{"reader"}},
					{Role: "requestable", RequiresRequest: true, Values: []string{"writer"}},
				},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, MakeResourcePrincipalSets(tc.in))
		})
	}
}

