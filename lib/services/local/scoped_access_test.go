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
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestScopedRoleEvents verifies the expected behavior of backend events for the ScopedRole family of types.
func TestScopedRoleEvents(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testScopedRoleEvents)
}

func testScopedRoleEvents(t *testing.T) {
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
		t.Helper()
		synctest.Wait()
		select {
		case event := <-watcher.Events():
			return event
		case <-watcher.Done():
			require.FailNow(t, "Watcher exited with error", watcher.Error())
		default:
			require.FailNow(t, "No event ready, synctest bubble is durably blocked")
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
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
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
		RoleRevisions: map[string]string{
			role.Metadata.Name: crsp.Role.Metadata.Revision,
		},
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)
	assignmentResource := (event.Resource).(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]).UnwrapT()
	require.Empty(t, cmp.Diff(acrsp.Assignment, assignmentResource, protocmp.Transform() /* deliberately not ignoring revision */))

	// delete the assignment and verify delete event is well-formed.
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:    assignment.Metadata.Name,
		SubKind: assignment.SubKind,
	})
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)

	require.Empty(t, cmp.Diff(&types.ResourceHeader{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: types.Metadata{
			Name: assignment.Metadata.Name,
		},
	}, event.Resource.(*types.ResourceHeader), protocmp.Transform()))

	// Assert that any materialized assignments put into the backend (possibly
	// by an auth service on a later version) don't make it into the event
	// stream. Use the backend directly to skip subkind validation.
	assignment.SubKind = scopedaccess.SubKindMaterialized
	item, err := scopedRoleAssignmentToItem(assignment)
	require.NoError(t, err)
	_, err = service.bk.Put(ctx, item)
	require.NoError(t, err)
	synctest.Wait()
	select {
	case evt := <-watcher.Events():
		t.Fatalf("expected no event, got %v", evt)
	default:
	}
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
				AssignableScopes: []string{"/"},
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

	// basic root assignment to test standard CRD operations with (initially invalid,
	// will be modified later to be valid)
	assignment01 := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  "role-02", // not assignable to root
					Scope: "/",
				},
			},
		},
		Version: types.V1,
	}

	// check that assignment to root fails since the target role is only assignable to /foo
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-02": roleRevisions[1],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that assignment with an invalid resource scope fails
	assignment01.Spec.Assignments[0].Role = "role-01" // fix role to be assignable to root
	assignment01.Scope = "/foo"                       // invalid scope for root assignment
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that assignment of correct role still fails if revision is incorrect
	assignment01.Scope = "/" // fix scope to be valid for root assignment
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[1], // revision of role-02, not role-01
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// check that otherwise valid assignment fails if subkind is unset.
	assignment01.SubKind = ""
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that otherwise valid assignment fails if subkind is materialized.
	assignment01.SubKind = scopedaccess.SubKindMaterialized
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that assignment of correct role with correct revision works
	assignment01.SubKind = scopedaccess.SubKindDynamic
	crsp, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// Check that the assignment can be retrieved.
	grsp, err := service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    assignment01.Metadata.Name,
		SubKind: scopedaccess.SubKindDynamic,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that create fails if the assignment already exists
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with incorrect revision fails
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: roleRevisions[0],
		SubKind:  crsp.Assignment.SubKind,
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with empty subkind fails
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
		SubKind:  "",
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that delete of assignment with unknown subkind fails
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
		SubKind:  "unknown",
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that delete of assignment with correct revision works
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
		SubKind:  crsp.Assignment.SubKind,
	})
	require.NoError(t, err)

	// verify that the assignment is gone
	_, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    assignment01.Metadata.Name,
		SubKind: assignment01.SubKind,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// set up a more non-trivial assignment with multiple sub-assignments
	assignment02 := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
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
					Role:  "role-03", // role-03 cannot by assigned to by an assignment in the root resource scope
					Scope: "/foo",
				},
			},
		},
		Version: types.V1,
	}

	// verify that assignment with a mix of conflicting and correct resource scopes fails
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[1],
			"role-03": roleRevisions[2],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that a mix of valid and invalid role revisions fails
	assignment02.Spec.Assignments = assignment02.Spec.Assignments[:2] // remove role-03 assignment
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[2], // revision of role-03, not role-02
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that assignment with some but not all of the role revisions fails
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			// role-02 is missing
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that assignment with all of the role revisions works
	crsp, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[1],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment02, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// Check that the assignment can be retrieved
	grsp, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    assignment02.Metadata.Name,
		SubKind: assignment02.SubKind,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// create an assignment that assigns the same role at multiple separate scopes (covers a specific
	// bug where original impl would construct invalid conditional actions when multiple sub-assignments
	// are made for the same role).
	assignment03 := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
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
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.NoError(t, err)

	// verify that deletion of assignment works
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:    assignment03.Metadata.Name,
		SubKind: assignment03.SubKind,
	})
	require.NoError(t, err)
}

