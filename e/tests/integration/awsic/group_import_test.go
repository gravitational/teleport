package awsic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestAWSGroupImportCreatesAccessLists(t *testing.T) {
	ctx := t.Context()

	mockIC := icsdk.NewClientMock(nil)
	mockSCIM := scimsdk.NewSCIMClientMock()
	setupMockAWSICEnvironment(t, mockIC, mockSCIM)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
		common.WithUser(t, "bob", "requester"),
	)

	aliceClient := sut.GetClusterClientForUser(t, "alice")
	auth := sut.Teleport.Process.GetAuthServer()

	mustSetupAWSIdentityCenterIntegration(t, aliceClient.AuthClient)

	// On the first run through, the underlying SCIM provisioner has to adopt
	// the downstream groups before the IC permissions calculator and provisioner
	// even get started on them. This can take a while in resource-constrained
	// environments (like the flaky test detector), so we need to give
	// this assertion a bit more time than the regular `require.Eventually()` 3
	// seconds

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			accounts, _, err := auth.ListIdentityCenterAccounts(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.Accounts, len(accounts))

			permissionSet, _, err := auth.ListPermissionSets(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.PermissionSets, len(permissionSet))

			accList, _, err := auth.ListAccessLists(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.Groups, len(accList))
		},
		10*time.Second, 100*time.Millisecond)

	mustUpdatePlugin(t, aliceClient.AuthClient, func(plugin *types.PluginAWSICSettings) {
		plugin.AwsAccountsFilters = []*types.AWSICResourceFilter{{Include: &types.AWSICResourceFilter_Id{Id: "1111111111"}}}
		plugin.GroupSyncFilters = []*types.AWSICResourceFilter{{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "Group1"}}}
	})

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		accounts, _, err := auth.ListIdentityCenterAccounts(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, accounts, 1)

		accList, _, err := auth.ListAccessLists(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, accList, 1)
		require.Equal(t, "Group1", accList[0].Spec.Title)
	}, time.Second*3, time.Millisecond*30)
}

// TestAWSGroupImportCreatesNoAccessListsWhenRoleSyncModeIsNONE asserts that
//   - the GroupSyncFilters must be a simple "exclude: *" when RoleSyncMode is NONE, and
//   - no Access Lists are created when AWS group import is run when RoleSyncMode
//     is NONE and GroupSyncFilters is "exclude: *"
func TestAWSGroupImportCreatesNoAccessListsWhenRoleSyncModeIsNONE(t *testing.T) {
	mockIC := icsdk.NewClientMock(nil)
	mockSCIM := scimsdk.NewSCIMClientMock()
	setupMockAWSICEnvironment(t, mockIC, mockSCIM)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
	)

	admin := sut.GetClusterClientForUser(t, "alice")
	auth := sut.Teleport.Process.GetAuthServer()

	mustSetupOIDCIntegration(t, admin.AuthClient)

	// WHEN I try create an AWS IC plugin with RolesSyncMode == NONE and an empty (i.e. include ALL)
	// group sync mode, expect it to fail because the GroupSyncFilter is invalid
	err := createAWSIdentityCenterPlugin(t.Context(), admin.AuthClient,
		withRolesSyncMode(types.AWSICRolesSyncModeNone))
	require.Error(t, err)

	// WHEN I try create an AWS IC plugin with RolesSyncMode == NONE and any sort
	// of inclusive GroupSyncFilter  EXPECT it to fail because the GroupSyncFilter
	// is invalid
	err = createAWSIdentityCenterPlugin(t.Context(), admin.AuthClient,
		withRolesSyncMode(types.AWSICRolesSyncModeNone),
		withGroupSyncFilterInclude("banana"))
	require.Error(t, err)

	// WHEN I try create an AWS IC plugin with RolesSyncMode == NONE an inclusive
	// GroupSyncFilter AND an exclude-all filter, EXPECT it to still fail because
	// a simple exclude-all is the only valid filter when RolesSyncMode == NONE
	err = createAWSIdentityCenterPlugin(t.Context(), admin.AuthClient,
		withRolesSyncMode(types.AWSICRolesSyncModeNone),
		withGroupSyncFilterInclude("banana"),
		withGroupSyncFilterExclude("*"))
	require.Error(t, err)

	// WHEN I try create an AWS IC plugin with RolesSyncMode == NONE and a single
	// exclude-all GroupSyncFilter, expect it to work because the request has the only
	// valid GroupSyncFilter when  RolesSyncMode == NONE
	err = createAWSIdentityCenterPlugin(t.Context(), admin.AuthClient,
		withRolesSyncMode(types.AWSICRolesSyncModeNone),
		withGroupSyncFilterExclude("*"))
	require.NoError(t, err)

	// ALSO EXPECT that that when Integration center integration comes online and
	// runs the AWS group import, the import status is still set to DONE and no
	// Access Lists are created
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assertImportStatus(ctx, t, auth, groupImportStatusCodeIs(types.AWSICGroupImportStatusCode_DONE))
		assertSCIMUsers(ctx, t, mockSCIM, "alice")
	}, time.Second*3, time.Millisecond*30)
	accList, _, err := auth.ListAccessLists(t.Context(), 0, "" /* first page of results */)
	require.NoError(t, err)
	require.Empty(t, accList)
}

