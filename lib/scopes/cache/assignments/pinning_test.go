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
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/aa/bb",
			}.Build(),
			ok: true,
			expect: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/aa/bb",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":   {"/aa": {"role-01"}},
					"/aa": {"/aa": {"role-03"}, "/aa/bb": {"role-04"}},
				}),
			}.Build(),
		},
		{
			name: "ancestral",
			user: "alice",
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/",
			}.Build(),
			ok: true,
			expect: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":      {"/aa": {"role-01"}, "/bb": {"role-02"}},
					"/aa":    {"/aa": {"role-03"}, "/aa/bb": {"role-04"}},
					"/aa/bb": {"/aa/bb": {"role-05"}, "/aa/bb/cc": {"role-06"}},
				}),
			}.Build(),
		},
		{
			name: "orthogonal",
			user: "carol",
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/xx",
			}.Build(),
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
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-root",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: "root-role", Scope: "/staging"}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-staging",
			}.Build(),
			Scope: "/staging",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: "staging-role", Scope: "/staging"}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "alice-staging-west",
			}.Build(),
			Scope: "/staging/west",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: "west-role", Scope: "/staging/west"}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	for _, assignment := range assignments {
		err := cache.Put(assignment)
		require.NoError(t, err)
	}

	pin := scopesv1.Pin_builder{
		Kind:  scopesv1.PinKind_PIN_KIND_USER,
		Scope: "/staging/west",
	}.Build()
	err := cache.PopulatePinnedAssignmentsForUser(t.Context(), "alice", pin)
	require.NoError(t, err)

	// verify the tree was populated
	require.NotNil(t, pin.GetAssignmentTree())

	// verify the tree fits within the size limit
	treeSize := proto.Size(pin.GetAssignmentTree())
	require.LessOrEqual(t, treeSize, cache.cfg.MaxAssignmentTreeBytes,
		"assignment tree should be pruned to fit within configured limit")

	// verify that the pruned tree retained the most important assignment
	expectedTree := map[string]map[string][]string{
		"/": {
			"/staging": {"root-role"},
		},
	}
	actualTree := pinning.AssignmentTreeIntoMap(pin.GetAssignmentTree())
	require.Equal(t, expectedTree, actualTree,
		"pruning should preserve highest authority assignments (root level)")
}

func TestPopulatePinnedAssignmentsForBot(t *testing.T) {
	t.Parallel()

	bernardScope := "/aa"
	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		// Very "normal" assignment. Scope of SRA matches Bot scope, and
		// assigned scope is same.
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-01",
			}.Build(),
			Scope: bernardScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-01",
						Scope: bernardScope,
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		// Assignment to child-scope of main scope
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-02",
			}.Build(),
			Scope: bernardScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-02",
						Scope: bernardScope + "/child",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		// SRA in parent scope, assigning to bot scope.
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-03",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-03",
						Scope: bernardScope,
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		// SRA in parent scope, assigning to bot's child scope
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-04",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "role-04",
						Scope: bernardScope + "/child",
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		// `bot_scope` mismatches bot's actual scope - this should be ignored.
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-invalid-01",
			}.Build(),
			Scope: bernardScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: "/mismatched",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "bernard-invalid-01",
						Scope: bernardScope,
					}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
		// SRA above bot scope ignored.
		// nb: we may eventually loosen this to behave more like users.
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerpb.Metadata_builder{
				Name: "bernard-invalid-02",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				BotName:  "bernard",
				BotScope: bernardScope,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "bernard-invalid-02",
						Scope: "/",
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
			SubKind: scopedaccess.SubKindDynamic,
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

		require.NoError(t, cache.Put(assignment))

		rsp, err := cache.GetScopedRoleAssignment(t.Context(), scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    assignment.GetMetadata().GetName(),
			SubKind: scopedaccess.SubKindDynamic,
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, rsp.GetAssignment())
	}

	tts := []struct {
		name        string
		botName     string
		botScope    string
		pin         *scopesv1.Pin
		errContains string
		expect      *scopesv1.Pin
	}{
		{
			name:     "standard",
			botName:  "bernard",
			botScope: bernardScope,
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: bernardScope,
			}.Build(),
			expect: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: bernardScope,
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":          {bernardScope: {"role-03"}, bernardScope + "/child": {"role-04"}},
					bernardScope: {bernardScope: {"role-01"}, bernardScope + "/child": {"role-02"}},
				}),
			}.Build(),
		},
		{
			name:     "pin scope at child of bot scope",
			botName:  "bernard",
			botScope: bernardScope,
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: bernardScope + "/child",
			}.Build(),
			expect: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: bernardScope + "/child",
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/":          {bernardScope: {"role-03"}, bernardScope + "/child": {"role-04"}},
					bernardScope: {bernardScope: {"role-01"}, bernardScope + "/child": {"role-02"}},
				}),
			}.Build(),
		},
		{
			name:     "pin scope outside bot scope",
			botName:  "bernard",
			botScope: bernardScope,
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/bb",
			}.Build(),
			errContains: "is not subject to bot scope",
		},
		{
			name:     "no matching assignments",
			botName:  "no-such-bot",
			botScope: "/aa",
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/aa",
			}.Build(),
			errContains: "no scoped role assignments found",
		},
		{
			name:     "empty bot name",
			botScope: "/aa",
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/aa",
			}.Build(),
			errContains: "missing bot name",
		},
		{
			name:    "empty bot scope",
			botName: "bernard",
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: "/aa",
			}.Build(),
			errContains: "missing bot scope",
		},
		{
			name:     "pin with pre-existing assignment tree",
			botName:  "bernard",
			botScope: bernardScope,
			pin: scopesv1.Pin_builder{
				Kind:  scopesv1.PinKind_PIN_KIND_USER,
				Scope: bernardScope,
				AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {bernardScope: {"role-03"}},
				}),
			}.Build(),
			errContains: "already contains an assignment tree",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			pin := proto.CloneOf(tt.pin)
			err := cache.PopulatePinnedAssignmentsForBot(
				t.Context(), tt.botName, tt.botScope, pin,
			)
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, pin)
			require.Empty(t, cmp.Diff(pin, tt.expect, protocmp.Transform()))
		})
	}
}
