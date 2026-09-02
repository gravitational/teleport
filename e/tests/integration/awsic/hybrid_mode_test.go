package awsic

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

// userTrackingSCIMClient is a mock SCIM client that records any operation on a
// user
type userTrackingSCIMClient struct {
	scimsdk.Client
	mu      sync.Mutex
	creates []*scimsdk.User
	updates []*scimsdk.User
	deletes []string
}

func getUserName(u *scimsdk.User) string {
	return u.UserName
}

func (c *userTrackingSCIMClient) requireNoUserOperations(t require.TestingT) {
	c.mu.Lock()
	defer c.mu.Unlock()
	require.Empty(t, c.creates, "No users should be created: %v", sliceutils.Map(c.creates, getUserName))
	require.Empty(t, c.updates, "No users should be updated: %v", sliceutils.Map(c.updates, getUserName))
	require.Empty(t, c.deletes, "No users should be deleted: %v", c.updates)
}

// CreateUser creates a new user.
func (s *userTrackingSCIMClient) CreateUser(ctx context.Context, user *scimsdk.User) (*scimsdk.User, error) {
	s.mu.Lock()
	s.creates = append(s.creates, user)
	s.mu.Unlock()
	return s.Client.CreateUser(ctx, user)
}

// DeleteUser deletes a user.
func (s *userTrackingSCIMClient) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	s.deletes = append(s.deletes, id)
	s.mu.Unlock()
	return s.Client.DeleteUser(ctx, id)
}

