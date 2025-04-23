// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services"
)

func newTestBackend(t *testing.T, ctx context.Context, clock clockwork.Clock) backend.Backend {
	t.Helper()
	sqliteBackend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqliteBackend.Close())
	})
	return sqliteBackend
}

func TestIdentityCenterResourceCRUD(t *testing.T) {
	t.Parallel()

	const resourceID = "alpha"

	testCases := []struct {
		name               string
		createResource     func(*testing.T, context.Context, services.IdentityCenter, string) types.Resource153
		getResource        func(context.Context, services.IdentityCenter, string) (types.Resource153, error)
		updateResource     func(context.Context, services.IdentityCenter, types.Resource153) (types.Resource153, error)
		upsertResource     func(context.Context, services.IdentityCenter, types.Resource153) (types.Resource153, error)
		deleteAllResources func(context.Context, services.IdentityCenter) (*emptypb.Empty, error)
	}{
		{
			name: "Account",
			createResource: func(subtestT *testing.T, subtestCtx context.Context, svc services.IdentityCenter, id string) types.Resource153 {
				return makeTestIdentityCenterAccount(subtestT, subtestCtx, svc, id)
			},
			getResource: func(subtestCtx context.Context, svc services.IdentityCenter, id string) (types.Resource153, error) {
				return svc.GetIdentityCenterAccount(subtestCtx, services.IdentityCenterAccountID(id))
			},
			updateResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				acct := r.(services.IdentityCenterAccount)
				return svc.UpdateIdentityCenterAccount(subtestCtx, acct)
			},
			upsertResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				acct := r.(services.IdentityCenterAccount)
				return svc.UpsertIdentityCenterAccount(subtestCtx, acct)
			},
			deleteAllResources: func(subtestCtx context.Context, svc services.IdentityCenter) (*emptypb.Empty, error) {
				return svc.DeleteAllIdentityCenterAccounts(subtestCtx, &identitycenterv1.DeleteAllIdentityCenterAccountsRequest{})
			},
		},
		{
			name: "PermissionSet",
			createResource: func(subtestT *testing.T, subtestCtx context.Context, svc services.IdentityCenter, id string) types.Resource153 {
				return makeTestIdentityCenterPermissionSet(subtestT, subtestCtx, svc, id)
			},
			getResource: func(subtestCtx context.Context, svc services.IdentityCenter, id string) (types.Resource153, error) {
				return svc.GetPermissionSet(subtestCtx, services.PermissionSetID(id))
			},
			updateResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				ps := r.(*identitycenterv1.PermissionSet)
				return svc.UpdatePermissionSet(subtestCtx, ps)
			},
			deleteAllResources: func(subtestCtx context.Context, svc services.IdentityCenter) (*emptypb.Empty, error) {
				return svc.DeleteAllPermissionSets(subtestCtx, &identitycenterv1.DeleteAllPermissionSetsRequest{})
			},
		},
		{
			name: "AccountAssignment",
			createResource: func(subtestT *testing.T, subtestCtx context.Context, svc services.IdentityCenter, id string) types.Resource153 {
				return makeTestIdentityCenterAccountAssignment(subtestT, subtestCtx, svc, id)
			},
			getResource: func(subtestCtx context.Context, svc services.IdentityCenter, id string) (types.Resource153, error) {
				return svc.GetAccountAssignment(subtestCtx, services.IdentityCenterAccountAssignmentID(id))
			},
			updateResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				asmt := r.(services.IdentityCenterAccountAssignment)
				return svc.UpdateAccountAssignment(subtestCtx, asmt)
			},
			upsertResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				asmt := r.(services.IdentityCenterAccountAssignment)
				return svc.UpsertAccountAssignment(subtestCtx, asmt)
			},
			deleteAllResources: func(subtestCtx context.Context, svc services.IdentityCenter) (*emptypb.Empty, error) {
				return svc.DeleteAllAccountAssignments(subtestCtx, &identitycenterv1.DeleteAllAccountAssignmentsRequest{})
			},
		},
		{
			name: "PrincipalAssignment",
			createResource: func(subtestT *testing.T, subtestCtx context.Context, svc services.IdentityCenter, id string) types.Resource153 {
				return makeTestIdentityCenterPrincipalAssignment(subtestT, subtestCtx, svc, id)
			},
			getResource: func(subtestCtx context.Context, svc services.IdentityCenter, id string) (types.Resource153, error) {
				return svc.GetPrincipalAssignment(subtestCtx, services.PrincipalAssignmentID(id))
			},
			updateResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				asmt := r.(*identitycenterv1.PrincipalAssignment)
				return svc.UpdatePrincipalAssignment(subtestCtx, asmt)
			},
			upsertResource: func(subtestCtx context.Context, svc services.IdentityCenter, r types.Resource153) (types.Resource153, error) {
				asmt := r.(*identitycenterv1.PrincipalAssignment)
				return svc.UpsertPrincipalAssignment(subtestCtx, asmt)
			},
			deleteAllResources: func(subtestCtx context.Context, svc services.IdentityCenter) (*emptypb.Empty, error) {
				return svc.DeleteAllPrincipalAssignments(subtestCtx, &identitycenterv1.DeleteAllPrincipalAssignmentsRequest{})
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Run("OptimisticLocking", func(t *testing.T) {
				const resourceID = "alpha"

				ctx := newTestContext(t)
				clock := clockwork.NewFakeClock()
				backend := newTestBackend(t, ctx, clock)

				// GIVEN an IdentityCenter service populated with a resource
				uut, err := NewIdentityCenterService(IdentityCenterServiceConfig{Backend: backend})
				require.NoError(t, err)
				createdResource := test.createResource(t, ctx, uut, resourceID)

				// WHEN I modify the backend record for that resource...
				tmpResource, err := test.getResource(ctx, uut, resourceID)
				require.NoError(t, err)
				tmpResource.GetMetadata().Labels = map[string]string{"update": "1"}
				updatedResource, err := test.updateResource(ctx, uut, tmpResource)
				require.NoError(t, err)

				// EXPECT that any attempt to update backend via the original in-memory
				// version of the resource fails with a comparison error
				createdResource.GetMetadata().Labels = map[string]string{"update": "2"}
				_, err = test.updateResource(ctx, uut, createdResource)
				require.True(t, trace.IsCompareFailed(err), "expected a compare failed error, got %T (%s)", err, err)

				// EXPECT that the backend still reflects the first updated revision,
				// rather than failed update
				r, err := test.getResource(ctx, uut, resourceID)
				require.NoError(t, err)
				require.Equal(t, "1", r.GetMetadata().Labels["update"])

				// WHEN I attempt update the backend via the latest revision of the
				// record...
				updatedResource.GetMetadata().Labels["update"] = "3"
				_, err = test.updateResource(ctx, uut, updatedResource)

				// EXPECT the update to succeed, and the backend record to have been
				// updated
				require.NoError(t, err)
				r, err = test.getResource(ctx, uut, resourceID)
				require.NoError(t, err)
				require.Equal(t, "3", r.GetMetadata().Labels["update"])
			})

			t.Run("UnconditionalUpsert", func(t *testing.T) {
				t.Parallel()

				if test.upsertResource == nil {
					t.Skip(test.name + " does not support unconditional upsert")
				}

				ctx := newTestContext(t)
				clock := clockwork.NewFakeClock()
				backend := newTestBackend(t, ctx, clock)

				// GIVEN an IdentityCenter service populated with a resource
				uut, err := NewIdentityCenterService(IdentityCenterServiceConfig{Backend: backend})
				require.NoError(t, err)
				originalResource := test.createResource(t, ctx, uut, resourceID)

				// GIVEN that the backend record for that resource has been changed
				// between us looking up the original resource and us committing
				// any changes to it...
				tmpResource, err := test.getResource(ctx, uut, resourceID)
				require.NoError(t, err)
				tmpResource.GetMetadata().Labels = map[string]string{"update": "1"}
				_, err = test.updateResource(ctx, uut, tmpResource)
				require.NoError(t, err)

				// WHEN I attempt to Update the modified original resource
				_, err = test.updateResource(ctx, uut, originalResource)

				// EXPECT the Update to fail due to the changed underlying record
				require.True(t, trace.IsCompareFailed(err), "expected a compare failed error, got %T (%s)", err, err)

				// WHEN I attempt to Upsert the modified original resource
				originalResource.GetMetadata().Labels = map[string]string{"update": "2"}
				_, err = test.upsertResource(ctx, uut, originalResource)

				// EXPECT that an upsert will succeed, even though the underlying
				// record has changed
				require.NoError(t, err)

				// EXPECT that the backend reflects the updated values from the
				// upsert
				r, err := test.getResource(ctx, uut, resourceID)
				require.NoError(t, err)
				require.Equal(t, "2", r.GetMetadata().Labels["update"])
			})

			t.Run("DeleteAllResources", func(t *testing.T) {
				t.Parallel()

				ctx := newTestContext(t)
				clock := clockwork.NewFakeClock()
				backend := newTestBackend(t, ctx, clock)
				defer backend.Close()

				// GIVEN an IdentityCenter service populated with a resource
				uut, err := NewIdentityCenterService(IdentityCenterServiceConfig{Backend: backend})
				require.NoError(t, err)

				resourceTestNames := []string{"r1", "r2"}
				for _, v := range resourceTestNames {
					test.createResource(t, ctx, uut, v)
				}

				// EXPECT that the backend records for the resources created above can be fetched
				var resourceNamesFromBackend []string
				for _, v := range resourceTestNames {
					r, err := test.getResource(ctx, uut, v)
					require.NoError(t, err)
					resourceNamesFromBackend = append(resourceNamesFromBackend, r.GetMetadata().GetName())
				}
				require.ElementsMatch(t, resourceTestNames, resourceNamesFromBackend)

				// WHEN I attempt to Delete resources
				_, err = test.deleteAllResources(ctx, uut)
				require.NoError(t, err)

				// EXPECT that the backend reflects the resource were deleted.
				for _, v := range resourceTestNames {
					_, err := test.getResource(ctx, uut, v)
					require.ErrorContains(t, err, "doesn't exist")
				}
			})
		})
	}
}

