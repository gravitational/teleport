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

func TestAWSIdentityCenterIntegration(t *testing.T) {
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
		listUserResp, err := mockSCIM.ListUsers(t.Context())
		require.NoError(c, err)
		require.Len(c, listUserResp.Users, 2)

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
