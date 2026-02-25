package awsic

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// failFirstNDeleteAssignmentClient is an [icsdk.Client] that wraps an
// underlying client and makes the first N calls to DeleteAccountAssignment
// return a transient error, simulating a temporarily unavailable AWS API.
type failFirstNDeleteAssignmentClient struct {
	icsdk.Client
	mu          sync.Mutex
	failN       int
	deleteCount int
}

// DeleteAccountAssignment fails the first [failFirstNDeleteAssignmentClient.failN]
// calls and delegates subsequent calls to the underlying client.
func (c *failFirstNDeleteAssignmentClient) DeleteAccountAssignment(ctx context.Context, req *icsdk.DeleteAccountAssignmentRequest) (*icsdk.AccountAssignmentResponse, error) {
	c.mu.Lock()
	c.deleteCount++
	count := c.deleteCount
	c.mu.Unlock()

	if count <= c.failN {
		return nil, trace.ConnectionProblem(nil, "simulated transient failure deleting account assignment")
	}
	return c.Client.DeleteAccountAssignment(ctx, req)
}

// TestAccountAssignmentDeletionIsRetriedAfterFailure asserts that when deleting
// an account assignment from AWS Identity Center fails on the first attempt, the
// assignment provisioner retries the deletion and it eventually succeeds.
func TestAccountAssignmentDeletionIsRetriedAfterFailure(t *testing.T) {
	ctx := t.Context()

	// GIVEN a mock Identity Center state with a single user and no initial
	// account assignments
	awsState := icsdk.NewMockedAWSState(
		icsdk.WithUser("uid_alice", "alice"),
	)

	// GIVEN an IC API client configured to fail the first DeleteAccountAssignment
	// call, simulating a transient AWS API error
	unifiedClient := ictest.NewUnifiedMockClient(awsState)
	failingICClient := &failFirstNDeleteAssignmentClient{
		Client: unifiedClient.ViaAPI(),
		failN:  1,
	}
	setupMockAWSICEnvironment(t, failingICClient, unifiedClient.ViaSCIM())

	const (
		adminAccountID   = "1111111111"
		adminPermSetARN  = "arn:aws:sso:::permissionSet/Admin"
		icAssignmentRole = "ic-alice-admin"
	)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "admin", "editor"),
		// alice starts with a role that grants an IC account assignment
		common.WithUser(t, "alice", "requester", icAssignmentRole),
		common.WithRole(t, icAssignmentRole,
			common.WithAccountAssignment(types.Allow, adminAccountID, adminPermSetARN)),
	)

	auth := sut.Teleport.Process.GetAuthServer()
	mustSetupAWSIdentityCenterIntegration(t, sut.GetClusterClientForUser(t, "admin").AuthClient)

	// EXPECT the IC service to adopt alice and provision her account assignment
	// in AWS before we proceed with the deletion test
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assertPrincipalAssignment(ctx, t, auth, principal.GetIDForUserName("alice"),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasAccountAssignment(adminPermSetARN, adminAccountID),
			)

			assertICUser(ctx, t, unifiedClient, "alice",
				hasAccountAssignments(&icsdk.Assignment{
					AccountID:        "1111111111",
					PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
					PrincipalType:    ssoadmintypes.PrincipalTypeUser,
				}))
		},
		10*time.Second, 100*time.Millisecond,
		"Initial account assignment must be provisioned")

	// WHEN alice's IC assignment role is removed, triggering account assignment
	// deletion in AWS. The provisioner will compute a diff (desired: none; AWS:
	// has Admin/1111111111) and attempt to delete the assignment.
	mustUpdateUser(ctx, t, auth, "alice", func(u types.User) {
		u.SetRoles(slices.DeleteFunc(u.GetRoles(), func(r string) bool {
			return r == icAssignmentRole
		}))
	})

	// EXPECT that despite the first deletion attempt failing, the assignment
	// provisioner retries and the account assignment is eventually deleted from
	// AWS and that the Teleport principal assignment record is updated to
	// reflect the successfully-provisioned (empty) assignment state
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assertICUser(ctx, t, unifiedClient, "alice",
				hasAccountAssignments( /* none */ ))

			assertPrincipalAssignment(ctx, t, auth, principal.GetIDForUserName("alice"),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasNoAccountAssignments,
			)
		},
		10*time.Second, 100*time.Millisecond,
		"Account assignment deletion must be retried and eventually succeed")

	// ASSERT that the deletion was retried at least once, confirming that the
	// first attempt actually failed and the retry mechanism kicked in
	failingICClient.mu.Lock()
	defer failingICClient.mu.Unlock()
	require.Greater(t, failingICClient.deleteCount, 1,
		"Account assignment deletion must have been attempted more than once")
}
