package awsic

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestGroupMemberProvisioning(t *testing.T) {
	ctx := t.Context()

	// GIVEN a SCIM client configured to fail the first deletion attempt for
	// both users and groups to simulate a transient downstream error. This is
	// in order to assert that a failed provisioning will self-heal on a subsequent
	// provisioning attempt.
	unifiedClient := ictest.NewUnifiedMockClient(icsdk.NewMockedAWSState())
	setupMockAWSICEnvironment(t, unifiedClient.ViaAPI(), unifiedClient.ViaSCIM())

	// GIVEN a Teleport cluster with the Identity Center integration running in
	// full hand-off mode.
	allUsers := []string{"alice", "bob", "claire", "dave", "erica", "fred"}
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "admin", "editor"),
		common.WithRole(t, "aws-access-all-areas",
			common.WithAccountAssignment(types.Allow, "*", "*")),
		common.WithUser(t, "alice", "requester"),
		common.WithUser(t, "bob", "requester"),
		common.WithUser(t, "claire", "requester"),
		common.WithUser(t, "dave", "requester"),
		common.WithUser(t, "erica", "requester"),
		common.WithUser(t, "fred", "requester"),
	)
	auth := sut.Teleport.Process.GetAuthServer()
	aclService := sut.Teleport.Process.GetAuthServer().AccessListsInternal
	mustSetupAWSIdentityCenterIntegration(t, sut.GetClusterClientForUser(t, "admin").AuthClient)

	// EXPECT the IC service to start up and provision alice and the Group1
	// access list before we proceed with the deletion test
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// All users must be provisioned into AWS Identity Center
			for _, n := range allUsers {
				assertPrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(n),
					hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				)
			}
		},
		10*time.Second, 100*time.Millisecond,
		"Initial provisioning must complete")

	// WHEN I create a simple Access List
	simpleACL := common.CreateAccessList(t, sut,
		common.WithName("simple-access-list"),
		common.WithTitle("Simple Access List"),
		common.WithOwners("admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"aws-access-all-areas"}}),
		common.WithMembers("alice", "bob", "claire"))

	// EXPECT that a corresponding group is created in Identity Center
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requireICGroup(ctx, t, unifiedClient.ViaAPI(), simpleACL.Spec.Title,
			withMembers("alice", "bob", "claire"))
	}, 10*time.Second, 100*time.Millisecond)

	// WHEN I modify the Access Lists' members...
	members := common.GetAccessListMembers(t, sut, simpleACL.GetName())
	members = slices.DeleteFunc(members, func(m *accesslist.AccessListMember) bool {
		return m.Spec.MembershipKind == accesslist.MembershipKindUser &&
			m.Spec.Name == "bob"
	})
	members = append(members,
		common.NewAccessListMember(t, simpleACL.GetName(), "dave", accesslist.MembershipKindUser),
		common.NewAccessListMember(t, simpleACL.GetName(), "fred", accesslist.MembershipKindUser))
	var err error
	simpleACL, _, err = aclService.UpsertAccessListWithMembers(ctx, simpleACL, members)
	require.NoError(t, err)

	// EXPECT that a corresponding group member list is updated in Identity Center
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requireICGroup(ctx, t, unifiedClient.ViaAPI(), simpleACL.Spec.Title,
			withMembers("alice", "claire", "dave", "fred"))
	}, 10*time.Second, 100*time.Millisecond)
}

type patchFailingSCIMClient struct {
	scimsdk.Client
}

// PatchGroupMembers is rigged to fail on every [failRate]'th request request
func (pfsc *patchFailingSCIMClient) PatchGroupMembers(ctx context.Context, groupID string, toAdd, toRemove []*scimsdk.GroupMember) error {
	return &scimsdk.InvalidMemberError{
		Candidates: sliceutils.Map(toAdd, (*scimsdk.GroupMember).GetExternalID),
	}
}

// TestGroupMemberProvisioningRecordsFailedPatch asserts that failed patch
func TestGroupMemberProvisioningRecordsFailedPatch(t *testing.T) {
	ctx := t.Context()

	// GIVEN a SCIM client configured to fail when patching a member list
	unifiedClient := ictest.NewUnifiedMockClient(icsdk.NewMockedAWSState())
	scimClient := &patchFailingSCIMClient{
		Client: unifiedClient.ViaSCIM(),
	}
	setupMockAWSICEnvironment(t, unifiedClient.ViaAPI(), scimClient)

	// GIVEN a Teleport cluster with the Identity Center integration running in
	// full hand-off mode.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "admin", "editor"),
		common.WithRole(t, "aws-access-all-areas",
			common.WithAccountAssignment(types.Allow, "*", "*")),
	)

	// GIVEN some Teleport Users
	allUsers := []string{"alice", "bob", "claire", "dave", "erica", "fred"}
	for _, userName := range allUsers {
		common.MustCreateUser(t, sut, userName, "requester")
	}

	// GIVEN an Identity Center integration running in full-handoff mode
	mustSetupAWSIdentityCenterIntegration(t,
		sut.GetClusterClientForUser(t, "admin").AuthClient)

	// EXPECT the IC service to start up and provision alice and the Group1
	// access list before we proceed with the deletion test
	auth := sut.Teleport.Process.GetAuthServer()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// All users must be provisioned into AWS Identity Center
			for _, n := range allUsers {
				assertPrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(n),
					hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				)
			}
		},
		10*time.Second, 100*time.Millisecond,
		"Initial provisioning must complete")

	// WHEN I create an Access List containing all users above
	simpleACL := common.CreateAccessList(t, sut,
		common.WithName("simple-access-list"),
		common.WithTitle("Simple Access List"),
		common.WithOwners("admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"aws-access-all-areas"}}),
		common.WithMembers(allUsers...))

	// EXPECT that
	//  - a corresponding group is created in Identity Center,
	//  - the member list patch has failed due to the rigged SCIM client
	//  - the error is recorded in the Access List's provisioning state
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			requireICGroup(ctx, t, unifiedClient.ViaAPI(), simpleACL.Spec.Title)
			requireSCIMProvisioningState(ctx, t, auth, provisioning.GetIDForAccessList(simpleACL),
				hasSCIMProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE),
				hasSCIMErrorMatching("Attempted to add an invalid or obsolete user to a group."))
		},
		10*time.Second, 100*time.Millisecond,
		"AccessList provisioning error must be recorded")
}

