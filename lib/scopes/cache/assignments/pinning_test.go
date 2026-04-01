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
	"github.com/gravitational/teleport/lib/scopes/pinning"
)

// TestPopulatePinnedAssignmentsForUser verifies the basic expected behavior of scope pin assignment population.
func TestPopulatePinnedAssignmentsForUser(t *testing.T) {
	t.Parallel()

	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
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

	cache := NewAssignmentCache(AssignmentCacheConfig{})
	for _, assignment := range assignments {
		_, err := cache.GetScopedRoleAssignment(t.Context(), &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
		})
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		cache.Put(assignment)

		rsp, err := cache.GetScopedRoleAssignment(t.Context(), &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: assignment.GetSubKind(),
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
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":   {"/aa": {"role-01"}},
					"/aa": {"/aa": {"role-03"}, "/aa/bb": {"role-04"}},
				}),
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
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":      {"/aa": {"role-01"}, "/bb": {"role-02"}},
					"/aa":    {"/aa": {"role-03"}, "/aa/bb": {"role-04"}},
					"/aa/bb": {"/aa/bb": {"role-05"}, "/aa/bb/cc": {"role-06"}},
				}),
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

// TestAssignmentTreePruning verifies that the assignment cache actually prunes oversized assignment trees
// as expected. See [pinning.PruneAssignmentTree] for detailed discussion of assignment tree pruning.
func TestAssignmentTreePruning(t *testing.T) {
	t.Parallel()

	// set up a cache with a very small max tree size in order to verify pruning behavior
	cache := NewAssignmentCache(AssignmentCacheConfig{
		MaxAssignmentTreeBytes: 50,
	})

	// set up some initial assignments. pruning operates at the scope of origin level, so assignments
	// must have a mix of resource scopes to provide a natural pruning boundary.
	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: &headerpb.Metadata{
				Name: "alice-root",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "root-role", Scope: "/staging"},
				},
			},
			Version: types.V1,
		},
		{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: &headerpb.Metadata{
				Name: "alice-staging",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "staging-role", Scope: "/staging"},
				},
			},
			Version: types.V1,
		},
		{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: &headerpb.Metadata{
				Name: "alice-staging-west",
			},
			Scope: "/staging/west",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "west-role", Scope: "/staging/west"},
				},
			},
			Version: types.V1,
		},
	}

	for _, assignment := range assignments {
		err := cache.Put(assignment)
		require.NoError(t, err)
	}

	pin := &scopesv1.Pin{
		Scope: "/staging/west",
	}
	err := cache.PopulatePinnedAssignmentsForUser(t.Context(), "alice", pin)
	require.NoError(t, err)

	// verify the tree was populated
	require.NotNil(t, pin.AssignmentTree)

	// verify the tree fits within the size limit
	treeSize := proto.Size(pin.AssignmentTree)
	require.LessOrEqual(t, treeSize, cache.cfg.MaxAssignmentTreeBytes,
		"assignment tree should be pruned to fit within configured limit")

	// verify that the pruned tree retained the most important assignment
	expectedTree := map[string]map[string][]string{
		"/": {
			"/staging": {"root-role"},
		},
	}
	actualTree := pinning.AssignmentTreeIntoMap(pin.AssignmentTree)
	require.Equal(t, expectedTree, actualTree,
		"pruning should preserve highest authority assignments (root level)")
}

func TestPopulatePinnedAssignmentsForBot(t *testing.T) {
	t.Parallel()

	bernardScope := "/aa"
	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		// Very "normal" assignment. Scope of SRA matches Bot scope, and
		// assigned scope is same.
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-01",
			},
			Scope: bernardScope,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "role-01",
						Scope: bernardScope,
					},
				},
			},
			Version: types.V1,
		},
		// Assignment to child-scope of main scope
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-02",
			},
			Scope: bernardScope,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "role-02",
						Scope: bernardScope + "/child",
					},
				},
			},
			Version: types.V1,
		},
		// SRA in parent scope, assigning to bot scope.
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-03",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "role-03",
						Scope: bernardScope,
					},
				},
			},
			Version: types.V1,
		},
		// SRA in parent scope, assigning to bot's child scope
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-04",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "role-04",
						Scope: bernardScope + "/child",
					},
				},
			},
			Version: types.V1,
		},
		// `bot_scope` mismatches bot's actual scope - this should be ignored.
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-invalid-01",
			},
			Scope: bernardScope,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: "/mismatched",
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "bernard-invalid-01",
						Scope: bernardScope,
					},
				},
			},
			Version: types.V1,
		},
		// Scope of effect above Bot Scope is ignored.
		// nb: maybe incorrect? discussion with forrest april 1st.
		{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerpb.Metadata{
				Name: "bernard-invalid-02",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "bernard-invalid-02",
						Scope: "/",
					},
				},
			},
			Version: types.V1,
		},
	}

	cache := NewAssignmentCache(AssignmentCacheConfig{})
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
		name     string
		botName  string
		botScope string
		pin      *scopesv1.Pin
		ok       bool
		expect   *scopesv1.Pin
	}{
		{
			name:     "standard",
			botName:  "bernard",
			botScope: bernardScope,
			pin: &scopesv1.Pin{
				Scope: bernardScope,
			},
			ok: true,
			expect: &scopesv1.Pin{
				Scope: bernardScope,
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":          {bernardScope: {"role-03"}, bernardScope + "/child": {"role-04"}},
					bernardScope: {bernardScope: {"role-01"}, bernardScope + "/child": {"role-02"}},
				}),
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			pin := proto.CloneOf(tt.pin)
			err := cache.PopulatePinnedAssignmentsForBot(
				t.Context(), tt.botName, tt.botScope, pin,
			)
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
