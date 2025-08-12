package awsic

import (
	"testing"
	"time"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
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
	"github.com/gravitational/teleport/lib/utils/pagination"
)

func TestAWSGroupImportCreatesAccessLists(t *testing.T) {
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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertSCIMUsers(t.Context(), c, mockSCIM, "alice", "bob")

		accounts, _, err := auth.ListIdentityCenterAccounts(t.Context(), 0, &pagination.PageRequestToken{})
		require.NoError(c, err)
		require.Len(c, mockIC.Accounts, len(accounts))

		permissionSet, _, err := auth.ListPermissionSets(t.Context(), 0, &pagination.PageRequestToken{})
		require.NoError(c, err)
		require.Len(c, mockIC.PermissionSets, len(permissionSet))

		accList, _, err := auth.ListAccessLists(t.Context(), 0, "")
		require.NoError(c, err)
		require.Len(c, mockIC.Groups, len(accList))
	}, time.Second*3, time.Millisecond*30)

	mustUpdatePlugin(t, aliceClient.AuthClient, func(plugin *types.PluginAWSICSettings) {
		plugin.AwsAccountsFilters = []*types.AWSICResourceFilter{{Include: &types.AWSICResourceFilter_Id{Id: "1111111111"}}}
		plugin.GroupSyncFilters = []*types.AWSICResourceFilter{{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "Group1"}}}
	})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		accounts, _, err := auth.ListIdentityCenterAccounts(t.Context(), 0, &pagination.PageRequestToken{})
		require.NoError(c, err)
		require.Len(c, accounts, 1)

		accList, _, err := auth.ListAccessLists(t.Context(), 0, "")
		require.NoError(c, err)
		require.Len(c, accList, 1)
		require.Equal(c, "Group1", accList[0].Spec.Title)
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
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ctx := t.Context()
		assertImportStatus(ctx, c, auth, groupImportStatusCodeIs(types.AWSICGroupImportStatusCode_DONE))
		assertSCIMUsers(ctx, c, mockSCIM, "alice")
	}, time.Second*3, time.Millisecond*30)
	accList, _, err := auth.ListAccessLists(t.Context(), 0, "" /* first page of results */)
	require.NoError(t, err)
	require.Empty(t, accList)
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

	// GIVEN a mock AWS config with several groups..
	awsState := icsdk.MockedAWSStateType{
		Info: icsdk.InstanceInfo{
			OwnerAccountID:  "2222222222",
			Name:            "Mock Identity Center Instance",
			IdentityStoreID: "store1",
			Status:          ssoadmintypes.InstanceStatusActive,
		},
		Accounts: []*icsdk.Account{
			{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"},
		},
		Users: []*icsdk.User{
			{ID: "uid_alice", UserName: "alice"},
			{ID: "uid_bob", UserName: "bob"},
			{ID: "uid_charlotte", UserName: "charlotte"},
			{ID: "uid_dave", UserName: "dave"},
		},
		Groups: []*icsdk.Group{
			{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
			{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
			{DisplayName: "Group3", ID: "group3", IdentityStoreID: "store1"},
		},
		GroupMemberships: map[string][]*icsdk.GroupMember{
			"group1": {
				{MemberID: "uid_alice"},
				{MemberID: "uid_charlotte"},
			},
			"group2": {
				{MemberID: "uid_alice"},
				{MemberID: "uid_bob"},
				{MemberID: "uid_charlotte"},
				{MemberID: "uid_dave"},
			},
			"group3": {
				{MemberID: "uid_dave"},
			},
		},
	}

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

	// EXPECT the plugin to start up, and eventually all of the groups are
	// imported into Teleport with their corresponding Access List Provisioning
	// records marked as "PROVISIONED"
	expectedACLs := []string{"Group1", "Group2", "Group3"}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertAccessLists(ctx, c, auth, expectedACLs...)
		for _, aclTitle := range expectedACLs {
			acl, err := getAccessListByTitle(ctx, auth, aclTitle)
			if !assert.NoError(c, err) {
				return
			}
			principalID, err := principal.GetIDForPrincipalResource(acl)
			if !assert.NoError(t, err) {
				return
			}

			assertPrincipalAssignment(ctx, c, auth, principalID,
				hasProvisioningState(identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED))
		}

	}, time.Second*3, time.Millisecond*30)

	// WHEN I update the group filter to exclude "Group2"
	mustUpdatePlugin(t, aliceClient.AuthClient, func(plugin *types.PluginAWSICSettings) {
		plugin.GroupSyncFilters = []*types.AWSICResourceFilter{{Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: "Group2"}}}
	})

	// EXPECT that the Teleport Access List representing "Group2" is deleted
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertAccessLists(ctx, c, auth, "Group1", "Group3")
	}, time.Second*3, time.Millisecond*30)

	// EXPECT that the original AWS group was *NOT* deleted, and its member list
	// is preserved.
	requireSCIMGroup(ctx, t, mockSCIM, "Group2", "uid_alice", "uid_bob", "uid_charlotte", "uid_dave")

	// WHEN I explicitly delete the imported group "Group3"
	acl := mustGetAccessListByTitle(ctx, t, auth, "Group3")
	require.NoError(t, auth.DeleteAccessList(ctx, acl.GetName()))

	// EXPECT that the Teleport Access List representing "Group2" is deleted
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertAccessLists(ctx, c, auth, "Group1")
	}, time.Second*3, time.Millisecond*30)

	// EXPECT that the AWS group "Group3" is also deleted from AWS
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertSCIMGroupsByDisplayName(ctx, c, mockSCIM, "Group1", "Group2")
	}, time.Second*3, time.Millisecond*30,
	)
}
