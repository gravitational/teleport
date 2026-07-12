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

package access

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

func TestMatchSecondaryAssignmentFilters(t *testing.T) {
	t.Parallel()

	assignment := scopedaccessv1.ScopedRoleAssignment_builder{
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{Role: "role-a", Scope: "/aa"}.Build(),
				scopedaccessv1.Assignment_builder{Role: "role-b", Scope: "/aa/bb"}.Build(),
			},
		}.Build(),
	}.Build()

	tests := []struct {
		name string
		req  *scopedaccessv1.ListScopedRoleAssignmentsRequest
		want bool
	}{
		{
			name: "no filters matches",
			req:  scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{}.Build(),
			want: true,
		},
		{
			name: "matching user",
			req:  scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{User: "alice"}.Build(),
			want: true,
		},
		{
			name: "non-matching user",
			req:  scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{User: "bob"}.Build(),
			want: false,
		},
		{
			name: "matching role",
			req:  scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{Role: "role-b"}.Build(),
			want: true,
		},
		{
			name: "non-matching role",
			req:  scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{Role: "role-x"}.Build(),
			want: false,
		},
		{
			name: "matching assigned scope",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				AssignedScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/aa/bb"}.Build(),
			}.Build(),
			want: true,
		},
		{
			name: "non-matching assigned scope",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				AssignedScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/cc"}.Build(),
			}.Build(),
			want: false,
		},
		{
			name: "all filters together match",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				User:                "alice",
				Role:                "role-a",
				AssignedScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/aa"}.Build(),
			}.Build(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MatchSecondaryAssignmentFilters(tt.req, assignment))
		})
	}
}
