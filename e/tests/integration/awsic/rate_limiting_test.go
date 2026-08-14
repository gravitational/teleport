package awsic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// concurrentMockICClient wraps the default mock Identity Center API client in
// order allow concurrent calls to CreateAccountAssignment, and to count the
// number of concurrent calls that actually happen in a test.
type concurrentMockICClient struct {
	*icsdk.ClientMock

	createAssignmentCount              int
	concurrentCreateAssignmentCalls    int
	maxConcurrentCreateAssignmentCalls int
}

// CreateAccountAssignment adds a new assignment based on the request parameters.
func (c *concurrentMockICClient) CreateAccountAssignment(ctx context.Context, req *icsdk.CreateAccountAssignmentRequest) (*icsdk.AccountAssignmentResponse, error) {
	c.Mu.Lock()
	c.createAssignmentCount++
	c.concurrentCreateAssignmentCalls++
	c.maxConcurrentCreateAssignmentCalls = max(c.concurrentCreateAssignmentCalls, c.maxConcurrentCreateAssignmentCalls)
	c.Mu.Unlock()

	defer func() {
		c.Mu.Lock()
		c.concurrentCreateAssignmentCalls--
		c.Mu.Unlock()
	}()

	// Make the asynchronous call take some appreciable amount of time, but
	// still bail out when asked.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(1 * time.Millisecond):
		// happy path, continue
	}

	return c.ClientMock.CreateAccountAssignment(ctx, req)
}

func (c *concurrentMockICClient) getCreateAssignmentCount() int {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.createAssignmentCount
}

func (c *concurrentMockICClient) getUserCount() int {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return len(c.Users)
}

func TestCreateAccountAssignmentRateLimiting(t *testing.T) {
	const userCount = 10
	const concurrentCreateAccountAssignmentLimit = 15
	ctx := t.Context()

	// GIVEN an AWS configuration with various accounts and permission sets
	awsState := icsdk.NewMockedAWSState(
		icsdk.WithAccounts(
			&icsdk.Account{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"},
			&icsdk.Account{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"},
			&icsdk.Account{Name: "Account3", ID: "3333333333", ARN: "arn:aws:iam::2222222222:account/Account3"},
			&icsdk.Account{Name: "Account4", ID: "4444444444", ARN: "arn:aws:iam::2222222222:account/Account4"},
			&icsdk.Account{Name: "Account5", ID: "5555555555", ARN: "arn:aws:iam::2222222222:account/Account5"},
		),
		icsdk.WithPermissionSets(
			&icsdk.PermissionSet{Name: "PS01", Description: "PS 01", ARN: "arn:aws:sso:::permissionSet/ps-01"},
			&icsdk.PermissionSet{Name: "PS02", Description: "PS 02", ARN: "arn:aws:sso:::permissionSet/ps-02"},
			&icsdk.PermissionSet{Name: "PS03", Description: "PS 03", ARN: "arn:aws:sso:::permissionSet/ps-03"},
			&icsdk.PermissionSet{Name: "PS04", Description: "PS 04", ARN: "arn:aws:sso:::permissionSet/ps-04"},
			&icsdk.PermissionSet{Name: "PS05", Description: "PS 05", ARN: "arn:aws:sso:::permissionSet/ps-05"},
			&icsdk.PermissionSet{Name: "PS06", Description: "PS 06", ARN: "arn:aws:sso:::permissionSet/ps-06"},
			&icsdk.PermissionSet{Name: "PS07", Description: "PS 07", ARN: "arn:aws:sso:::permissionSet/ps-07"},
			&icsdk.PermissionSet{Name: "PS08", Description: "PS 08", ARN: "arn:aws:sso:::permissionSet/ps-08"},
			&icsdk.PermissionSet{Name: "PS09", Description: "PS 09", ARN: "arn:aws:sso:::permissionSet/ps-09"},
			&icsdk.PermissionSet{Name: "PS10", Description: "PS 10", ARN: "arn:aws:sso:::permissionSet/ps-10"},
		))

	// GIVEN a Teleport cluster  with a nontrivial number of users...
	client := ictest.NewUnifiedMockClient(awsState)
	mockAPIClient := concurrentMockICClient{ClientMock: &client.ClientMock}
	setupMockAWSICEnvironment(t, &mockAPIClient, client.ViaSCIM())

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "aws-access-all-areas", common.WithAccountAssignment(types.Allow, "*", "*")),
		common.WithUser(t, "admin", "editor"),
	)
	users := make([]types.User, 0, userCount)
	for i := range userCount {
		users = append(users, common.MustCreateUser(t, sut, fmt.Sprintf("user-%03d", i)))
	}

	// GIVEN a running Identity Center integration
	mustSetupAWSIdentityCenterIntegration(t, sut.GetClusterClientForUser(t, "admin").AuthClient,
		withRolesSyncMode(types.AWSICRolesSyncModeNone),
		withGroupSyncFilterExclude("*"))
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			require.Equal(t, userCount+1, mockAPIClient.getUserCount())
		},
		30*time.Second, 300*time.Millisecond,
		"Initial state setup failed or timed out")

	// WHEN I update all users to hold a role that grants them multiple account
	// assignments each...
	auth := sut.Teleport.Process.GetAuthServer()
	for i, user := range users {
		user.AddRole("aws-access-all-areas")
		updatedUser, err := auth.UpdateUser(ctx, user)
		require.NoError(t, err)
		users[i] = updatedUser
	}

	// EXPECT that account assignments for all AWS Permission Sets on all
	// AWS accounts will be created for each user
	expectedAssignmentCount := userCount * len(awsState.Accounts) * len(awsState.PermissionSets)
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			require.Equal(t, expectedAssignmentCount, mockAPIClient.getCreateAssignmentCount())
		},
		10*time.Second, 100*time.Millisecond,
		"Account assignment creation failed or timed out")

	// EXPECT that there are never more than [concurrentCreateAccountAssignmentLimit]
	// createAccountAssignment calls in-flight at any time.
	require.LessOrEqual(t,
		mockAPIClient.maxConcurrentCreateAssignmentCalls,
		concurrentCreateAccountAssignmentLimit)
}
