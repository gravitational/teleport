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

package assignments

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	srpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedrole/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

// TestListScopedRoleAssignmentsScenarios tests particualr more tricky ListScopedRoleAssignments scenarios, such
// as attempts to use an out-of-band cursor to read values outside of the target scope.
func TestListScopedRoleASsignmentsScenarios(t *testing.T) {
	t.Parallel()

	assignments := []*srpb.ScopedRoleAssignment{
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-01",
						Scope: "/aa",
					},
					{
						Role:  "role-02",
						Scope: "/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-02",
			},
			Scope: "/aa",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-03",
						Scope: "/aa",
					},
					{
						Role:  "role-04",
						Scope: "/aa/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-01",
						Scope: "/aa",
					},
					{
						Role:  "role-02",
						Scope: "/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-02",
			},
			Scope: "/aa",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-03",
						Scope: "/aa",
					},
					{
						Role:  "role-04",
						Scope: "/aa/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-03",
			},
			Scope: "/aa/bb",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-05",
						Scope: "/aa/bb",
					},
					{
						Role:  "role-06",
						Scope: "/aa/bb/cc",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-03",
			},
			Scope: "/aa/bb",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-05",
						Scope: "/aa/bb",
					},
					{
						Role:  "role-06",
						Scope: "/aa/bb/cc",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "carol-01",
			},
			Scope: "/bb",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "carol",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-07",
						Scope: "/bb",
					},
					{
						Role:  "role-08",
						Scope: "/bb/cc",
					},
				},
			},
			Version: types.V1,
		},
	}

	cache := NewAssignmentCache()
	for _, assignment := range assignments {
		cache.Put(assignment)
	}

	// verify expected behavior for standard cursors in resources subject to scope mode
	rsp, err := cache.ListScopedRoleAssignments(t.Context(), &srpb.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-02@/aa",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious root out-of-band cursor in resources subject to scope mode (ignored)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &srpb.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"alice-02", "bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious orthogonal out-of-band cursor in resources subject to scope mode (ignored)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &srpb.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:carol-01@/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"alice-02", "bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.Assignments))

	// verify expected behavior for standard cursors in policies applicable to scope mode
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &srpb.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"bob-01", "alice-02", "bob-02"}, collectAssignmentNames(rsp.Assignments))

	fmt.Println("---> starting final query <---")
	// try to inject a malicious child out-of-band cursor in policies applicable to scope mode (ignored)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &srpb.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-03@/aa/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Empty(t, collectAssignmentNames(rsp.Assignments))
}

// TestListScopedRoleAssignmentsBasics tests some basic functionality of the ListScopedRoleAssignments method.
func TestListScopedRoleAssignmentsBasics(t *testing.T) {
	t.Parallel()

	assignments := []*srpb.ScopedRoleAssignment{
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-01",
						Scope: "/aa",
					},
					{
						Role:  "role-02",
						Scope: "/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-02",
			},
			Scope: "/aa",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-03",
						Scope: "/aa",
					},
					{
						Role:  "role-04",
						Scope: "/aa/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-01",
						Scope: "/aa",
					},
					{
						Role:  "role-02",
						Scope: "/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-02",
			},
			Scope: "/aa",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-03",
						Scope: "/aa",
					},
					{
						Role:  "role-04",
						Scope: "/aa/bb",
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "carol-01",
			},
			Scope: "/bb",
			Spec: &srpb.ScopedRoleAssignmentSpec{
				User: "carol",
				Assignments: []*srpb.Assignment{
					{
						Role:  "role-05",
						Scope: "/bb",
					},
					{
						Role:  "role-06",
						Scope: "/bb/cc",
					},
				},
			},
			Version: types.V1,
		},
	}

	tts := []struct {
		name   string
		req    *srpb.ListScopedRoleAssignmentsRequest
		expect [][]string
	}{
		{
			name: "all single page explicit excess",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				PageSize: int32(len(assignments) + 1),
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
					"alice-02",
					"bob-02",
					"carol-01",
				},
			},
		},
		{
			name: "all single page implicit excess",
			req:  &srpb.ListScopedRoleAssignmentsRequest{},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
					"alice-02",
					"bob-02",
					"carol-01",
				},
			},
		},
		{
			name: "all single page exact",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				PageSize: int32(len(assignments)),
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
					"alice-02",
					"bob-02",
					"carol-01",
				},
			},
		},
		{
			name: "all multi page",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				PageSize: 2,
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
				},
				{
					"alice-02",
					"bob-02",
				},
				{
					"carol-01",
				},
			},
		},
		{
			name: "user single page",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				User: "alice",
			},
			expect: [][]string{
				{
					"alice-01",
					"alice-02",
				},
			},
		},
		{
			name: "user multi page",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				PageSize: 1,
				User:     "alice",
			},
			expect: [][]string{
				{"alice-01"},
				{"alice-02"},
			},
		},
		{
			name: "user nonexistent",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				User: "dave",
			},
			expect: nil,
		},
		{
			name: "role single page",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				Role: "role-01",
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
				},
			},
		},
		{
			name: "role multi page",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				PageSize: 1,
				Role:     "role-01",
			},
			expect: [][]string{
				{"alice-01"},
				{"bob-01"},
			},
		},
		{
			name: "role nonexistent",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				Role: "role-99",
			},
			expect: nil,
		},
		{
			name: "resouce scope root",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/",
				},
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
					"alice-02",
					"bob-02",
					"carol-01",
				},
			},
		},
		{
			name: "resouce scope non-root",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/aa",
				},
			},
			expect: [][]string{
				{
					"alice-02",
					"bob-02",
				},
			},
		},
		{
			name: "policy scope root",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/",
				},
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
				},
			},
		},
		{
			name: "policy scope non-root",
			req: &srpb.ListScopedRoleAssignmentsRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/aa",
				},
			},
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
					"alice-02",
					"bob-02",
				},
			},
		},
	}

	cache := NewAssignmentCache()
	for _, assignment := range assignments {
		cache.Put(assignment)
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			var out [][]string
			for {
				rsp, err := cache.ListScopedRoleAssignments(t.Context(), tt.req)
				require.NoError(t, err)

				if len(rsp.Assignments) == 0 {
					break
				}

				out = append(out, collectAssignmentNames(rsp.Assignments))

				if rsp.NextPageToken == "" {
					break
				}

				tt.req.PageToken = rsp.NextPageToken
			}

			require.Equal(t, tt.expect, out)
		})
	}
}

func collectAssignmentNames(assignments []*srpb.ScopedRoleAssignment) []string {
	var names []string
	for _, assignment := range assignments {
		names = append(names, assignment.Metadata.Name)
	}
	return names
}
