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

package local

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend/memory"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestScopedRoleEvents verifies the expected behavior of backend events for the ScopedRole family of types.
func TestScopedRoleEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedAccessService(backend)

	events := NewEventsService(backend)

	watcher, err := events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind: scopedaccess.KindScopedRole,
			},
			{
				Kind: scopedaccess.KindScopedRoleAssignment,
			},
		},
	})
	require.NoError(t, err)
	defer watcher.Close()

	getNextEvent := func() types.Event {
		select {
		case event := <-watcher.Events():
			return event
		case <-watcher.Done():
			require.FailNow(t, "Watcher exited with error", watcher.Error())
		case <-time.After(time.Second * 5):
			require.FailNow(t, "Timeout waiting for event", watcher.Error())
		}

		panic("unreachable")
	}

	event := getNextEvent()
	require.Equal(t, types.OpInit, event.Type)

	// Create a ScopedRole and verify create event is well-formed.
	role := &scopedaccessv1.ScopedRole{
		Kind: scopedaccess.KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: "test-role",
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/foo", "/bar"},
		},
		Version: types.V1,
	}

	crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: role,
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)

	resource := (event.Resource).(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT()
	require.Empty(t, cmp.Diff(crsp.Role, resource, protocmp.Transform() /* deliberately not ignoring revision */))

	// delete the role and verify delete event is well-formed.
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: role.Metadata.Name,
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)

	require.Empty(t, cmp.Diff(&types.ResourceHeader{
		Kind: scopedaccess.KindScopedRole,
		Metadata: types.Metadata{
			Name: role.Metadata.Name,
		},
	}, event.Resource.(*types.ResourceHeader), protocmp.Transform()))

	// recreate scoped role so that we can use it for testing assignment events
	crsp, err = service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: role,
	})
	require.NoError(t, err)

	_ = getNextEvent() // drain the role create event

	assignment := &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  role.Metadata.Name,
					Scope: "/foo",
				},
			},
		},
		Version: types.V1,
	}

	acrsp, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment,
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)
	assignmentResource := (event.Resource).(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]).UnwrapT()
	require.Empty(t, cmp.Diff(acrsp.Assignment, assignmentResource, protocmp.Transform() /* deliberately not ignoring revision */))

	// delete the assignment and verify delete event is well-formed.
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: assignment.Metadata.Name,
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)

	require.Empty(t, cmp.Diff(&types.ResourceHeader{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: types.Metadata{
			Name: assignment.Metadata.Name,
		},
	}, event.Resource.(*types.ResourceHeader), protocmp.Transform()))
}

