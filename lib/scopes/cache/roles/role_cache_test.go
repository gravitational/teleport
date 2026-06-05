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

package roles

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

// TestListScopedRolesScenarios tests particular more tricky ListScopedRoles scenarios, such
// as attempts to use an out-of-band cursor to read values outside of the target scope.
func TestListScopedRolesScenarios(t *testing.T) {
	t.Parallel()

	roles := []*scopedaccessv1.ScopedRole{
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "admin-01",
			}.Build(),
			Scope:   "/",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "admin-02",
			}.Build(),
			Scope:   "/aa",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "user-01",
			}.Build(),
			Scope:   "/",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "user-02",
			}.Build(),
			Scope:   "/aa",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "admin-03",
			}.Build(),
			Scope:   "/aa/bb",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "user-03",
			}.Build(),
			Scope:   "/aa/bb",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "intern-01",
			}.Build(),
			Scope:   "/bb",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
	}

	cache := NewRoleCache()
	for _, role := range roles {
		_, err := cache.GetScopedRole(t.Context(), scopedaccessv1.GetScopedRoleRequest_builder{
			Name: role.GetMetadata().GetName(),
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(role)

		rsp, err := cache.GetScopedRole(t.Context(), scopedaccessv1.GetScopedRoleRequest_builder{
			Name: role.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, rsp.GetRole())
	}

	// verify expected behavior for standard cursors in resources subject to scope mode
	rsp, err := cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:user-02@/aa",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"user-02", "admin-03", "user-03"}, collectRoleNames(rsp.GetRoles()))

	// try to inject a malicious root out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:user-01@/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"admin-02", "user-02", "admin-03", "user-03"}, collectRoleNames(rsp.GetRoles()))

	// try to inject a malicious orthogonal out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:intern-01@/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"admin-02", "user-02", "admin-03", "user-03"}, collectRoleNames(rsp.GetRoles()))

	// verify expected behavior for standard cursors in policies applicable to scope mode
	rsp, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:user-01@/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"user-01", "admin-02", "user-02"}, collectRoleNames(rsp.GetRoles()))

	// try to inject a malicious child out-of-band cursor in policies applicable to scope mode (effect is
	// to ignore all items in valid query path).
	rsp, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:user-03@/aa/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Empty(t, collectRoleNames(rsp.GetRoles()))

	// try to inject a malicious orthogonal out-of-band cursor in policies applicable to scope mode (effect is to
	// ignore root, but process leaf normally).
	rsp, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v1:intern-01@/bb",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, rsp.GetNextPageToken())
	require.Equal(t, []string{"admin-02", "user-02"}, collectRoleNames(rsp.GetRoles()))

	// verify rejection of unknown cursor version
	_, err = cache.ListScopedRoles(t.Context(), scopedaccessv1.ListScopedRolesRequest_builder{
		ResourceScope: scopespb.Filter_builder{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		}.Build(),
		PageToken: "v2:user-02@/aa",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestListScopedRolesBasics tests some basic functionality of the ListScopedRoles method.
func TestListScopedRolesBasics(t *testing.T) {
	t.Parallel()

	roles := []*scopedaccessv1.ScopedRole{
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "admin-01",
			}.Build(),
			Scope:   "/",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "admin-02",
			}.Build(),
			Scope:   "/aa",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "user-01",
			}.Build(),
			Scope:   "/",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "user-02",
			}.Build(),
			Scope:   "/aa",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerpb.Metadata_builder{
				Name: "intern-01",
			}.Build(),
			Scope:   "/bb",
			Spec:    &scopedaccessv1.ScopedRoleSpec{},
			Version: types.V1,
		}.Build(),
	}

	tts := []struct {
		name   string
		req    *scopedaccessv1.ListScopedRolesRequest
		expect [][]string
	}{
		{
			name: "all single page explicit excess",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				PageSize: int32(len(roles) + 1),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
					"intern-01",
				},
			},
		},
		{
			name: "all single page implicit excess",
			req:  &scopedaccessv1.ListScopedRolesRequest{},
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
					"intern-01",
				},
			},
		},
		{
			name: "all single page exact",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				PageSize: int32(len(roles)),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
					"intern-01",
				},
			},
		},
		{
			name: "all multi page",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				PageSize: 2,
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
				},
				{
					"admin-02",
					"user-02",
				},
				{
					"intern-01",
				},
			},
		},
		{
			name: "resource scope root",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
					"intern-01",
				},
			},
		},
		{
			name: "resource scope non-root",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/aa",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"admin-02",
					"user-02",
				},
			},
		},
		{
			name: "policy scope root",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
				},
			},
		},
		{
			name: "policy scope non-root",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/aa",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
				},
			},
		},
		{
			name: "name filter",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				NameFilter: "admin",
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"admin-02",
				},
			},
		},
		{
			name: "name case insensitive",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				NameFilter: "AdMiN",
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"admin-02",
				},
			},
		},
		{
			name: "name filter paged",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				NameFilter: "min",
				PageSize:   1,
			}.Build(),
			expect: [][]string{
				{"admin-01"},
				{"admin-02"},
			},
		},
		{
			name: "name filter policy scope root",
			req: scopedaccessv1.ListScopedRolesRequest_builder{
				NameFilter: "01",
				ResourceScope: scopespb.Filter_builder{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/",
				}.Build(),
			}.Build(),
			expect: [][]string{
				{
					"admin-01",
					"user-01",
				},
			},
		},
	}

	cache := NewRoleCache()
	for _, role := range roles {
		_, err := cache.GetScopedRole(t.Context(), scopedaccessv1.GetScopedRoleRequest_builder{
			Name: role.GetMetadata().GetName(),
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(role)

		rsp, err := cache.GetScopedRole(t.Context(), scopedaccessv1.GetScopedRoleRequest_builder{
			Name: role.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, rsp.GetRole())
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			var out [][]string
			for {
				rsp, err := cache.ListScopedRoles(t.Context(), tt.req)
				require.NoError(t, err)

				if len(rsp.GetRoles()) == 0 {
					break
				}

				out = append(out, collectRoleNames(rsp.GetRoles()))

				if rsp.GetNextPageToken() == "" {
					break
				}

				tt.req.SetPageToken(rsp.GetNextPageToken())
			}

			require.Equal(t, tt.expect, out)
		})
	}
}

func collectRoleNames(roles []*scopedaccessv1.ScopedRole) []string {
	var names []string
	for _, role := range roles {
		names = append(names, role.GetMetadata().GetName())
	}
	return names
}
