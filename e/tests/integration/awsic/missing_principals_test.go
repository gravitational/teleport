package awsic

import (
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/e/tests/common"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TestMissingAWSPrincipal tests the behavior when a principal (user or group) is
// deleted from AWS Identity Center outside of Teleport's control.
// The system should detect the missing principal and reset the external ID to
// trigger SCIM reprovisioning.
func TestMissingAWSPrincipal(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stderr, logutils.SlogTextHandlerConfig{
				EnableColors: false,
				Level:        slog.LevelDebug,
			})))

	t.Run("FullControl", func(t *testing.T) {
		// GIVEN an Identity Center service with a provisioned user
		sut, mockClient := requireTestClusterWithIdentityCenter(t,
			common.WithRole(t, "aws-access-all-areas", common.WithAccountAssignment(types.Allow, "*", "*")),
			common.WithRole(t, "aws-access-2222222222", common.WithAccountAssignment(types.Allow, "2222222222", "*")))

		t.Run("UserDeleted", func(t *testing.T) {
			testUserReprovision(t, "full-handoff-francis", mockClient, sut, mockClient.DeleteMockUser)
		})

		t.Run("UserMoved", func(t *testing.T) {
			testUserReprovision(t, "full-handoff-fred", mockClient, sut, func(externalID string) {
				changeMockUserID(mockClient, externalID, "updated_"+externalID)
			})
		})

		t.Run("GroupDeleted", func(t *testing.T) {
			testGroupReprovision(t, "full-hand-off-flyers", mockClient, sut, mockClient.DeleteMockGroup)
		})

		t.Run("GroupMoved", func(t *testing.T) {
			testGroupReprovision(t, "full-hand-off-farmers", mockClient, sut, func(externalID string) {
				changeMockGroupID(mockClient, externalID, "updated_"+externalID)
			})
		})
	})

	t.Run("HybridMode", func(t *testing.T) {
		// GIVEN a Teleport cluster with the Identity Center plugin running in
		// hybrid mode
		awsState := icsdk.NewMockedAWSState(
			icsdk.WithPermissionSets(
				&icsdk.PermissionSet{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
				&icsdk.PermissionSet{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
			))

		unifiedClient := test.NewUnifiedMockClient(awsState)
		mockSCIM := &userTrackingSCIMClient{Client: unifiedClient.ViaSCIM()}
		setupMockAWSICEnvironment(t, unifiedClient.ViaAPI(), mockSCIM)

		sut := common.InitSUT(t,
			common.WithLicense("../../../fixtures/license-eub.pem"),
			common.WithRole(t, "aws-access-all-areas", common.WithAccountAssignment(types.Allow, "*", "*")),
			common.WithRole(t, "aws-access-2222222222", common.WithAccountAssignment(types.Allow, "2222222222", "*")),
			common.WithUser(t, "admin", "editor"),
		)
		adminUser := sut.GetClusterClientForUser(t, "admin")

		mustCreateAWSICPlugin(t, adminUser.AuthClient,
			withSAMLProviderName(""),
			withDefaultAccessListOwners("admin"))

		t.Run("UserDeleted", func(t *testing.T) {
			const (
				userID            = "hybrid-helga"
				initialExternalID = "uid-" + userID
			)
			unifiedClient.AddUserToState(initialExternalID, userID)

			ctx := t.Context()
			auth := sut.Teleport.Process.GetAuthServer()

			// GIVEN an Identity Center service provisioned user
			common.MustCreateUserWithCleanup(t, sut, userID, "aws-access-2222222222")

			// Wait for the user to be provisioned via SCIM and get an external ID
			require.EventuallyWithT(t,
				func(t *assert.CollectT) {
					requireICUser(ctx, t, unifiedClient, userID,
						hasAccountAssignments(
							icUserAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
							icUserAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222")))

					requirePrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(userID),
						hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
						hasExternalID(initialExternalID),
						hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
						hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222"))
				},
				10*time.Second, 100*time.Millisecond)

			// WHEN the user is deleted from AWS Identity Center (simulating external deletion)
			unifiedClient.DeleteMockUser(initialExternalID)

			// AND we trigger an assignment calculation by updating the user roles
			require.NoError(t, common.UpdateUser(ctx, auth, userID, func(u types.User) {
				u.SetRoles([]string{"aws-access-all-areas"})
			}))

			// EXPECT that the system should detect the missing principal and
			// delete the principal's external ID and account assignments, along
			// with their SCIM provisioning External ID
			require.EventuallyWithT(t,
				func(t *assert.CollectT) {
					requirePrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(userID),
						hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE),
						hasExternalID(""),
						hasNoAccountAssignments)

					requireSCIMProvisioningState(ctx, t, auth, provisioning.GetIDForUserName(userID),
						hasSCIMExternalID(""))
				},
				15*time.Second, 200*time.Millisecond)
		})

		t.Run("UserMoved", func(t *testing.T) {
			testUserReprovision(t, "hybrid-hyacinth", unifiedClient, sut,
				func(externalID string) {
					changeMockUserID(unifiedClient, externalID, "updated_"+externalID)
				})
		})

		t.Run("GroupDeleted", func(t *testing.T) {
			testGroupReprovision(t, "hybrid-mode-harriers", unifiedClient, sut, unifiedClient.DeleteMockGroup)
		})

		t.Run("GroupMoved", func(t *testing.T) {
			testGroupReprovision(t, "hybrid-mode-harriers", unifiedClient, sut,
				func(externalID string) {
					changeMockGroupID(unifiedClient, externalID, "updated_"+externalID)
				})
		})
	})
}

func testUserReprovision(t *testing.T, userID string, client *test.UnifiedClientMock, sut *common.SUT, mutateMockState func(string)) {
	ctx := t.Context()
	auth := sut.Teleport.Process.GetAuthServer()
	initialExternalID := "uid-" + userID

	client.AddUserToState(initialExternalID, userID)
	common.MustCreateUser(t, sut, userID, "aws-access-all-areas")

	// Wait for the user to be provisioned via SCIM and get an external ID
	var provisionedExternalID string
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			requireICUser(ctx, t, client, userID,
				hasAccountAssignments(
					icUserAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
					icUserAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "1111111111"),
					icUserAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
					icUserAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222")))

			pa := requirePrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(userID),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasAnyExternalID,
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "1111111111"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222"))

			// record the matching assignment record's external ID for later
			// comparison
			provisionedExternalID = pa.GetSpec().GetExternalId()
		},
		10*time.Second, 100*time.Millisecond)
	require.Equal(t, initialExternalID, provisionedExternalID)

	// WHEN I manually assign a new ID to the downstream group to simulate
	// a group being deleted and re-created outside of Teleport's
	// control
	mutateMockState(initialExternalID)

	// AND we trigger an assignment calculation by updating the user roles
	require.NoError(t, common.UpdateUser(ctx, auth, userID, func(u types.User) {
		u.SetRoles([]string{"aws-access-2222222222"})
	}))

	// EXPECT that the system should detect the missing principal and re-acquire
	// them via SCIM (resulting in a new ExternalID)
	var actualExternalID string
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			requireICUser(ctx, t, client, userID,
				hasAccountAssignments(
					icUserAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
					icUserAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222")))

			pa := requirePrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(userID),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasAnyExternalID,
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222"))

			// record the matching assignment record's external ID for later
			// comparison
			actualExternalID = pa.GetSpec().GetExternalId()
		},
		15*time.Second, 200*time.Millisecond)

	require.NotEqual(t, initialExternalID, actualExternalID)
}

