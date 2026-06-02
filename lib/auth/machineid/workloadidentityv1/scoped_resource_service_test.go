// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package workloadidentityv1_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// newScopedWorkloadIdentityUser creates a scoped user assigned a scoped role
// granting full WorkloadIdentity CRUD within the given scope, and returns the
// username. The returned user can be used with authtest.TestScopedUser to mint
// a scoped client pinned to the scope.
func newScopedWorkloadIdentityUser(
	t *testing.T,
	srv *authtest.TLSServer,
	adminClient *authclient.Client,
	username string,
	scope string,
) string {
	t.Helper()
	ctx := t.Context()

	scopedSvc := adminClient.ScopedAccessServiceClient()
	role, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: username + "-role",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{scope},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{types.KindWorkloadIdentity},
						Verbs: []string{
							types.VerbCreate,
							types.VerbReadNoSecrets,
							types.VerbUpdate,
							types.VerbDelete,
							types.VerbList,
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(ctx, srv.Auth(), username)
	require.NoError(t, err)

	resp, err := scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: user.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: role.Role.Metadata.Name, Scope: scope},
				},
			},
		},
	})
	require.NoError(t, err)

	// Wait for the assignment to propagate to the cache used by the authorizer.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := srv.Auth().ScopedAccessCache.GetScopedRoleAssignment(
			ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
				Name:    resp.GetAssignment().GetMetadata().GetName(),
				SubKind: resp.GetAssignment().GetSubKind(),
			})
		require.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	return user.GetName()
}

func newScopedWorkloadIdentity(name, scope, spiffeID string) *workloadidentityv1pb.WorkloadIdentity {
	return &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Scope: scope,
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: spiffeID,
			},
		},
	}
}

func TestResourceService_ScopedWorkloadIdentity(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := t.Context()

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	const grantedScope = "/scopes/granted"
	const otherScope = "/scopes/other"

	grantedUser := newScopedWorkloadIdentityUser(t, srv, adminClient, "scoped-granted", grantedScope)
	otherUser := newScopedWorkloadIdentityUser(t, srv, adminClient, "scoped-other", otherScope)

	grantedClient, err := srv.NewClient(authtest.TestScopedUser(grantedUser, grantedScope))
	require.NoError(t, err)
	t.Cleanup(func() { _ = grantedClient.Close() })
	grantedSvc := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(grantedClient.GetConnection())

	otherClient, err := srv.NewClient(authtest.TestScopedUser(otherUser, otherScope))
	require.NoError(t, err)
	t.Cleanup(func() { _ = otherClient.Close() })
	otherSvc := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(otherClient.GetConnection())

	t.Run("create success", func(t *testing.T) {
		wi := newScopedWorkloadIdentity("create-ok", grantedScope, grantedScope+"/_/foo-svc")
		created, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: wi,
		})
		require.NoError(t, err)
		require.Equal(t, grantedScope, created.GetScope())

		// Confirm it persisted with the scope.
		fetched, err := srv.Auth().GetWorkloadIdentity(ctx, "create-ok")
		require.NoError(t, err)
		require.Equal(t, grantedScope, fetched.GetScope())
	})

	t.Run("create rejects SPIFFE ID outside scope", func(t *testing.T) {
		wi := newScopedWorkloadIdentity("create-bad-id", grantedScope, otherScope+"/_/foo-svc")
		_, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: wi,
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
		require.ErrorContains(t, err, "must be prefixed with the scope")
	})

	t.Run("create rejects SPIFFE ID missing separator", func(t *testing.T) {
		wi := newScopedWorkloadIdentity("create-no-sep", grantedScope, grantedScope+"/foo-svc")
		_, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: wi,
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	})

	t.Run("create denied creating in another scope", func(t *testing.T) {
		wi := newScopedWorkloadIdentity("create-wrong-scope", otherScope, otherScope+"/_/foo-svc")
		_, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: wi,
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("get success", func(t *testing.T) {
		wi := newScopedWorkloadIdentity("get-ok", grantedScope, grantedScope+"/_/get-svc")
		created, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: wi,
		})
		require.NoError(t, err)

		got, err := grantedSvc.GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name: "get-ok",
		})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(created, got, protocmp.Transform()))
	})

	t.Run("get from another scope returns not found", func(t *testing.T) {
		_, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: newScopedWorkloadIdentity("get-crossscope", grantedScope, grantedScope+"/_/x-svc"),
		})
		require.NoError(t, err)

		// otherUser has WorkloadIdentity read in /scopes/other, so it passes the
		// maybe-check, but must not be able to see a resource in /scopes/granted.
		_, err = otherSvc.GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name: "get-crossscope",
		})
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})

	t.Run("update rejects scope transition", func(t *testing.T) {
		created, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: newScopedWorkloadIdentity("update-transition", grantedScope, grantedScope+"/_/u-svc"),
		})
		require.NoError(t, err)

		// Attempt to move it to a different scope.
		moved := newScopedWorkloadIdentity("update-transition", otherScope, otherScope+"/_/u-svc")
		moved.Metadata.Revision = created.GetMetadata().GetRevision()
		_, err = grantedSvc.UpdateWorkloadIdentity(ctx, &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
			WorkloadIdentity: moved,
		})
		// The declared scope /scopes/other is not one the granted user can act
		// in, so this is denied before reaching the transition check.
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("update within scope succeeds", func(t *testing.T) {
		created, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: newScopedWorkloadIdentity("update-ok", grantedScope, grantedScope+"/_/orig"),
		})
		require.NoError(t, err)

		updated := newScopedWorkloadIdentity("update-ok", grantedScope, grantedScope+"/_/changed")
		updated.Metadata.Revision = created.GetMetadata().GetRevision()
		got, err := grantedSvc.UpdateWorkloadIdentity(ctx, &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
			WorkloadIdentity: updated,
		})
		require.NoError(t, err)
		require.Equal(t, grantedScope+"/_/changed", got.GetSpec().GetSpiffe().GetId())
	})

	t.Run("upsert rejects scopedness transition", func(t *testing.T) {
		// An unscoped admin can reach the transition check for any scope, so use
		// it to exercise the checkNoScopeTransition BadParameter directly:
		// create an unscoped resource, then attempt to upsert it as scoped.
		adminSvc := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(adminClient.GetConnection())
		_, err := adminSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "transition",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = adminSvc.UpsertWorkloadIdentity(ctx, &workloadidentityv1pb.UpsertWorkloadIdentityRequest{
			WorkloadIdentity: newScopedWorkloadIdentity("transition", grantedScope, grantedScope+"/_/t-svc"),
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
		require.ErrorContains(t, err, "scope of a workload_identity cannot be changed")
	})

	t.Run("delete success", func(t *testing.T) {
		_, err := grantedSvc.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: newScopedWorkloadIdentity("delete-ok", grantedScope, grantedScope+"/_/d-svc"),
		})
		require.NoError(t, err)

		_, err = grantedSvc.DeleteWorkloadIdentity(ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
			Name: "delete-ok",
		})
		require.NoError(t, err)

		_, err = srv.Auth().GetWorkloadIdentity(ctx, "delete-ok")
		require.True(t, trace.IsNotFound(err), "expected NotFound after delete, got %v", err)
	})
}