// TestScopedRoleAssignmentInteraction verifies the expected interaction rules between scoped roles and
// scoped role assignments.
func TestScopedRoleAssignmentInteraction(t *testing.T) {
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
				AssignableScopes: []string{"/"},
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
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/bar"},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "role-04",
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/bin"},
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

	// set up a non-trivial assignment with multiple sub-assignments
	assignment01 := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "alice",
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
					Role:  "role-03",
					Scope: "/bar",
				},
			},
		},
		Version: types.V1,
	}

	crsp, err := service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[1],
			"role-03": roleRevisions[2],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// check that unrelated role can be deleted
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     "role-04",
		Revision: roleRevisions[3],
	})
	require.NoError(t, err)

	// check that deleting a role referenced by an assignment fails
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     "role-01",
		Revision: roleRevisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// check that updated a role s.t. it would invalidate an assignment fails
	updatedRole := apiutils.CloneProtoMsg(roles[1])
	updatedRole.Spec.AssignableScopes = []string{"/bin"} // role-02 is now assignable to /bin, not /foo
	updatedRole.Metadata.Revision = roleRevisions[1]
	_, err = service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: updatedRole,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that deletion of a role s.t. it would invalidate an assignment fails
	_, err = service.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     "role-02",
		Revision: roleRevisions[1],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// delete the assignment
	_, err = service.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
		SubKind:  assignment01.SubKind,
	})
	require.NoError(t, err)

	// check that update of role now succeeds
	urrsp, err := service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: updatedRole,
	})
	require.NoError(t, err)

	// check that recreate of assignment would now fail due to conflicting role
	_, err = service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": urrsp.Role.Metadata.Revision, // revision of updated role-02
			"role-03": roleRevisions[2],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

func TestGetScopedRoleAssignmentSubkinds(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	backend, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	defer backend.Close()

	service := NewScopedAccessService(backend)

	// SubKind is required for GetScopedRoleAssignment.
	_, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: "assignment-01",
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// Getting a materialized assignment from the backend is an error.
	_, err = service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name:    "assignment-01",
		SubKind: scopedaccess.SubKindMaterialized,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
}

// TestScopedRoleInteractionWithAccessListGrants verifies the expected
// interaction between access list grants and scoped role writes. Namely:
//   - scoped roles cannot update their assignable scopes if that would
//     invalidate an assignment from an access list
//   - scoped roles cannot be deleted if they are assigned from an access list
func TestScopedRoleInteractionWithAccessListGrants(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{Context: ctx, Clock: clock})
	require.NoError(t, err)
	defer bk.Close()

	accessListService := newAccessListService(t, bk, modulestest.EnterpriseModules())
	scopedAccessService := NewScopedAccessService(bk)

	// Create a base scoped role to update.
	role := &scopedaccessv1.ScopedRole{
		Kind: scopedaccess.KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: "testrole",
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{
				"/test/member",
				"/test/owner",
			},
		},
		Version: types.V1,
	}
	createRoleResp, err := scopedAccessService.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{Role: role})
	require.NoError(t, err)

	// Create an access list that grants the scoped role.
	al := newAccessList(t, "testlist", clock, withOwnerRequires(accesslist.Requires{}), withMemberRequires(accesslist.Requires{}))
	al.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{
			Role:  "testrole",
			Scope: "/test/member",
		},
	}
	al.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{
		{
			Role:  "testrole",
			Scope: "/test/owner",
		},
	}
	_, err = accessListService.UpsertAccessList(ctx, al)
	require.NoError(t, err)

	alm := newAccessListMember(t, "testlist", "alice")
	_, err = accessListService.UpsertAccessListMember(ctx, alm)
	require.NoError(t, err)

	// Cannot update the scoped role if it would invalidate the existing member grant.
	updatedRole := apiutils.CloneProtoMsg(createRoleResp.GetRole())
	updatedRole.Spec.AssignableScopes = []string{"/test/owner"}
	_, err = scopedAccessService.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{Role: updatedRole})
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	require.ErrorContains(t, err, `would invalidate access list "testlist" spec.grants`)

	// Cannot update the scoped role if it would invalidate the existing owner grant.
	updatedRole = apiutils.CloneProtoMsg(createRoleResp.GetRole())
	updatedRole.Spec.AssignableScopes = []string{"/test/member"}
	_, err = scopedAccessService.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{Role: updatedRole})
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	require.ErrorContains(t, err, `would invalidate access list "testlist" spec.owner_grants`)

	// Cannot delete a scoped role granted by an access list.
	_, err = scopedAccessService.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     "testrole",
		Revision: createRoleResp.GetRole().GetMetadata().GetRevision(),
	})
	require.Error(t, err)
	require.ErrorAs(t, err, new(*trace.CompareFailedError))
	require.ErrorContains(t, err, `while access list "testlist"`)

	// After deleting the access list, the scoped role can be updated or deleted.
	err = accessListService.DeleteAccessList(ctx, al.GetName())
	require.NoError(t, err)
	updateRoleResp, err := scopedAccessService.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{Role: updatedRole})
	require.NoError(t, err)
	_, err = scopedAccessService.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name:     "testrole",
		Revision: updateRoleResp.GetRole().GetMetadata().GetRevision(),
	})
	require.NoError(t, err)
}