// failFirstNDeletesSCIMClient is a [scimsdk.Client] that wraps an underlying
// client and makes the first N calls to DeleteUser and DeleteGroup return a
// transient error, simulating a temporarily unavailable downstream system.
type failFirstNDeletesSCIMClient struct {
	scimsdk.Client
	mu               sync.Mutex
	failN            int
	userDeleteCount  int
	groupDeleteCount int
}

// DeleteUser fails the first [failFirstNDeletesSCIMClient.failN] calls and
// delegates subsequent calls to the underlying client.
func (c *failFirstNDeletesSCIMClient) DeleteUser(ctx context.Context, id string) error {
	c.mu.Lock()
	c.userDeleteCount++
	count := c.userDeleteCount
	c.mu.Unlock()

	if count <= c.failN {
		return trace.ConnectionProblem(nil, "simulated transient failure deleting user %q", id)
	}
	return c.Client.DeleteUser(ctx, id)
}

// DeleteGroup fails the first [failFirstNDeletesSCIMClient.failN] calls and
// delegates subsequent calls to the underlying client.
func (c *failFirstNDeletesSCIMClient) DeleteGroup(ctx context.Context, id string) error {
	c.mu.Lock()
	c.groupDeleteCount++
	count := c.groupDeleteCount
	c.mu.Unlock()

	if count <= c.failN {
		return trace.ConnectionProblem(nil, "simulated transient failure deleting group %q", id)
	}
	return c.Client.DeleteGroup(ctx, id)
}

// TestDeletionIsRetriedAfterFailure asserts that when deleting a user or group
// from Identity Center fails on the first attempt, the provisioning service
// retries the deletion and it eventually succeeds.
func TestDeletionIsRetriedAfterFailure(t *testing.T) {
	ctx := t.Context()

	// GIVEN a mock Identity Center state with a single user and group
	awsState := icsdk.NewMockedAWSState(
		icsdk.WithUser("uid_alice", "alice"),
		icsdk.WithGroup("group1", "Group1", "uid_alice"),
	)

	// GIVEN a SCIM client configured to fail the first deletion attempt for
	// both users and groups, simulating a transient downstream error
	unifiedClient := ictest.NewUnifiedMockClient(awsState)
	riggedClient := &failFirstNDeletesSCIMClient{
		Client: unifiedClient.ViaSCIM(),
		failN:  1,
	}
	setupMockAWSICEnvironment(t, unifiedClient.ViaAPI(), riggedClient)

	// GIVEN a Teleport cluster with the Identity Center integration running in
	// full hand-off mode.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "admin", "editor"),
		common.WithUser(t, "alice", "requester"),
	)
	auth := sut.Teleport.Process.GetAuthServer()
	mustSetupAWSIdentityCenterIntegration(t, sut.GetClusterClientForUser(t, "admin").AuthClient)

	// EXPECT the IC service to start up and provision alice and the Group1
	// access list before we proceed with the deletion test
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// alice must be adopted as a SCIM user with a known external ID
			assertPrincipalAssignment(ctx, t, auth, principal.GetIDForUserName("alice"),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasExternalID("uid_alice"),
			)

			// Group1 must be imported as an access list and adopted as a SCIM group
			acl, err := getAccessListByTitle(ctx, auth, "Group1")
			require.NoError(t, err)
			assertPrincipalAssignment(ctx, t, auth, principal.GetIDForAccessList(acl),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasExternalID("group1"),
			)
		},
		10*time.Second, 100*time.Millisecond,
		"Initial provisioning must complete")

	// WHEN alice is deleted from Teleport, triggering a SCIM user deletion
	require.NoError(t, auth.DeleteUser(ctx, "alice"))

	// WHEN the Group1 access list is deleted from Teleport, triggering a SCIM
	// group deletion
	group1ACL := mustGetAccessListByTitle(ctx, t, auth, "Group1")
	require.NoError(t, auth.DeleteAccessList(ctx, group1ACL.GetName()))

	// EXPECT that despite the first deletion attempts failing, the provisioning
	// service retries the deletions and both alice and Group1 are eventually
	// removed from Identity Center
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assertSCIMUsers(ctx, t, riggedClient, "admin" /* note absence of user "alice" */)
			assertSCIMGroupsByDisplayName(ctx, t, riggedClient /* expect no groups */)
		},
		10*time.Second, 100*time.Millisecond,
		"Deletions must be retried and eventually succeed")

	// ASSERT that both deletions were retried at least once, confirming that
	// the first attempt actually failed and the retry mechanism kicked in
	riggedClient.mu.Lock()
	defer riggedClient.mu.Unlock()
	require.Greater(t, riggedClient.userDeleteCount, 1,
		"User deletion must have been attempted more than once")
	require.Greater(t, riggedClient.groupDeleteCount, 1,
		"Group deletion must have been attempted more than once")
}
