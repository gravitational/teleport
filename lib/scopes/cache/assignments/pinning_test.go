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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestPopulatePinnedAssignmentsForUser verifies the basic expected behavior of scope pin assignment population.
func TestPopulatePinnedAssignmentsForUser(t *testing.T) {
	t.Parallel()

	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-01",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-02",
			},
			Scope: "/aa",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-01",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-02",
			},
			Scope: "/aa",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "alice-03",
			},
			Scope: "/aa/bb",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bob-03",
			},
			Scope: "/aa/bb",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
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
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "carol-01",
			},
			Scope: "/bb",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "carol",
				Assignments: []*scopedaccessv1.Assignment{
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
		_, err := cache.GetScopedRoleAssignment(t.Context(), &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name: assignment.GetMetadata().GetName(),
		})
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(assignment)

		rsp, err := cache.GetScopedRoleAssignment(t.Context(), &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name: assignment.GetMetadata().GetName(),
		})
		require.NoError(t, err)
		require.NotNil(t, rsp.GetAssignment())
	}

	tts := []struct {
		name   string
		user   string
		pin    *scopesv1.Pin
		ok     bool
		expect *scopesv1.Pin
	}{
		{
			name: "descendant",
			user: "bob",
			pin: &scopesv1.Pin{
				Scope: "/aa/bb",
			},
			ok: true,
			expect: &scopesv1.Pin{
				Scope: "/aa/bb",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/aa": {
						Roles: []string{"role-01", "role-03"},
					},
					"/aa/bb": {
						Roles: []string{"role-04"},
					},
				},
			},
		},
		{
			name: "ancestral",
			user: "alice",
			pin: &scopesv1.Pin{
				Scope: "/",
			},
			ok: true,
			expect: &scopesv1.Pin{
				Scope: "/",
				Assignments: map[string]*scopesv1.PinnedAssignments{
					"/aa": {
						Roles: []string{"role-01", "role-03"},
					},
					"/aa/bb": {
						Roles: []string{"role-04", "role-05"},
					},
					"/aa/bb/cc": {
						Roles: []string{"role-06"},
					},
					"/bb": {
						Roles: []string{"role-02"},
					},
				},
			},
		},
		{
			name: "orthogonal",
			user: "carol",
			pin: &scopesv1.Pin{
				Scope: "/xx",
			},
			ok: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			pin := proto.CloneOf(tt.pin)
			err := cache.PopulatePinnedAssignmentsForUser(t.Context(), tt.user, pin)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}
			require.NotNil(t, pin)
			require.Empty(t, cmp.Diff(pin, tt.expect, protocmp.Transform()))
		})
	}
}
