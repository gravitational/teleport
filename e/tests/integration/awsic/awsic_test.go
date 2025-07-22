package awsic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
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