func copyToHeap[T any](v T) *T {
	return &v
}

// TestImportedGroupsAreNotDeletedOnFilterChange asserts that groups that were
// originally imported from AWS IC are not deleted during an import operation if
// the group inclusion filters are changed such the group is now excluded.
//
// The corresponding Teleport access lists are expected to be deleted.
// If an imported Access List is explicitly deleted by a user, the corresponding
// AWS group is expected to be deleted.
func TestImportedGroupsAreNotDeletedOnFilterChange(t *testing.T) {
	ctx := t.Context()

	expectedUsers := []*icsdk.User{
		{ID: "uid_alice", UserName: "alice"},
		{ID: "uid_bob", UserName: "bob"},
		{ID: "uid_charlotte", UserName: "charlotte"},
		{ID: "uid_dave", UserName: "dave"},
	}

	expectedGroups := []icsdk.Group{
		{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
		{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
		{DisplayName: "Group3", ID: "group3", IdentityStoreID: "store1"},
	}

	// GIVEN a mock AWS config with several groups..
	awsState := icsdk.NewMockedAWSState(
		icsdk.WithUsers(expectedUsers...),
		icsdk.WithGroups(sliceutils.Map(expectedGroups, copyToHeap)...),
		icsdk.WithGroupMembers("group1", "uid_alice", "uid_charlotte"),
		icsdk.WithGroupMembers("group2", "uid_alice", "uid_bob", "uid_charlotte", "uid_dave"),
		icsdk.WithGroupMembers("group3", "uid_dave"),
	)

	// GIVEN a Teleport cluster...
	client := ictest.NewUnifiedMockClient(awsState)
	mockIC := client.ViaAPI()
	mockSCIM := client.ViaSCIM()
	setupMockAWSICEnvironment(t, mockIC, mockSCIM)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
		common.WithUser(t, "bob", "requester"),
		common.WithUser(t, "charlotte", "requester"),
		common.WithUser(t, "dave", "requester"),
	)

	aliceClient := sut.GetClusterClientForUser(t, "alice")
	auth := sut.Teleport.Process.GetAuthServer()

	// WHEN I create a new Identity Center plugin resource
	mustSetupAWSIdentityCenterIntegration(t, aliceClient.AuthClient)

	// EXPECT the plugin to start up, and that eventually
	//  - all of the IC users are adopted by Teleport, and
	//  - all of the IC groups are imported into Teleport
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that Teleport has adopted the Identity Center users and correctly
			// bound them to their corresponding Teleport users
			for _, icUser := range expectedUsers {
				requirePrincipalAssignment(ctx, t, auth, principal.GetIDForUserName(icUser.UserName),
					hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
					hasExternalID(icUser.ID),
				)
			}

			// EXPECT that the Teleport Access Lists have created and correctly bound
			// to their corresponding Identity Center groups
			for _, awsGroup := range expectedGroups {
				acl, err := getAccessListByTitle(ctx, auth, awsGroup.DisplayName)
				require.NoError(t, err)

				requirePrincipalAssignment(ctx, t, auth, principal.GetIDForAccessList(acl),
					hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED),
					hasExternalID(awsGroup.ID),
				)
			}
		},
		// Initial Identity Center startup takes a while to run, especially under
		// the flakey test detector, so we give this more than the usual 3s to run
		10*time.Second, 100*time.Millisecond,
		"Initial conditions setup failed or timed out")

	// WHEN I update the group filter to exclude "Group2"
	mustUpdatePlugin(t, aliceClient.AuthClient, func(plugin *types.PluginAWSICSettings) {
		plugin.GroupSyncFilters = []*types.AWSICResourceFilter{
			{Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: "Group2"}},
		}
	})

	// EXPECT that the Teleport Access List representing "Group2" is deleted
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assertAccessLists(ctx, t, auth, "Group1", "Group3")
		},
		time.Second*10, time.Millisecond*30,
		"Expected Access List excluded by filter to be deleted")

	// EXPECT that the original AWS group was *NOT* deleted, and its member list
	// is preserved.
	requireSCIMGroup(ctx, t, mockSCIM, "Group2",
		hasMembers("uid_alice", "uid_bob", "uid_charlotte", "uid_dave"))

	// WHEN I explicitly delete the imported group "Group3"
	acl := mustGetAccessListByTitle(ctx, t, auth, "Group3")
	require.NoError(t, auth.DeleteAccessList(ctx, acl.GetName()))

	// EXPECT that the Teleport Access List representing "Group2" is deleted
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assertAccessLists(ctx, t, auth, "Group1")
	}, time.Second*3, time.Millisecond*30)

	// EXPECT that the AWS group "Group3" is also deleted from AWS
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assertSCIMGroupsByDisplayName(ctx, t, mockSCIM, "Group1", "Group2")
	}, time.Second*3, time.Millisecond*30)
}
