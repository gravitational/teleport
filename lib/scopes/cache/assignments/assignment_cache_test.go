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
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestListScopedRoleAssignmentsScenarios tests particular more tricky ListScopedRoleAssignments scenarios, such
// as attempts to use an out-of-band cursor to read values outside of the target scope.
func TestListScopedRoleAssignmentsScenarios(t *testing.T) {
	t.Parallel()

	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-01",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-02",
						Scope: "/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-02",
			}.Build(),
			Scope: "/aa",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-03",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-04",
						Scope: "/aa/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bob-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-01",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-02",
						Scope: "/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bob-02",
			}.Build(),
			Scope: "/aa",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-03",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-04",
						Scope: "/aa/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-03",
			}.Build(),
			Scope: "/aa/bb",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-05",
						Scope: "/aa/bb",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-06",
						Scope: "/aa/bb/cc",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bob-03",
			}.Build(),
			Scope: "/aa/bb",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-05",
						Scope: "/aa/bb",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-06",
						Scope: "/aa/bb/cc",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "carol-01",
			}.Build(),
			Scope: "/bb",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "carol",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-07",
						Scope: "/bb",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-08",
						Scope: "/bb/cc",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	cache := NewAssignmentCache(AssignmentCacheConfig{})
	for _, assignment := range assignments {
		_, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(assignment)

		rsp, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, rsp.GetAssignment())
	}

	// verify expected behavior for standard cursors in resources subject to scope mode
	rsp, err := cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:bob-02/dynamic@/aa",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.GetAssignments()))

	// try to inject a malicious root out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:bob-01/dynamic@/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"alice-02", "bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.GetAssignments()))

	// try to inject a malicious orthogonal out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:carol-01/dynamic@/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"alice-02", "bob-02", "alice-03", "bob-03"}, collectAssignmentNames(rsp.GetAssignments()))

	// verify expected behavior for standard cursors in policies applicable to scope mode
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:bob-01/dynamic@/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"bob-01", "alice-02", "bob-02"}, collectAssignmentNames(rsp.GetAssignments()))

	// try to inject a malicious child out-of-band cursor in policies applicable to scope mode (effect is
	// to ignore all items in valid query path).
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:bob-03/dynamic@/aa/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Empty(t, collectAssignmentNames(rsp.GetAssignments()))

	// try to inject a malicious orthogonal out-of-band cursor in policies applicable to scope mode (effect is to
	// ignore root, but process leaf normally).
	rsp, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:carol-01/dynamic@/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"alice-02", "bob-02"}, collectAssignmentNames(rsp.GetAssignments()))

	// verify rejection of unknown cursor version
	_, err = cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v2:bob-02/dynamic@/aa",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestListScopedRoleAssignmentsBasics tests some basic functionality of the ListScopedRoleAssignments method.
func TestListScopedRoleAssignmentsBasics(t *testing.T) {
	t.Parallel()

	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-01",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-02",
						Scope: "/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-02",
			}.Build(),
			Scope: "/aa",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-03",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-04",
						Scope: "/aa/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bob-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-01",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-02",
						Scope: "/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bob-02",
			}.Build(),
			Scope: "/aa",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-03",
						Scope: "/aa",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-04",
						Scope: "/aa/bb",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "carol-01",
			}.Build(),
			Scope: "/bb",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "carol",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-05",
						Scope: "/bb",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "role-06",
						Scope: "/bb/cc",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	tts := []struct {
		name   string
		req    *scopedaccessv1.ListScopedRoleAssignmentsRequest
		expect [][]string
	}{
		{
			name: "all single page explicit excess",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				PageSize: int32(len(assignments) + 1),
			}.Build(),
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
			req:  &scopedaccessv1.ListScopedRoleAssignmentsRequest{},
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
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				PageSize: int32(len(assignments)),
			}.Build(),
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
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				PageSize: 2,
			}.Build(),
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
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				User: "alice",
			}.Build(),
			expect: [][]string{
				{
					"alice-01",
					"alice-02",
				},
			},
		},
		{
			name: "user multi page",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				PageSize: 1,
				User:     "alice",
			}.Build(),
			expect: [][]string{
				{"alice-01"},
				{"alice-02"},
			},
		},
		{
			name: "user nonexistent",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				User: "dave",
			}.Build(),
			expect: nil,
		},
		{
			name: "role single page",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				Role: "role-01",
			}.Build(),
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
				},
			},
		},
		{
			name: "role multi page",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				PageSize: 1,
				Role:     "role-01",
			}.Build(),
			expect: [][]string{
				{"alice-01"},
				{"bob-01"},
			},
		},
		{
			name: "role nonexistent",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				Role: "role-99",
			}.Build(),
			expect: nil,
		},
		{
			name: "resource scope root",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/",
				}.Build(),
			}.Build(),
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
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/aa",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"alice-02",
					"bob-02",
				},
			},
		},
		{
			name: "policy scope root",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"alice-01",
					"bob-01",
				},
			},
		},
		{
			name: "policy scope non-root",
			req: scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/aa",
				}.Build(),
			}.Build(),
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

	cache := NewAssignmentCache(AssignmentCacheConfig{})
	for _, assignment := range assignments {
		_, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(assignment)

		rsp, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, rsp.GetAssignment())
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			var out [][]string
			for {
				rsp, err := cache.ListScopedRoleAssignments(t.Context(), tt.req)
				require.NoError(t, err)

				if len(rsp.GetAssignments()) == 0 {
					break
				}

				out = append(out, collectAssignmentNames(rsp.GetAssignments()))

				if rsp.GetNextPageToken() == "" {
					break
				}

				tt.req.SetPageToken(rsp.GetNextPageToken())
			}

			require.Equal(t, tt.expect, out)
		})
	}
}