func makeTestIdentityCenterAccount(t *testing.T, ctx context.Context, svc services.IdentityCenter, id string) services.IdentityCenterAccount {
	t.Helper()
	created, err := svc.CreateIdentityCenterAccount(ctx, services.IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:     types.KindIdentityCenterAccount,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: id},
			Spec: &identitycenterv1.AccountSpec{
				Id:          "aws-account-id-" + id,
				Arn:         fmt.Sprintf("arn:aws:sso::%s:", id),
				Description: "Test account " + id,
				PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
					{
						Name: "dummy",
						Arn:  "arn:aws:sso:::permissionSet/ic-instance/ps-instance",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	return created
}

func makeTestIdentityCenterPermissionSet(t *testing.T, ctx context.Context, svc services.IdentityCenter, id string) *identitycenterv1.PermissionSet {
	t.Helper()
	created, err := svc.CreatePermissionSet(ctx, &identitycenterv1.PermissionSet{
		Kind:     types.KindIdentityCenterPermissionSet,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: id},
		Spec: &identitycenterv1.PermissionSetSpec{
			Arn:         fmt.Sprintf("arn:aws:sso:::permissionSet/ic-instance/%s", id),
			Name:        "aws-permission-set-" + id,
			Description: "Test permission set " + id,
		},
	})
	require.NoError(t, err)
	return created
}

func makeTestIdentityCenterAccountAssignment(t *testing.T, ctx context.Context, svc services.IdentityCenter, id string) services.IdentityCenterAccountAssignment {
	t.Helper()
	created, err := svc.CreateAccountAssignment(ctx, services.IdentityCenterAccountAssignment{
		AccountAssignment: &identitycenterv1.AccountAssignment{
			Kind:     types.KindIdentityCenterAccountAssignment,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: id},
			Spec: &identitycenterv1.AccountAssignmentSpec{
				Display: "Some-Permission-set on Some-AWS-account",
				PermissionSet: &identitycenterv1.PermissionSetInfo{
					Arn:  "arn:aws:sso:::permissionSet/ic-instance/ps-instance",
					Name: "some permission set",
				},
				AccountName: "Some Account Name",
				AccountId:   "some account id",
			},
		},
	})
	require.NoError(t, err)
	return created
}

func makeTestIdentityCenterPrincipalAssignment(t *testing.T, ctx context.Context, svc services.IdentityCenterPrincipalAssignments, id string) *identitycenterv1.PrincipalAssignment {
	t.Helper()
	created, err := svc.CreatePrincipalAssignment(ctx, &identitycenterv1.PrincipalAssignment{
		Kind:     types.KindIdentityCenterPrincipalAssignment,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: id},
		Spec: &identitycenterv1.PrincipalAssignmentSpec{
			PrincipalType:    identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
			PrincipalId:      id,
			ExternalIdSource: "scim",
			ExternalId:       "some external id",
		},
		Status: &identitycenterv1.PrincipalAssignmentStatus{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
		},
	})
	require.NoError(t, err)
	return created
}
