package identitycenter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
)

func TestUserCreation(t *testing.T) {
	ctx := t.Context()

	// GIVEN a running cluster...
	fixture := ictest.NewFixture(t, ictest.WithStartedCache)
	fixture.CreatePluginResource(t, ictest.WithoutImport)
	_, stopService := runNewTestService(t, ctx, fixture)
	defer stopService()

	// EXPECT that the AWS resource sync will eventually complete at least one
	// pass, indicating that the server is up and running.
	downstream := fixture.SCIMClient
	serviceIsReady := func(c *assert.CollectT) {
		// If Account assignments representing the cartesian product of the IC-
		// managed AWS accounts and permission sets exists in our local service,
		// then we can assume that the resource sync service is up and has
		// completed at least one reconciliation.
		expectedAccountAssignments := len(fixture.ICClient.Accounts) * len(fixture.ICClient.PermissionSets)
		if !ictest.AssertSequenceLength(c, expectedAccountAssignments, iter.AllAccountAssignments(ctx, fixture.Auth)) {
			return
		}
	}
	require.EventuallyWithT(t, serviceIsReady, 10*time.Second, time.Second)

	// GIVEN a role that grants the holder access to all permission sets on all
	// accounts
	accessAllAreas, err := fixture.Auth.CreateRole(ctx, ictest.AccountAssignmentRole{
		Name:             "access-all-areas",
		AccountID:        "*",
		PermissionSetARN: "*",
	}.Build(t))
	require.NoError(t, err)

	// WHEN I create a new user that holds the above role...
	const userPaul = "paul@atreides.ar"
	paul, err := types.NewUser(userPaul)
	require.NoError(t, err)

	paul.AddRole(accessAllAreas.GetName())
	_, err = fixture.Auth.Services.CreateUser(ctx, paul)
	require.NoError(t, err)

	// EXPECT that the user and their account assignments will all eventually be
	// provisioned into AWS
	userAndAccountAssignmentsExist := func(c *assert.CollectT) {
		extUser, err := downstream.GetUserByUserName(ctx, userPaul)
		if !assert.NoError(c, err) {
			return
		}

		assignments, err := fixture.ICClient.ListUserAssignments(ctx, extUser.ID)
		if !assert.NoError(c, err) {
			return
		}
		assert.Len(c, assignments, 4)
	}
	require.EventuallyWithT(t, userAndAccountAssignmentsExist, 10*time.Second, time.Second)
}
