package okta

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/utils/set"
)

// Verifies app and group only sync pushes RBAC changes to Okta and respects the bidirectional sync
// flag.
func Test_AppAndGroup_only_sync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create Okta user.
	const userEmail = "test-okta-user-1@example.com"
	user := fakeOkta.CreateUser("test-okta-user-1")

	// Assign the user to the SAML app.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))

	// Create Okta groups.
	group1 := fakeOkta.CreateBuiltInGroup("group1")
	group2 := fakeOkta.CreateBuiltInGroup("group2")

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	userWatcher := sut.NewResourceWatcher(t, types.KindUser)
	userGroupWatcher := sut.NewResourceWatcher(t, types.KindUserGroup)
	oktaAssignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)
	ulsWatcher := sut.NewResourceWatcher(t, types.KindUserLoginState)
	accessRequestWatcher := sut.NewResourceWatcher(t, types.KindAccessRequest)

	// Create okta-reviewer roles and assign it to alice-admin user.
	oktaReviewerRole := createRole(t, sut, "okta-reviewer",
		roleAllowDesc{
			reviewRequests: &types.AccessReviewConditions{
				Roles: []string{teleport.SystemOktaAccessRoleName},
			},
		},
		roleDenyDesc{},
	)
	assignRoles(t, sut, "alice-admin", oktaReviewerRole.GetName())

	// Create the integration with App and Group only sync and bidirectional sync enabled
	integrationSettings := integrationSettings{
		enableUserSync:          true,
		enableAppGroupSync:      true,
		enableAccessListSync:    false,
		enableBidirectionalSync: true,
	}
	mustCreateIntegration(t, sut, oktaAuthClient, createIntegrationSettings{
		integrationSettings: integrationSettings,
		apiCredentials:      apiCredentials,
		reuseConnector:      "okta-pre-created-test",
	})

	updateOktaPlugin(t, sut.Teleport.Process.GetAuthServer(), func(p *types.PluginV1) {
		assignmentProcessLoopDuration := time.Millisecond * 300
		p.Spec.GetOkta().SyncSettings.TimeBetweenAssignmentProcessLoops = assignmentProcessLoopDuration.String()
	})

	t.Run("verify users synced", func(t *testing.T) {
		common.WaitForPutEvent(t, userWatcher, func(u types.User) bool {
			return u.GetName() == userEmail
		})
		common.WaitForPutEvent(t, ulsWatcher, func(s *userloginstate.UserLoginState) bool {
			return s.GetName() == userEmail && slices.Contains(s.GetRoles(), teleport.SystemOktaRequesterRoleName)
		})
	})

	t.Run("verify groups synced", func(t *testing.T) {
		groups := set.New(group1.Id, group2.Id)
		common.WaitForPutEvent(t, userGroupWatcher, func(g types.UserGroup) bool {
			return groups.Remove(g.GetName()).Len() == 0
		})
	})

	t.Run("make sure there are not assignments to any group", func(t *testing.T) {
		require.Empty(t, fakeOkta.GroupAssignments(group1.Id))
		require.Empty(t, fakeOkta.GroupAssignments(group2.Id))
	})

	var accessRequestName string

	t.Run("create access request to group1 and wait for the okta_assignment for it", func(t *testing.T) {
		accessRequest := createAccessRequest(t, sut, group1.Id, types.KindUserGroup, userEmail)
		common.WaitForPutEvent(t, accessRequestWatcher, func(a types.AccessRequest) bool {
			return a.GetName() == accessRequest.GetName() && a.GetState() == types.RequestState_PENDING
		})

		approveAccessRequest(t, sut, accessRequest.GetName(), "alice-admin")
		common.WaitForPutEvent(t, oktaAssignmentWatcher, func(a types.OktaAssignment) bool {
			return a.GetName() == accessRequest.GetName()
		})

		accessRequestName = accessRequest.GetName()
	})

	t.Run("verify assignment to the group1 is processed", func(t *testing.T) {
		common.WaitForPutEvent(t, oktaAssignmentWatcher, func(a types.OktaAssignment) bool {
			return a.GetName() == accessRequestName && fakeOkta.UserAssignedGroup(group1.Id, user.Id)
		})
		require.Empty(t, fakeOkta.GroupAssignments(group2.Id))
	})

	// Disable bidirectional sync
	integrationSettings.enableBidirectionalSync = false
	mustUpdateIntegration(t, sut, oktaAuthClient, integrationSettings)

	t.Run("lock access request to group1 and wait for the okta_assignment to be marked for cleanup", func(t *testing.T) {
		lock, err := types.NewLock(accessRequestName, types.LockSpecV2{Target: types.LockTarget{AccessRequest: accessRequestName}})
		require.NoError(t, err)
		err = authServer.Services.UpsertLock(ctx, lock)
		require.NoError(t, err)

		common.WaitForPutEvent(t, oktaAssignmentWatcher, func(a types.OktaAssignment) bool {
			return a.GetName() == accessRequestName &&
				a.GetCleanupTime().Before(time.Now()) &&
				!a.IsFinalized()
		})
	})

	t.Run("verify assignment to the group1 is still there on the Okta side", func(t *testing.T) {
		require.True(t, fakeOkta.UserAssignedGroup(group1.Id, user.Id))
		require.Empty(t, fakeOkta.GroupAssignments(group2.Id))
	})

	// Enable bidirectional sync
	integrationSettings.enableBidirectionalSync = true
	mustUpdateIntegration(t, sut, oktaAuthClient, integrationSettings)

	t.Run("okta_assignment is cleaned up", func(t *testing.T) {
		common.WaitForDeleteEvent(t, oktaAssignmentWatcher, func(r types.Resource) bool {
			return r.GetName() == accessRequestName
		})
	})

	t.Run("verify assignment to the group1 is cleaned up on the Okta side", func(t *testing.T) {
		require.Empty(t, fakeOkta.GroupAssignments(group1.Id))
		require.Empty(t, fakeOkta.GroupAssignments(group2.Id))
	})
}
