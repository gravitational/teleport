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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	srpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedrole/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend/memory"
	sr "github.com/gravitational/teleport/lib/scopes/roles"
)

// TestScopedRoleBasicCRUD tests the basic CRUD operations of the ScopedRoleService, excluding the more non-trivial
// scenarios involving roles with active assignments, which are tested separately.
func TestScopedRoleBasicCRUD(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedRoleService(backend)

	basicRoles := []*srpb.ScopedRole{
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "basic-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "basic-02",
			},
			Scope: "/bar",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/bar/**"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "basic-03",
			},
			Scope: "/baz",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/baz/**"},
			},
			Version: types.V1,
		},
	}

	var revisions []string

	// verify the expected behavior of CreateScopedRole
	for _, role := range basicRoles {
		crsp, err := service.CreateScopedRole(ctx, &srpb.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
		require.NotEmpty(t, crsp.Role.Metadata.Revision)
		require.Empty(t, cmp.Diff(role, crsp.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerpb.Metadata{}, "revision")))

		// Check that the role can be retrieved.
		grsp, err := service.GetScopedRole(ctx, &srpb.GetScopedRoleRequest{
			Name: role.Metadata.Name,
		})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(crsp.Role, grsp.Role, protocmp.Transform() /* deliberately not ignoring revision */))

		revisions = append(revisions, grsp.Role.Metadata.Revision)
	}

	require.Len(t, revisions, len(basicRoles))

	// verify that create fails if the role already exists
	_, err = service.CreateScopedRole(ctx, &srpb.CreateScopedRoleRequest{
		Role: basicRoles[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify a basic allowable update
	basic01Mod := apiutils.CloneProtoMsg(basicRoles[0])
	basic01Mod.Spec.AssignableScopes = []string{"/foo", "/bar"}
	basic01Mod.Metadata.Revision = revisions[0]

	ursp, err := service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.NoError(t, err)
	require.NotEmpty(t, ursp.Role.Metadata.Revision)
	require.Empty(t, cmp.Diff(basic01Mod, ursp.Role, protocmp.Transform(), protocmp.IgnoreFields(&headerpb.Metadata{}, "revision")))

	// verify that update really happened
	grsp, err := service.GetScopedRole(ctx, &srpb.GetScopedRoleRequest{
		Name: basic01Mod.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ursp.Role, grsp.Role, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that update fails if the revision is wrong
	_, err = service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that update is rejected if the role's scope is changed
	basic01Mod = apiutils.CloneProtoMsg(ursp.Role)
	basic01Mod.Scope = "/foo"

	_, err = service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: basic01Mod,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that update fails if the role does not exist
	_, err = service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: &srpb.ScopedRole{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name:     "non-existent",
				Revision: revisions[0],
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete fails if the role does not exist
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name: "non-existent",
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete fails if the revision does not match
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name:     basicRoles[0].Metadata.Name,
		Revision: revisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify successful unconditional delete
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name: basicRoles[0].Metadata.Name,
	})
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, &srpb.GetScopedRoleRequest{
		Name: basicRoles[0].Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// verify successful conditional delete
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name:     basicRoles[1].Metadata.Name,
		Revision: revisions[1],
	})
	require.NoError(t, err)

	// verify that the role is gone
	_, err = service.GetScopedRole(ctx, &srpb.GetScopedRoleRequest{
		Name: basicRoles[1].Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

// TestScopedRoleAssignmentBasicCRD tests the basic CRD operations of the ScopedRoleAssignmentService, excluding the more non-trivial
// scenarios involving roles with active assignments, which are tested separately.
func TestScopedRoleAssignmentBasicCRD(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedRoleService(backend)

	roles := []*srpb.ScopedRole{
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-02",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-03",
			},
			Scope: "/foo",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
	}

	var roleRevisions []string

	// Create the roles.
	for _, role := range roles {
		rsp, err := service.CreateScopedRole(ctx, &srpb.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)

		roleRevisions = append(roleRevisions, rsp.Role.Metadata.Revision)
	}

	// basic root assignment to test standard CRD operations with (initially invalid,
	// will be modified later to be valid)
	assignment01 := &srpb.ScopedRoleAssignment{
		Kind: sr.KindScopedRoleAssignment,
		Metadata: &headerpb.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &srpb.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*srpb.Assignment{
				{
					Role:  "role-02", // not assignable to root
					Scope: "/",
				},
			},
		},
		Version: types.V1,
	}

	// check that assignment to root fails since the target role is only assignable to /foo
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
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
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that assignment of correct role still fails if revision is incorrect
	assignment01.Scope = "/" // fix scope to be valid for root assignment
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[1], // revision of role-02, not role-01
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// check that assignment of correct role with correct revision works
	crsp, err := service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerpb.Metadata{}, "revision")))

	// Check that the assignment can be retrieved.
	grsp, err := service.GetScopedRoleAssignment(ctx, &srpb.GetScopedRoleAssignmentRequest{
		Name: assignment01.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))

	// verify that create fails if the assignment already exists
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with incorrect revision fails
	_, err = service.DeleteScopedRoleAssignment(ctx, &srpb.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: roleRevisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that delete of assignment with correct revision works
	_, err = service.DeleteScopedRoleAssignment(ctx, &srpb.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
	})
	require.NoError(t, err)

	// verify that the assignment is gone
	_, err = service.GetScopedRoleAssignment(ctx, &srpb.GetScopedRoleAssignmentRequest{
		Name: assignment01.Metadata.Name,
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// set up a more non-trivial assignment with multiple sub-assignments
	assignment02 := &srpb.ScopedRoleAssignment{
		Kind: sr.KindScopedRoleAssignment,
		Metadata: &headerpb.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &srpb.ScopedRoleAssignmentSpec{
			User: "bob",
			Assignments: []*srpb.Assignment{
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
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
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
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[2], // revision of role-03, not role-02
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// verify that assignment with some but not all of the role revisions fails
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			// role-02 is missing
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// verify that assignment with all of the role revisions works
	crsp, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment02,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[1],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment02, protocmp.Transform(), protocmp.IgnoreFields(&headerpb.Metadata{}, "revision")))

	// Check that the assignment can be retrieved
	grsp, err = service.GetScopedRoleAssignment(ctx, &srpb.GetScopedRoleAssignmentRequest{
		Name: assignment02.Metadata.Name,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(crsp.Assignment, grsp.Assignment, protocmp.Transform() /* deliberately not ignoring revision */))
}

// TestScopedRoleAssignmentInteraction verifies the expected interaction rules between scoped roles and
// scoped role assignments.
func TestScopedRoleAssignmentInteraction(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	defer backend.Close()

	service := NewScopedRoleService(backend)

	roles := []*srpb.ScopedRole{
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-01",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-02",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-03",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/bar"},
			},
			Version: types.V1,
		},
		{
			Kind: sr.KindScopedRole,
			Metadata: &headerpb.Metadata{
				Name: "role-04",
			},
			Scope: "/",
			Spec: &srpb.ScopedRoleSpec{
				AssignableScopes: []string{"/bin"},
			},
			Version: types.V1,
		},
	}

	var roleRevisions []string

	// Create the roles.
	for _, role := range roles {
		rsp, err := service.CreateScopedRole(ctx, &srpb.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)

		roleRevisions = append(roleRevisions, rsp.Role.Metadata.Revision)
	}

	// set up a non-trivial assignment with multiple sub-assignments
	assignment01 := &srpb.ScopedRoleAssignment{
		Kind: sr.KindScopedRoleAssignment,
		Metadata: &headerpb.Metadata{
			Name: uuid.New().String(),
		},
		Scope: "/",
		Spec: &srpb.ScopedRoleAssignmentSpec{
			User: "alice",
			Assignments: []*srpb.Assignment{
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

	crsp, err := service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
		Assignment: assignment01,
		RoleRevisions: map[string]string{
			"role-01": roleRevisions[0],
			"role-02": roleRevisions[1],
			"role-03": roleRevisions[2],
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, crsp.Assignment.Metadata.Revision)
	require.Empty(t, cmp.Diff(crsp.Assignment, assignment01, protocmp.Transform(), protocmp.IgnoreFields(&headerpb.Metadata{}, "revision")))

	// check that unrelated role can be deleted
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name:     "role-04",
		Revision: roleRevisions[3],
	})
	require.NoError(t, err)

	// check that deleting a role referenced by an assignment fails
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name:     "role-01",
		Revision: roleRevisions[0],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// check that updated a role s.t. it would invalidate an assignment fails
	updatedRole := apiutils.CloneProtoMsg(roles[1])
	updatedRole.Spec.AssignableScopes = []string{"/bin"} // role-02 is now assignable to /bin, not /foo
	updatedRole.Metadata.Revision = roleRevisions[1]
	_, err = service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: updatedRole,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)

	// check that deletion of a role s.t. it would invalidate an assignment fails
	_, err = service.DeleteScopedRole(ctx, &srpb.DeleteScopedRoleRequest{
		Name:     "role-02",
		Revision: roleRevisions[1],
	})
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got %v", err)

	// delete the assignment
	_, err = service.DeleteScopedRoleAssignment(ctx, &srpb.DeleteScopedRoleAssignmentRequest{
		Name:     assignment01.Metadata.Name,
		Revision: crsp.Assignment.Metadata.Revision,
	})
	require.NoError(t, err)

	// check that update of role now succeeds
	urrsp, err := service.UpdateScopedRole(ctx, &srpb.UpdateScopedRoleRequest{
		Role: updatedRole,
	})
	require.NoError(t, err)

	// check that recreate of assignment would now fail due to conflicting role
	_, err = service.CreateScopedRoleAssignment(ctx, &srpb.CreateScopedRoleAssignmentRequest{
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
