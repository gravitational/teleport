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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
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
	role := scopedaccessv1.ScopedRole_builder{
		Kind: scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{
			Name: "test-role",
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{"/foo", "/bar"},
		}.Build(),
		Version: types.V1,
	}.Build()

	crsp, err := service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: role,
	}.Build())
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)

	resource := (event.Resource).(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT()
	require.Empty(t, cmp.Diff(crsp.GetRole(), resource, protocmp.Transform() /* deliberately not ignoring revision */))

	// delete the role and verify delete event is well-formed.
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:  role.GetMetadata().GetName(),
		Scope: "/",
	}.Build())
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)

	deletedRole := event.Resource.(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT()
	require.Empty(t, cmp.Diff(scopedaccessv1.ScopedRole_builder{
		Kind: scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{
			Name: role.GetMetadata().GetName(),
		}.Build(),
		Scope: "/",
	}.Build(), deletedRole, protocmp.Transform()))

	// recreate scoped role so that we can use it for testing assignment events
	_, err = service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: role,
	}.Build())
	require.NoError(t, err)

	_ = getNextEvent() // drain the role create event

	assignment := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  scopes.QualifiedName{Scope: "/", Name: role.GetMetadata().GetName()}.String(),
					Scope: "/foo",
				}.Build(),
			},
		}.Build(),
		Version: types.V1,
	}.Build()

	acrsp, err := service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment,
	}.Build())
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)
	assignmentResource := (event.Resource).(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]).UnwrapT()
	require.Empty(t, cmp.Diff(acrsp.GetAssignment(), assignmentResource, protocmp.Transform() /* deliberately not ignoring revision */))

	// delete the assignment and verify delete event is well-formed.
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:    assignment.GetMetadata().GetName(),
		SubKind: assignment.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.NoError(t, err)

	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)

	deletedAssignment := event.Resource.(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]).UnwrapT()
	require.Empty(t, cmp.Diff(scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: assignment.GetMetadata().GetName(),
		}.Build(),
		Scope: "/",
	}.Build(), deletedAssignment, protocmp.Transform()))

	// Assert that any materialized assignments put into the backend (possibly
	// by an auth service on a later version) don't make it into the event
	// stream. Use the backend directly to skip subkind validation.
	assignment.SetSubKind(scopedaccess.SubKindMaterialized)
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

// TestScopedRoleEventsScopeFilter verifies that the local backend event watcher honors a watch kind's
// scope filter (equivalently to the fanout), applying it to both put and delete events.
func TestScopedRoleEventsScopeFilter(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testScopedRoleEventsScopeFilter)
}

