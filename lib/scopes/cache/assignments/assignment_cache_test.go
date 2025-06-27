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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

// TestListScopedRoleAssignmentsScenarios tests particular more tricky ListScopedRoleAssignments scenarios, such
// as attempts to use an out-of-band cursor to read values outside of the target scope.
func TestListScopedRoleASsignmentsScenarios(t *testing.T) {
	t.Parallel()

	assignments := []*accessv1.ScopedRoleAssignment{
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-01",
			},
			Scope: "/",
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "carol",
				Assignments: []*accessv1.Assignment{
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
	rsp, err := cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-02@/aa",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious root out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"alice-02", "bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious orthogonal out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
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
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"bob-01", "alice-02", "bob-02"}, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious child out-of-band cursor in policies applicable to scope mode (effect is
	// to ignore all items in valid query path).
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:bob-03@/aa/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Empty(t, collectAssignmentNames(rsp.Assignments))

	// try to inject a malicious orthogonal out-of-band cursor in policies applicable to scope mode (effect is to
	// ignore root, but process leaf normally).
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:carol-01@/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"alice-02", "bob-02"}, collectAssignmentNames(rsp.Assignments))

	// verify rejection of unknown cursor version
	_, err = cache.ListScopedRoleAssignments(t.Context(), &accessv1.ListScopedRoleAssignmentsRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v2:bob-02@/aa",
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestListScopedRoleAssignmentsBasics tests some basic functionality of the ListScopedRoleAssignments method.
func TestListScopedRoleAssignmentsBasics(t *testing.T) {
	t.Parallel()

	assignments := []*accessv1.ScopedRoleAssignment{
		{
			Kind: sr.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-01",
			},
			Scope: "/",
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*accessv1.Assignment{
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
			Spec: &accessv1.ScopedRoleAssignmentSpec{
				User: "carol",
				Assignments: []*accessv1.Assignment{
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
		req    *accessv1.ListScopedRoleAssignmentsRequest
		expect [][]string
	}{
		{
			name: "all single page explicit excess",
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req:  &accessv1.ListScopedRoleAssignmentsRequest{},
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
				User: "dave",
			},
			expect: nil,
		},
		{
			name: "role single page",
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
				Role: "role-99",
			},
			expect: nil,
		},
		{
			name: "resource scope root",
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			name: "resource scope non-root",
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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
			req: &accessv1.ListScopedRoleAssignmentsRequest{
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

func collectAssignmentNames(assignments []*accessv1.ScopedRoleAssignment) []string {
	var names []string
	for _, assignment := range assignments {
		names = append(names, assignment.Metadata.Name)
	}
	return names
}