// TestScopedRoleAssignmentSubKinds asserts that assignments with different
// subkinds are properly fetched and listed with pagination, even if their
// names collide.
func TestScopedRoleAssignmentSubKinds(t *testing.T) {
	t.Parallel()

	cache := NewAssignmentCache(AssignmentCacheConfig{})

	makeAssignment := func(name, subKind, user string) *scopedaccessv1.ScopedRoleAssignment {
		return scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: subKind,
			Metadata: headerpb.Metadata_builder{
				Name: name,
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: user,
				Assignments: []*scopedaccessv1.Assignment{scopedaccessv1.Assignment_builder{
					Role:  "role-01",
					Scope: "/foo",
				}.Build()},
			}.Build(),
			Version: types.V1,
		}.Build()
	}

	// Populate the cache with a dynamic and a materialized assignment with the same name.
	dynamic := makeAssignment("shared-name", scopedaccess.SubKindDynamic, "alice")
	materialized := makeAssignment("shared-name", scopedaccess.SubKindMaterialized, "bob")
	require.NoError(t, cache.Put(dynamic))
	require.NoError(t, cache.Put(materialized))

	// Make sure getting each assignment by (name, subkind) returns the right one.
	for _, tt := range []*scopedaccessv1.ScopedRoleAssignment{dynamic, materialized} {
		rsp, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    tt.GetMetadata().GetName(),
			SubKind: tt.GetSubKind(),
		}.Build())
		require.NoError(t, err)
		require.Equal(t, tt.GetSpec().GetUser(), rsp.GetAssignment().GetSpec().GetUser())
		require.Equal(t, tt.GetSubKind(), rsp.GetAssignment().GetSubKind())
	}

	// Trying to get an assignment without specifying a subkind returns NotFound.
	_, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name: "shared-name",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// Make sure paging across assignments does not skip or duplicate
	// assignments with matching names but different subkinds.
	var got []string
	pageToken := ""
	for {
		rsp, err := cache.ListScopedRoleAssignments(t.Context(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
			PageSize:  1,
			PageToken: pageToken,
		}.Build())
		require.NoError(t, err)
		require.Len(t, rsp.GetAssignments(), 1)

		assignment := rsp.GetAssignments()[0]
		got = append(got, assignment.GetMetadata().GetName()+"/"+assignment.GetSubKind())

		if rsp.GetNextPageToken() == "" {
			break
		}
		pageToken = rsp.GetNextPageToken()
	}

	require.ElementsMatch(t,
		[]string{
			"shared-name/dynamic",
			"shared-name/materialized",
		},
		got,
	)
}

func collectAssignmentNames(assignments []*scopedaccessv1.ScopedRoleAssignment) []string {
	var names []string
	for _, assignment := range assignments {
		names = append(names, assignment.GetMetadata().GetName())
	}
	return names
}