func testScopedRoleEventsScopeFilter(t *testing.T) {
	ctx := t.Context()

	backend, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	defer backend.Close()

	service := NewScopedAccessService(backend)
	events := NewEventsService(backend)

	// watch scoped roles at exactly "/foo".
	watcher, err := events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind:        scopedaccess.KindScopedRole,
			ScopeFilter: types.ScopeFilterFromProto(scopesv1.Filter_builder{Scope: "/foo", Mode: scopesv1.Mode_MODE_EXACT}.Build()),
		}},
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

	expectNoEvent := func() {
		t.Helper()
		synctest.Wait()
		select {
		case event := <-watcher.Events():
			require.FailNow(t, "expected no event", "got %v of kind %q at scope %q", event.Type, event.Resource.GetKind(), event.Resource.GetSubKind())
		default:
		}
	}

	createRole := func(name, scope string) {
		t.Helper()
		_, err := service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:     scopedaccess.KindScopedRole,
				Metadata: headerv1.Metadata_builder{Name: name}.Build(),
				Scope:    scope,
				Spec:     scopedaccessv1.ScopedRoleSpec_builder{AssignableScopes: []string{scope}}.Build(),
				Version:  types.V1,
			}.Build(),
		}.Build())
		require.NoError(t, err)
	}

	deleteRole := func(name, scope string) {
		t.Helper()
		_, err := service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
			Name:  name,
			Scope: scope,
		}.Build())
		require.NoError(t, err)
	}

	event := getNextEvent()
	require.Equal(t, types.OpInit, event.Type)

	// a role at "/foo" matches the filter: its put event is delivered.
	createRole("foo-role", "/foo")
	event = getNextEvent()
	require.Equal(t, types.OpPut, event.Type)
	require.Equal(t, "/foo", event.Resource.(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT().GetScope())

	// a role at "/bar" (orthogonal) does not match: no event.
	createRole("bar-role", "/bar")
	expectNoEvent()

	// a role at "/foo/sub" (descendant) does not match an EXACT filter: no event.
	createRole("sub-role", "/foo/sub")
	expectNoEvent()

	// the delete of the "/foo" role matches the filter (scoped deletes carry scope): delete is delivered.
	deleteRole("foo-role", "/foo")
	event = getNextEvent()
	require.Equal(t, types.OpDelete, event.Type)
	require.Equal(t, "/foo", event.Resource.(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT().GetScope())

	// the delete of the "/bar" role does not match: no event.
	deleteRole("bar-role", "/bar")
	expectNoEvent()
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
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "basic-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/foo"},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "basic-02",
			}.Build(),
			Scope: "/bar",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/bar/**"},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "basic-03",
			}.Build(),
			Scope: "/baz",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/baz/**"},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	var revisions []string

	// verify the expected behavior of CreateScopedRole
	for _, role := range basicRoles {
		crsp, err := service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: role,
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, crsp.GetRole().GetMetadata().GetRevision())
		require.Empty(t, cmp.Diff(role, crsp.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

		// Check that the role can be retrieved.
		grsp, err := service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
			Name:  role.GetMetadata().GetName(),
			Scope: role.GetScope(),
		}.Build())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(crsp.GetRole(), grsp.GetRole(), protocmp.Transform() /* deliberately not ignoring revision */))

		revisions = append(revisions, grsp.GetRole().GetMetadata().GetRevision())
	}

	require.Len(t, revisions, len(basicRoles))

	// verify that create fails if the role already exists
	_, err = service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: basicRoles[0],
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify a basic allowable update
	basic01Mod := apiutils.CloneProtoMsg(basicRoles[0])
	basic01Mod.GetSpec().SetAssignableScopes([]string{"/foo", "/bar"})
	basic01Mod.GetMetadata().SetRevision(revisions[0])

	ursp, err := service.UpdateScopedRole(ctx, scopedaccessv1.UpdateScopedRoleRequest_builder{
		Role: basic01Mod,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, ursp.GetRole().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(basic01Mod, ursp.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify that update really happened
	grsp, err := service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  basic01Mod.GetMetadata().GetName(),
		Scope: "/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ursp.GetRole(), grsp.GetRole(), protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that update fails if the revision is wrong
	_, err = service.UpdateScopedRole(ctx, scopedaccessv1.UpdateScopedRoleRequest_builder{
		Role: basic01Mod,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that update is rejected if the role's scope is changed
	basic01Mod = apiutils.CloneProtoMsg(ursp.GetRole())
	basic01Mod.SetScope("/foo")

	_, err = service.UpdateScopedRole(ctx, scopedaccessv1.UpdateScopedRoleRequest_builder{
		Role: basic01Mod,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that update fails if the role does not exist
	_, err = service.UpdateScopedRole(ctx, scopedaccessv1.UpdateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name:     "non-existent",
				Revision: revisions[0],
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/foo"},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete fails if the role does not exist
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:  "non-existent",
		Scope: "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify that delete fails if the revision does not match
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:     basicRoles[0].GetMetadata().GetName(),
		Scope:    basicRoles[0].GetScope(),
		Revision: revisions[0],
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify successful unconditional delete
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:  basicRoles[0].GetMetadata().GetName(),
		Scope: basicRoles[0].GetScope(),
	}.Build())
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  basicRoles[0].GetMetadata().GetName(),
		Scope: basicRoles[0].GetScope(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify successful conditional delete
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:     basicRoles[1].GetMetadata().GetName(),
		Scope:    basicRoles[1].GetScope(),
		Revision: revisions[1],
	}.Build())
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  basicRoles[1].GetMetadata().GetName(),
		Scope: basicRoles[1].GetScope(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify upsert creates when role does not exist
	basic04 := scopedaccessv1.ScopedRole_builder{
		Kind: scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{
			Name: "basic-04",
		}.Build(),
		Scope: "/qux",
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{"/qux"},
		}.Build(),
		Version: types.V1,
	}.Build()
	uprsp, err := service.UpsertScopedRole(ctx, scopedaccessv1.UpsertScopedRoleRequest_builder{
		Role: basic04,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, uprsp.GetRole().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(basic04, uprsp.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify upsert updates when role already exists (including with a stale/wrong revision)
	basic04Mod := apiutils.CloneProtoMsg(uprsp.GetRole())
	basic04Mod.GetSpec().SetAssignableScopes([]string{"/qux", "/qux/sub"})
	basic04Mod.GetMetadata().SetRevision(revisions[2]) // deliberately stale revision

	uprsp2, err := service.UpsertScopedRole(ctx, scopedaccessv1.UpsertScopedRoleRequest_builder{
		Role: basic04Mod,
	}.Build())
	require.NoError(t, err, "upsert should succeed despite stale revision")
	require.Empty(t, cmp.Diff(basic04Mod, uprsp2.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// namespaced by scope, so upserting the same name at a different scope creates a distinct role
	// rather than "moving" the existing one.
	basic04OtherScope := apiutils.CloneProtoMsg(uprsp2.GetRole())
	basic04OtherScope.SetScope("/other")
	basic04OtherScope.GetSpec().SetAssignableScopes([]string{"/other"})
	basic04OtherScope.GetMetadata().SetRevision("")
	upOther, err := service.UpsertScopedRole(ctx, scopedaccessv1.UpsertScopedRoleRequest_builder{
		Role: basic04OtherScope,
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(basic04OtherScope, upOther.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// the original role at /qux is untouched.
	origGet, err := service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  basic04.GetMetadata().GetName(),
		Scope: "/qux",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(basic04Mod, origGet.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// and the distinct role at /other is independently retrievable.
	otherGet, err := service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  basic04.GetMetadata().GetName(),
		Scope: "/other",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(basic04OtherScope, otherGet.GetRole(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))
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
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "role-01",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/foo", "/bar"},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "role-02",
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/foo"},
			}.Build(),
			Version: types.V1,
		}.Build(),
		scopedaccessv1.ScopedRole_builder{
			Kind: scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{
				Name: "role-03",
			}.Build(),
			Scope: "/foo",
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{"/foo"},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}

	var roleRevisions []string

	// Create the roles.
	for _, role := range roles {
		rsp, err := service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: role,
		}.Build())
		require.NoError(t, err)

		roleRevisions = append(roleRevisions, rsp.GetRole().GetMetadata().GetRevision())
	}

	// basic root assignment to test standard CRD operations with
	assignment01 := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "alice",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "/::role-02",
					Scope: "/", // root scope of effect is not permitted
				}.Build(),
			},
		}.Build(),
		Status: scopedaccessv1.ScopedRoleAssignmentStatus_builder{
			Origin: scopedaccessv1.ScopedRoleAssignmentStatus_Origin_builder{
				CreatorKind: scopedaccess.CreatorKindAccessList,
				CreatorName: "test-list",
			}.Build(),
		}.Build(),
		Version: types.V1,
	}.Build()

	// check that root scope of effect is rejected
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that a sub-assignment scope outside the assignment's resource scope is rejected
	assignment01.GetSpec().GetAssignments()[0].SetScope("/bar") // non-root, but outside resource scope /foo
	assignment01.SetScope("/foo")
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that otherwise valid assignment fails if subkind is unset.
	assignment01.SetScope("/") // fix resource scope
	assignment01.SetSubKind("")
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that otherwise valid assignment fails if subkind is materialized.
	assignment01.SetSubKind(scopedaccess.SubKindMaterialized)
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that otherwise valid assignment fails if subkind is unknown.
	assignment01.SetSubKind("unknown")
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that a valid assignment succeeds
	assignment01.SetSubKind(scopedaccess.SubKindDynamic)
	crsp, err := service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, crsp.GetAssignment().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(crsp.GetAssignment(), assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// check that the assignment can be retrieved.
	grsp, err := service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment01.GetMetadata().GetName(),
		SubKind: scopedaccess.SubKindDynamic,
		Scope:   "/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.GetAssignment(), grsp.GetAssignment(), protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that getting a materialized assignment from the backend is an error.
	_, err = service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment01.GetMetadata().GetName(),
		SubKind: scopedaccess.SubKindMaterialized,
		Scope:   "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that create fails if the assignment already exists
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify a basic allowable update
	assignment01Mod := apiutils.CloneProtoMsg(crsp.GetAssignment())
	assignment01Mod.GetSpec().GetAssignments()[0].SetScope("/foo")
	assignment01Mod.GetMetadata().SetRevision(crsp.GetAssignment().GetMetadata().GetRevision())

	ursp, err := service.UpdateScopedRoleAssignment(ctx, scopedaccessv1.UpdateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01Mod,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, ursp.GetAssignment().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(assignment01Mod, ursp.GetAssignment(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify that update really happened
	grsp, err = service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment01Mod.GetMetadata().GetName(),
		SubKind: assignment01Mod.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ursp.GetAssignment(), grsp.GetAssignment(), protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that update fails if the revision is wrong (stale revision from before the update)
	_, err = service.UpdateScopedRoleAssignment(ctx, scopedaccessv1.UpdateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01Mod,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// namespaced by scope, so changing the scope targets a different (nonexistent) resource
	// and fails rather than mutating the original.
	assignment01OtherScope := apiutils.CloneProtoMsg(ursp.GetAssignment())
	assignment01OtherScope.SetScope("/foo")
	_, err = service.UpdateScopedRoleAssignment(ctx, scopedaccessv1.UpdateScopedRoleAssignmentRequest_builder{
		Assignment: assignment01OtherScope,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that update fails if the assignment does not exist
	_, err = service.UpdateScopedRoleAssignment(ctx, scopedaccessv1.UpdateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Metadata: headerv1.Metadata_builder{
				Name:     "00000000-0000-0000-0000-000000000000",
				Revision: crsp.GetAssignment().GetMetadata().GetRevision(),
			}.Build(),
			Scope: "/",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: "/::role-02", Scope: "/foo"}.Build(),
				},
			}.Build(),
			Version: types.V1,
		}.Build(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with incorrect revision fails
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:     assignment01.GetMetadata().GetName(),
		Revision: roleRevisions[0],
		SubKind:  crsp.GetAssignment().GetSubKind(),
		Scope:    "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with materialized subkind fails
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:     assignment01.GetMetadata().GetName(),
		Revision: crsp.GetAssignment().GetMetadata().GetRevision(),
		SubKind:  scopedaccess.SubKindMaterialized,
		Scope:    "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that delete of assignment with unknown subkind fails
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:     assignment01.GetMetadata().GetName(),
		Revision: crsp.GetAssignment().GetMetadata().GetRevision(),
		SubKind:  "unknown",
		Scope:    "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that delete of assignment with correct revision works
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:     assignment01.GetMetadata().GetName(),
		Revision: ursp.GetAssignment().GetMetadata().GetRevision(),
		SubKind:  ursp.GetAssignment().GetSubKind(),
		Scope:    "/",
	}.Build())
	require.NoError(t, err)

	// verify that the assignment is gone
	_, err = service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment01.GetMetadata().GetName(),
		SubKind: assignment01.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// set up a more non-trivial assignment with multiple sub-assignments
	// assignment02 mixes roles from different resource scopes. cross-resource consistency (e.g. whether
	// a role at a given resource scope is accessible from the assignment's resource scope) is not enforced
	// at write time; it is enforced exclusively at the policy decision point.
	assignment02 := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "bob",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "/::role-01",
					Scope: "/foo",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "/::role-02",
					Scope: "/foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "/foo::role-03", // resource scope /foo, different from assignment resource scope /
					Scope: "/foo",
				}.Build(),
			},
		}.Build(),
		Version: types.V1,
	}.Build()

	crsp, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment02,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, crsp.GetAssignment().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(crsp.GetAssignment(), assignment02, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// Check that the assignment can be retrieved
	grsp, err = service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment02.GetMetadata().GetName(),
		SubKind: assignment02.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.GetAssignment(), grsp.GetAssignment(), protocmp.Transform() /* deliberately not ignoring revision */))

	// create an assignment that assigns the same role at multiple separate scopes (covers a specific
	// bug where original impl would construct invalid conditional actions when multiple sub-assignments
	// are made for the same role).
	assignment03 := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "carol",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "/::role-01",
					Scope: "/foo",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "/::role-01",
					Scope: "/bar",
				}.Build(),
			},
		}.Build(),
		Version: types.V1,
	}.Build()

	// check that creation of assignment works
	_, err = service.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: assignment03,
	}.Build())
	require.NoError(t, err)

	// verify that deletion of assignment works
	_, err = service.DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:    assignment03.GetMetadata().GetName(),
		SubKind: assignment03.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.NoError(t, err)

	// verify upsert creates when assignment does not exist
	assignment04 := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.New().String(),
		}.Build(),
		Scope: "/",
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			User: "dave",
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{Role: "/::role-01", Scope: "/foo"}.Build(),
			},
		}.Build(),
		Version: types.V1,
	}.Build()
	uaprsp, err := service.UpsertScopedRoleAssignment(ctx, scopedaccessv1.UpsertScopedRoleAssignmentRequest_builder{
		Assignment: assignment04,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, uaprsp.GetAssignment().GetMetadata().GetRevision())
	require.Empty(t, cmp.Diff(assignment04, uaprsp.GetAssignment(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// verify upsert updates when assignment already exists (including with a stale/wrong revision)
	assignment04Mod := apiutils.CloneProtoMsg(uaprsp.GetAssignment())
	assignment04Mod.GetSpec().SetAssignments(append(assignment04Mod.GetSpec().GetAssignments(), scopedaccessv1.Assignment_builder{
		Role: "/::role-02", Scope: "/foo",
	}.Build()))
	assignment04Mod.GetMetadata().SetRevision(roleRevisions[0]) // deliberately stale revision

	uaprsp2, err := service.UpsertScopedRoleAssignment(ctx, scopedaccessv1.UpsertScopedRoleAssignmentRequest_builder{
		Assignment: assignment04Mod,
	}.Build())
	require.NoError(t, err, "upsert should succeed despite stale revision")
	require.Len(t, uaprsp2.GetAssignment().GetSpec().GetAssignments(), 2)

	// under namespacing an assignment's (scope, name) is its identity, so upserting the same name at a
	// different scope creates a distinct assignment rather than moving the existing one; both coexist.
	assignment04OtherScope := apiutils.CloneProtoMsg(uaprsp2.GetAssignment())
	assignment04OtherScope.SetScope("/foo")
	assignment04OtherScope.GetMetadata().SetRevision("") // distinct resource, no prior revision
	upOther, err := service.UpsertScopedRoleAssignment(ctx, scopedaccessv1.UpsertScopedRoleAssignmentRequest_builder{
		Assignment: assignment04OtherScope,
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment04OtherScope, upOther.GetAssignment(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// the original assignment at / is untouched.
	origGet, err := service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment04.GetMetadata().GetName(),
		SubKind: assignment04.GetSubKind(),
		Scope:   "/",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment04Mod, origGet.GetAssignment(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	// and the distinct assignment at /foo is independently retrievable.
	otherGet, err := service.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
		Name:    assignment04.GetMetadata().GetName(),
		SubKind: assignment04.GetSubKind(),
		Scope:   "/foo",
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment04OtherScope, otherGet.GetAssignment(), protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))
}

// TestScopedAccessScopeConflict verifies that when a scoped resource's scope field disagrees with the
// scope encoded in its backend key, the resource is rejected appropriately. Get errors, List skips,
// and the event parser errors (closing the watcher). Delete succeeds when targeting the.
func TestScopedAccessScopeConflict(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	bk, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	defer bk.Close()

	service := NewScopedAccessService(bk)

	// role definition specifies scope `/foo`
	role := scopedaccessv1.ScopedRole_builder{
		Kind:     scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{Name: "conflicted"}.Build(),
		Scope:    "/foo",
		Spec:     scopedaccessv1.ScopedRoleSpec_builder{AssignableScopes: []string{"/foo"}}.Build(),
		Version:  types.V1,
	}.Build()

	validItem, err := scopedRoleToItem(role)
	require.NoError(t, err)

	// role key specified scope `/bar`
	conflictKey, err := scopedRoleKey{scope: "/bar", name: role.GetMetadata().GetName()}.Key()
	require.NoError(t, err)
	conflictItem := backend.Item{Key: conflictKey, Value: validItem.Value}
	_, err = service.bk.Put(ctx, conflictItem)
	require.NoError(t, err)

	//  write a well-formed role so we can confirm List skips only the conflicting one.
	validRole := scopedaccessv1.ScopedRole_builder{
		Kind:     scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{Name: "valid"}.Build(),
		Scope:    "/baz",
		Spec:     scopedaccessv1.ScopedRoleSpec_builder{AssignableScopes: []string{"/baz"}}.Build(),
		Version:  types.V1,
	}.Build()
	_, err = service.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{Role: validRole}.Build())
	require.NoError(t, err)

	// Get at the key-encoded scope finds the item but rejects it as a scope conflict.
	_, err = service.GetScopedRole(ctx, scopedaccessv1.GetScopedRoleRequest_builder{
		Name:  role.GetMetadata().GetName(),
		Scope: "/bar",
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// List skips the conflicting role but still returns the valid one.
	lrsp, err := service.ListScopedRoles(ctx, &scopedaccessv1.ListScopedRolesRequest{})
	require.NoError(t, err)
	var listedNames []string
	for _, r := range lrsp.GetRoles() {
		listedNames = append(listedNames, r.GetMetadata().GetName())
	}
	require.Equal(t, []string{"valid"}, listedNames)

	parser := newScopedRoleParser()

	// the Put event parser errors on the conflict.
	_, err = parser.parse(backend.Event{Type: types.OpPut, Item: conflictItem})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// the Delete event parser is unaffecte.
	deleted, err := parser.parse(backend.Event{Type: types.OpDelete, Item: conflictItem})
	require.NoError(t, err)
	deletedRole := deleted.(types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]).UnwrapT()
	require.Equal(t, "/bar", deletedRole.GetScope())
	require.Equal(t, role.GetMetadata().GetName(), deletedRole.GetMetadata().GetName())

	// Delete by the key-encoded scope succeeds
	_, err = service.DeleteScopedRole(ctx, scopedaccessv1.DeleteScopedRoleRequest_builder{
		Name:  role.GetMetadata().GetName(),
		Scope: "/bar",
	}.Build())
	require.NoError(t, err)
}