func TestUsersAreNotUpdatedInHybridMode(t *testing.T) {
	ctx := t.Context()
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stderr, logutils.SlogTextHandlerConfig{Level: slog.LevelDebug})))

	awsState := icsdk.NewMockedAWSState(
		icsdk.WithUser("uid_bob", "bob", icsdk.WithDisplayName("Bob Bobovitch")),
		icsdk.WithUser("uid_charlotte", "charlotte", icsdk.WithDisplayName("Charlotte D. Spider")),
		icsdk.WithUser("uid_dave", "dave", icsdk.WithDisplayName("Dave Davidson")),
		icsdk.WithUser("uid_emily", "emily", icsdk.WithDisplayName("Emily Emiliasdóttir")),
		icsdk.WithGroup("group1", "Group1", "uid_bob", "uid_charlotte"),
		icsdk.WithPermissionSets([]*icsdk.PermissionSet{
			{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
			{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
			{Name: "DataScientist", ARN: "arn:aws:sso:::permissionSet/DataScientist", Description: "Data Scientist permissions"},
		}...),
	)

	unifiedClient := ictest.NewUnifiedMockClient(awsState)
	mockSCIM := &userTrackingSCIMClient{Client: unifiedClient.ViaSCIM()}
	setupMockAWSICEnvironment(t, unifiedClient.ViaAPI(), mockSCIM)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		// A Teleport admin account
		common.WithUser(t, "admin", "editor"),

		// Users that have a corresponding AWS IC user
		common.WithUser(t, "bob", "requester", "admin-on-account1-1111111111", "readonly-on-account1-1111111111"),
		common.WithUser(t, "charlotte", "requester"),
		common.WithUser(t, "dave", "requester"),
		common.WithUser(t, "emily", "requester", "admin-on-account1-1111111111"),

		// Users without a corresponding AWS IC user
		common.WithUser(t, "zelda", "requester", "datascientist-on-account2-2222222222"),
	)
	auth := sut.Teleport.Process.GetAuthServer()
	adminUser := sut.GetClusterClientForUser(t, "admin")

	// GIVEN some expected properties of our users
	expectedUsers := []icsdk.User{
		{ID: "uid_bob", UserName: "bob"},
		{ID: "uid_charlotte", UserName: "charlotte"},
		{ID: "uid_dave", UserName: "dave"},
		{ID: "uid_emily", UserName: "emily"},
	}

	provisioningWatcher := sut.NewResourceWatcher(t, types.KindProvisioningPrincipalState)
	assignmentWatcher := sut.NewResourceWatcher(t, types.KindIdentityCenterPrincipalAssignment)
	assignmentAssertions := map[string][]principalAssignmentAssertion{
		"bob": {
			hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
			hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "1111111111"),
		},
		"emily": {
			hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
		},
	}

	// WHEN I create an Identity Center integration with an empty SamlIdpServiceProviderName,
	// which will trigger the plugin to rely on an external service to provision
	// users into IC, aka "hybrid mode"
	mustCreateAWSICPlugin(t, adminUser.AuthClient,
		withSAMLProviderName(""),
		withDefaultAccessListOwners("admin"))

	// EXPECT the plugin to start up, and that eventually all of the currently-
	// existing IC users are adopted by Teleport
	expectedPrincipalAssignments := make([]func(*identitycenterv1.PrincipalAssignment) bool, len(expectedUsers))
	expectedSCIMProvisioningStates := make([]func(*provisioningv1.PrincipalState) bool, len(expectedUsers))
	for i, icUser := range expectedUsers {
		expectedSCIMProvisioningStates[i] = scimProvisioningState(
			hasSCIMUserPrincipalID(icUser.UserName),
			hasSCIMProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
			hasSCIMExternalID(icUser.ID))

		expectedPrincipalAssignments[i] = principalAssignment(
			append(
				assignmentAssertions[icUser.UserName],
				hasUserPrincipalID(icUser.UserName),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasExternalID(icUser.ID),
			)...)
	}
	waitForAllSCIMProvisioningStates(t, provisioningWatcher, expectedSCIMProvisioningStates...)
	waitForAllPrincipalAssignments(t, assignmentWatcher, expectedPrincipalAssignments...)

	// EXPECT that bob's account assignments are provisioned into the remote IC
	// instance
	require.ElementsMatch(t,
		[]*icsdk.Assignment{
			&icsdk.Assignment{
				AccountID:        "1111111111",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
				PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			},
			&icsdk.Assignment{
				AccountID:        "1111111111",
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
				PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			},
		},
		getRemoteAccountAssignments(unifiedClient, "uid_bob"),
		"Bob's account assignments must be provisioned")

	// EXPECT that emily's account assignments are provisioned into the remote IC
	// instance
	require.ElementsMatch(t,
		[]*icsdk.Assignment{
			&icsdk.Assignment{
				AccountID:        "1111111111",
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
				PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			},
		},
		getRemoteAccountAssignments(unifiedClient, "uid_emily"),
		"Emily's account assignments must be provisioned")

	// EXPECT that the `zelda` Teleport user, who has no corresponding IC user,
	// is still sitting as "STALE" with no known external id
	requireSCIMProvisioningState(ctx, t, auth, provisioning.GetIDForUserName("zelda"),
		hasSCIMProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE),
		hasSCIMExternalID(""))

	// WHEN I update a Teleport user in a way that would normally cause a change in AWS
	mustUpdateUser(ctx, t, auth, "charlotte", func(u types.User) {
		u.SetTraits(map[string][]string{
			"okta/givenName":   {"Charlotte"},
			"okta/familyName":  {"Sometimes"},
			"okta/displayName": {"Charlotte Sometimes"},
		})
	})
	// EXPECT that the IC user will *not* have their name(s) updated (we can't
	// assert a negative here, but we will check the attributes of all users at
	// the end of the test to assert that we haven't tampered with them)

	// WHEN I delete a Teleport user
	require.NoError(t, auth.DeleteUser(ctx, "bob"))

	// EXPECT that the user's local state will be deleted, but that the corresponding
	// IC user still exists in the downstream IC instance
	waitForPrincipalAssignmentDeletion(t, assignmentWatcher, principal.GetIDForUserName("bob"))
	waitForSCIMProvisioningStateDeletion(t, provisioningWatcher, provisioning.GetIDForUserName("bob"))
	requireSCIMUsers(ctx, t, unifiedClient.ViaSCIM(), "bob", "charlotte", "dave", "emily")

	// WHEN I create an IC user for "zelda"
	unifiedClient.AddUserToState("uid_zelda", "zelda")

	// EXPECT that Teleport has adopted the new Identity Center user and correctly
	// bound them to the corresponding Teleport users.
	waitForSCIMProvisioningState(t, provisioningWatcher,
		hasSCIMUserPrincipalID("zelda"),
		hasSCIMProvisioningState(provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
		hasSCIMExternalID("uid_zelda"))
	waitForPrincipalAssignment(t, assignmentWatcher,
		hasUserPrincipalID("zelda"),
		hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
		hasExternalID("uid_zelda"),
		hasAccountAssignment("arn:aws:sso:::permissionSet/DataScientist", "2222222222"))

	// EXPECT that no user-modifying SCIM operations have been attempted
	mockSCIM.requireNoUserOperations(t)

	// EXPECT that we haven't made any discernable changes to the Identity Center
	// users
	expectedAWSState := icsdk.NewMockedAWSState(
		icsdk.WithUser("uid_bob", "bob", icsdk.WithDisplayName("Bob Bobovitch")),
		icsdk.WithUser("uid_charlotte", "charlotte", icsdk.WithDisplayName("Charlotte D. Spider")),
		icsdk.WithUser("uid_dave", "dave", icsdk.WithDisplayName("Dave Davidson")),
		icsdk.WithUser("uid_emily", "emily", icsdk.WithDisplayName("Emily Emiliasdóttir")),
		icsdk.WithUser("uid_zelda", "zelda"))
	require.ElementsMatch(t, expectedAWSState.Users, unifiedClient.Users,
		"No IC user records should be harmed in the running of this test")
}

// getRemoteAccountAssignments fetches the account assignments for a given user
// as recorded in the mock AWS back-end.
func getRemoteAccountAssignments(c *ictest.UnifiedClientMock, username string) []*icsdk.Assignment {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return slices.Clone(c.UserAssignments[username])
}