// TestScopedRoleBasicCRUD tests the basic CRUD operations of the ScopedAccessService, excluding the more non-trivial
// scenarios involving roles with active assignments, which are tested separately.
func TestScopedRoleBasicCRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedAccessService(backend)

	basicRoles := []*scopedaccessv1.ScopedRole{
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "basic-01",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "basic-02",
			},
			Scope: "/bar",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/bar/**"},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "basic-03",
			},
			Scope: "/baz",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/baz/**"},
			},
			Version: types.V1,
		},
	}

	var revisions []string

	// verify the expected behavior of CreateScopedRole
	for _, role := range basicRoles {
		crsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
		require.NotEmpty(t, crsp.Role.Metadata.Revision)
		require.Empty(t, cmp.Diff(role, crsp.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

		// Check that the role can be retrieved.
		grsp, err := service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: role.Metadata.Name,
		})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(crsp.Role, grsp.Role, protocmp.Transform() /* deliberately not ignoring revision */))

		revisions = append(revisions, grsp.Role.Metadata.Revision)
	}

	require.Len(t, revisions, len(basicRoles))

	// verify that create fails if the role already exists
	_, err = service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: basicRoles[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify a basic allowable update
	basic01Mod := apiutils.CloneProtoMsg(basicRoles[0])
	basic01Mod.Spec.AssignableScopes = []string{"/foo", "/bar"}
	basic01Mod.Metadata.Revision = revisions[0]

	ursp, err := service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.NoError(t, err)
	require.NotEmpty(t, ursp.Role.Metadata.Revision)
	require.Empty(t, cmp.Diff(basic01Mod, ursp.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify that update really happened
	grsp, err := service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: basic01Mod.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ursp.Role, grsp.Role, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that update fails if the revision is wrong
	_, err = service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that update is rejected if the role's scope is changed
	basic01Mod = apiutils.CloneProtoMsg(ursp.Role)
	basic01Mod.Scope = "/foo"

	_, err = service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that update fails if the role does not exist
	_, err = service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name:     "non-existent",
				Revision: revisions[0],
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete fails if the role does not exist
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: "non-existent",
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete fails if the revision does not match
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     basicRoles[0].Metadata.Name,
		Revision: revisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify successful unconditional delete
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: basicRoles[0].Metadata.Name,
	})
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: basicRoles[0].Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify successful conditional delete
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     basicRoles[1].Metadata.Name,
		Revision: revisions[1],
	})
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: basicRoles[1].Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify upsert creates when role does not exist
	basic04 := &scopedaccessv1.ScopedRole{
		Kind: scopedaccess.KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: "basic-04",
		},
		Scope: "/qux",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/qux"},
		},
		Version: types.V1,
	}
	uprsp, err := service.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: basic04,
	})
	require.NoError(t, err)
	require.NotEmpty(t, uprsp.Role.Metadata.Revision)
	require.Empty(t, cmp.Diff(basic04, uprsp.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify upsert updates when role already exists (including with a stale/wrong revision)
	basic04Mod := apiutils.CloneProtoMsg(uprsp.Role)
	basic04Mod.Spec.AssignableScopes = []string{"/qux", "/qux/sub"}
	basic04Mod.Metadata.Revision = revisions[2] // deliberately stale revision

	uprsp2, err := service.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: basic04Mod,
	})
	require.NoError(t, err, "upsert should succeed despite stale revision")
	require.Empty(t, cmp.Diff(basic04Mod, uprsp2.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify upsert rejects scope change
	basic04ScopeChange := apiutils.CloneProtoMsg(uprsp2.Role)
	basic04ScopeChange.Scope = "/other"
	basic04ScopeChange.Spec.AssignableScopes = []string{"/other"}
	_, err = service.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: basic04ScopeChange,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestScopedRoleAssignmentBasicCRD tests the basic CRD operations of the ScopedRoleAssignmentService, excluding the more non-trivial
// scenarios involving roles with active assignments, which are tested separately.
func TestScopedRoleAssignmentBasicCRD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedAccessService(backend)

	roles := []*scopedaccessv1.ScopedRole{
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "role-01",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo", "/bar"},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "role-02",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "role-03",
			},
			Scope: "/foo",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
	}

	var roleRevisions []string

	// Create the roles.
	for _, role := range roles {
		rsp, err := service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)

		roleRevisions = append(roleRevisions, rsp.Role.Metadata.Revision)
	}

	// basic root assignment to test standard CRD operations with
	assignment01 := &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  "role-02",
					Scope: "/", // root scope of effect is not permitted
				},
			},
		},
		Version: types.V1,
	}

	// check that root scope of effect is rejected
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that a sub-assignment scope outside the assignment's resource scope is rejected
	assignment01.Spec.Assignments[0].Scope = "/bar" // non-root, but outside resource scope /foo
	assignment01.Scope = "/foo"
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that a valid assignment (resource scope encompasses the sub-assignment scope) succeeds
	assignment01.Scope = "/" // fix resource scope
	crsp, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// Check that the assignment can be retrieved.
	grsp, err := service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: assignment01.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that create fails if the assignment already exists
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify a basic allowable update
	assignment01Mod := apiutils.CloneProtoMsg(crsp.Assignment)
	assignment01Mod.Spec.Assignments[0].Scope = "/foo"
	assignment01Mod.Metadata.Revision = crsp.Assignment.Metadata.Revision

	ursp, err := service.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: assignment01Mod,
	})
	require.NoError(t, err)
	require.NotEmpty(t, ursp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(assignment01Mod, ursp.Assignment, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify that update really happened
	grsp, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: assignment01Mod.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ursp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that update fails if the revision is wrong (stale revision from before the update)
	_, err = service.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: assignment01Mod,
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that update is rejected if the assignment's resource scope is changed
	assignment01ScopeChange := apiutils.CloneProtoMsg(ursp.Assignment)
	assignment01ScopeChange.Scope = "/foo"
	_, err = service.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: assignment01ScopeChange,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that update fails if the assignment does not exist
	_, err = service.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerv1.Metadata{
				Name:     "00000000-0000-0000-0000-000000000000",
				Revision: crsp.Assignment.Metadata.Revision,
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "role-02", Scope: "/foo"},
				},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with incorrect revision fails
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: roleRevisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with correct revision works
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: ursp.Assignment.Metadata.Revision,
	})
	require.NoError(t, err)

	// verify that the assignment is gone
	_, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: assignment01.Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// set up a more non-trivial assignment with multiple sub-assignments
	// assignment02 mixes roles from different resource scopes. cross-resource consistency (e.g. whether
	// a role at a given resource scope is accessible from the assignment's resource scope) is not enforced
	// at write time; it is enforced exclusively at the policy decision point.
	assignment02 := &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "bob",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  "role-01",
					Scope: "/foo",
				},
				{
					Role:  "role-02",
					Scope: "/foo/bar",
				},
				{
					Role:  "role-03", // resource scope /foo, different from assignment resource scope /
					Scope: "/foo",
				},
			},
		},
		Version: types.V1,
	}

	crsp, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment02, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// Check that the assignment can be retrieved
	grsp, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: assignment02.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// create an assignment that assigns the same role at multiple separate scopes (covers a specific
	// bug where original impl would construct invalid conditional actions when multiple sub-assignments
	// are made for the same role).
	assignment03 := &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "carol",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  "role-01",
					Scope: "/foo",
				},
				{
					Role:  "role-01",
					Scope: "/bar",
				},
			},
		},
		Version: types.V1,
	}

	// check that creation of assignment works
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment03,
	})
	require.NoError(t, err)

	// verify that deletion of assignment works
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: assignment03.Metadata.Name,
	})
	require.NoError(t, err)

	// verify upsert creates when assignment does not exist
	assignment04 := &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "dave",
			Assignments: []*scopedaccessv1.Assignment{
				{Role: "role-01", Scope: "/foo"},
			},
		},
		Version: types.V1,
	}
	uaprsp, err := service.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: assignment04,
	})
	require.NoError(t, err)
	require.NotEmpty(t, uaprsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(assignment04, uaprsp.Assignment, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify upsert updates when assignment already exists (including with a stale/wrong revision)
	assignment04Mod := apiutils.CloneProtoMsg(uaprsp.Assignment)
	assignment04Mod.Spec.Assignments = append(assignment04Mod.Spec.Assignments, &scopedaccessv1.Assignment{
		Role: "role-02", Scope: "/foo",
	})
	assignment04Mod.Metadata.Revision = roleRevisions[0] // deliberately stale revision

	uaprsp2, err := service.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: assignment04Mod,
	})
	require.NoError(t, err, "upsert should succeed despite stale revision")
	require.Len(t, uaprsp2.Assignment.Spec.Assignments, 2)

	// verify upsert rejects scope change
	assignment04ScopeChange := apiutils.CloneProtoMsg(uaprsp2.Assignment)
	assignment04ScopeChange.Scope = "/foo"
	_, err = service.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: assignment04ScopeChange,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}
