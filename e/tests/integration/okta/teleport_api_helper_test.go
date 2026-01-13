package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

type roleAllowDesc roleConditionsDesc
type roleDenyDesc roleConditionsDesc

type roleConditionsDesc struct {
	groupLabels    types.Labels
	reviewRequests *types.AccessReviewConditions
}

func createRole(t *testing.T, sut *common.SUT, name string, allow roleAllowDesc, deny roleDenyDesc) types.Role {
	t.Helper()
	ctx := context.Background()
	authServer := sut.Teleport.Process.GetAuthServer()

	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			GroupLabels:    allow.groupLabels,
			ReviewRequests: allow.reviewRequests,
		},
		Deny: types.RoleConditions{
			GroupLabels:    deny.groupLabels,
			ReviewRequests: deny.reviewRequests,
		},
	}

	role, err := types.NewRole(name, spec)
	require.NoError(t, err, "types.NewRole")

	created, err := authServer.CreateRole(ctx, role)
	require.NoError(t, err, "authServer.CreateRole")

	return created
}

func assignRoles(t *testing.T, sut *common.SUT, user string, roles ...string) {
	t.Helper()
	ctx := t.Context()
	authServer := sut.Teleport.Process.GetAuthServer()

	u, err := authServer.GetUser(ctx, user, true)
	require.NoError(t, err, "authServer.GetUser")

	u.SetRoles(append(u.GetRoles(), roles...))
	_, err = authServer.UpdateUser(ctx, u)
	require.NoError(t, err, "authServer.UpdateUser")
}

func createAccessRequest(t *testing.T, sut *common.SUT, resourceName, resourceType, requesterUser string) types.AccessRequest {
	t.Helper()
	ctx := t.Context()
	authServer := sut.Teleport.Process.GetAuthServer()

	clusterName, err := authServer.GetClusterName(ctx)
	require.NoError(t, err, "autServer.GetClusterName")

	req := &types.AccessRequestV3{
		Metadata: types.Metadata{
			Name: uuid.New().String(),
		},
		Spec: types.AccessRequestSpecV3{
			User:          requesterUser,
			RequestReason: "Need access",
			RequestedResourceIDs: []types.ResourceID{
				{
					Kind:        resourceType,
					Name:        resourceName,
					ClusterName: clusterName.GetClusterName(),
				},
			},
		},
	}
	created, err := authServer.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.NoError(t, err, "authServer.CreateAccessRequestV2")
	return created
}

func approveAccessRequest(t *testing.T, sut *common.SUT, requestID string, user string) {
	t.Helper()
	auth := sut.Teleport.Process.GetAuthServer()
	ctx := t.Context()
	r, err := auth.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: requestID,
		Review: types.AccessReview{
			Author:        user,
			ProposedState: types.RequestState_APPROVED,
		},
	})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := auth.GetOktaAssignment(ctx, r.GetName())
		require.NoError(t, err)
	}, time.Second, time.Millisecond*100)
}
