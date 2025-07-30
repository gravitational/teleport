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
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopespb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

// TestListScopedRolesScenarios tests particular more tricky ListScopedRoles scenarios, such
// as attempts to use an out-of-band cursor to read values outside of the target scope.
func TestListScopedRoleASsignmentsScenarios(t *testing.T) {
	t.Parallel()

	roles := []*accessv1.ScopedRole{
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "admin-01",
			},
			Scope:   "/",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "admin-02",
			},
			Scope:   "/aa",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "user-01",
			},
			Scope:   "/",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "user-02",
			},
			Scope:   "/aa",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "admin-03",
			},
			Scope:   "/aa/bb",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "user-03",
			},
			Scope:   "/aa/bb",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "intern-01",
			},
			Scope:   "/bb",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
	}

	cache := NewRoleCache()
	for _, role := range roles {
		cache.Put(role)
	}

	// verify expected behavior for standard cursors in resources subject to scope mode
	rsp, err := cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:user-02@/aa",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"user-02", "admin-03", "user-03"}, collectRoleNames(rsp.Roles))

	// try to inject a malicious root out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:user-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"admin-02", "user-02", "admin-03", "user-03"}, collectRoleNames(rsp.Roles))

	// try to inject a malicious orthogonal out-of-band cursor in resources subject to scope mode (no effect)
	rsp, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:intern-01@/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"admin-02", "user-02", "admin-03", "user-03"}, collectRoleNames(rsp.Roles))

	// verify expected behavior for standard cursors in policies applicable to scope mode
	rsp, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:user-01@/",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"user-01", "admin-02", "user-02"}, collectRoleNames(rsp.Roles))

	// try to inject a malicious child out-of-band cursor in policies applicable to scope mode (effect is
	// to ignore all items in valid query path).
	rsp, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:user-03@/aa/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Empty(t, collectRoleNames(rsp.Roles))

	// try to inject a malicious orthogonal out-of-band cursor in policies applicable to scope mode (effect is to
	// ignore root, but process leaf normally).
	rsp, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v1:intern-01@/bb",
	})
	require.NoError(t, err)
	require.Empty(t, rsp.NextPageToken)
	require.Equal(t, []string{"admin-02", "user-02"}, collectRoleNames(rsp.Roles))

	// verify rejection of unknown cursor version
	_, err = cache.ListScopedRoles(t.Context(), &accessv1.ListScopedRolesRequest{
		ResourceScope: &scopespb.Filter{
			Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
			Scope: "/aa",
		},
		PageToken: "v2:user-02@/aa",
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestListScopedRolesBasics tests some basic functionality of the ListScopedRoles method.
func TestListScopedRolesBasics(t *testing.T) {
	t.Parallel()

	roles := []*accessv1.ScopedRole{
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "admin-01",
			},
			Scope:   "/",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "admin-02",
			},
			Scope:   "/aa",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "user-01",
			},
			Scope:   "/",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "user-02",
			},
			Scope:   "/aa",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "intern-01",
			},
			Scope:   "/bb",
			Spec:    &accessv1.ScopedRoleSpec{},
			Version: types.V1,
		},
	}

	tts := []struct {
		name   string
		req    *accessv1.ListScopedRolesRequest
		expect [][]string
	}{
		{
			name: "all single page explicit excess",
			req: &accessv1.ListScopedRolesRequest{
				PageSize: int32(len(roles) + 1),
			},
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
			req:  &accessv1.ListScopedRolesRequest{},
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
			req: &accessv1.ListScopedRolesRequest{
				PageSize: int32(len(roles)),
			},
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
			req: &accessv1.ListScopedRolesRequest{
				PageSize: 2,
			},
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
			req: &accessv1.ListScopedRolesRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/",
				},
			},
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
			req: &accessv1.ListScopedRolesRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE,
					Scope: "/aa",
				},
			},
			expect: [][]string{
				{
					"admin-02",
					"user-02",
				},
			},
		},
		{
			name: "policy scope root",
			req: &accessv1.ListScopedRolesRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/",
				},
			},
			expect: [][]string{
				{
					"admin-01",
					"user-01",
				},
			},
		},
		{
			name: "policy scope non-root",
			req: &accessv1.ListScopedRolesRequest{
				ResourceScope: &scopespb.Filter{
					Mode:  scopespb.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE,
					Scope: "/aa",
				},
			},
			expect: [][]string{
				{
					"admin-01",
					"user-01",
					"admin-02",
					"user-02",
				},
			},
		},
	}

	cache := NewRoleCache()
	for _, role := range roles {
		cache.Put(role)
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			var out [][]string
			for {
				rsp, err := cache.ListScopedRoles(t.Context(), tt.req)
				require.NoError(t, err)

				if len(rsp.Roles) == 0 {
					break
				}

				out = append(out, collectRoleNames(rsp.Roles))

				if rsp.NextPageToken == "" {
					break
				}

				tt.req.PageToken = rsp.NextPageToken
			}

			require.Equal(t, tt.expect, out)
		})
	}
}

func collectRoleNames(roles []*accessv1.ScopedRole) []string {
	var names []string
	for _, role := range roles {
		names = append(names, role.Metadata.Name)
	}
	return names
}