func testGroupReprovision(t *testing.T, groupDisplayName string, client *test.UnifiedClientMock, sut *common.SUT, mutateMockState func(string)) {
	ctx := t.Context()
	auth := sut.Teleport.Process.GetAuthServer()

	// GIVEN a Teleport Access List provisioned into Identity Center as a Group
	acl := common.CreateAccessList(t, sut,
		common.WithName(uuid.NewString()),
		common.WithTitle(groupDisplayName),
		common.WithOwners("admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"requester", "aws-access-2222222222"}}))
	var initialExternalID string
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			requireICGroup(ctx, t, client, groupDisplayName,
				hasGroupAccountAssignments(
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222")))

			pa := requirePrincipalAssignment(ctx, t, auth,
				principal.GetIDForAccessListName(acl.GetName()),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasAnyExternalID,
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222"))
			initialExternalID = pa.GetSpec().GetExternalId()
		},
		15*time.Second, 200*time.Millisecond, "Initial group provisioning timed out or failed")

	// WHEN the group is deleted or moved from AWS Identity Center (depending on
	// supplied mutateMockState function)
	mutateMockState(initialExternalID)

	// AND we trigger an assignment recalculation by changing the granted roles
	mustGetAccessListByTitle(ctx, t, auth, groupDisplayName)
	acl, err := auth.AccessListsInternal.GetAccessList(ctx, acl.GetName())
	require.NoError(t, err)
	acl.Spec.Grants.Roles = []string{"requester", "aws-access-all-areas"}
	_, err = auth.AccessListsInternal.UpsertAccessList(ctx, acl)
	require.NoError(t, err)

	// EXPECT that the system detects the missing principal and re-provisions
	// the group via SCIM, resulting in a new external ID
	var updatedExternalID string
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			requireICGroup(ctx, t, client, groupDisplayName,
				hasGroupAccountAssignments(
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "1111111111"),
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
					icGroupAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222")))

			pa := requirePrincipalAssignment(ctx, t, auth,
				principal.GetIDForAccessListName(acl.GetName()),
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
				hasAnyExternalID,
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "1111111111"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "1111111111"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/Admin", "2222222222"),
				hasAccountAssignment("arn:aws:sso:::permissionSet/ReadOnly", "2222222222"))
			updatedExternalID = pa.GetSpec().GetExternalId()
		},
		15*time.Second, 200*time.Millisecond)

	assert.NotEqual(t, initialExternalID, updatedExternalID,
		"Group should have a different external ID after re-provisioning")
}

func changeMockUserID(c *test.UnifiedClientMock, oldID, newID string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	// Find and remove the user from the mock
	userIndex := slices.IndexFunc(c.Users, func(u *icsdk.MockUser) bool {
		return aws.ToString(u.UserId) == oldID
	})
	if userIndex == -1 {
		return
	}
	user := c.Users[userIndex]
	user.UserId = aws.String(newID)
	delete(c.UserAssignments, oldID)
}

// changeMockGroupID simulates an AWS IC group being deleted and re-created
// with a new ID by updating the group ID in the mock's data set.
func changeMockGroupID(c *test.UnifiedClientMock, oldID, newID string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	groupIndex := slices.IndexFunc(c.Groups, func(g *icsdk.Group) bool {
		return g.ID == oldID
	})
	if groupIndex == -1 {
		return
	}
	c.Groups[groupIndex].ID = newID
	c.GroupAssignments[newID] = c.GroupAssignments[oldID]
	delete(c.GroupAssignments, oldID)
	c.GroupMemberships[newID] = c.GroupMemberships[oldID]
	delete(c.GroupMemberships, oldID)
}
