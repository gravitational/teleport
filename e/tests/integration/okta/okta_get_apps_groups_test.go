package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func Test_GetApps_GetGroups_withPluginCredentials(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	_ = createOktaSAMLAPP(t, ctx, oktaApiClient, "trial-1234567_teleportsamlconnectorapp_1")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClient, withAppsGroupsUsersCount(7, 4, 1))
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	createPluginStaticCredentials(t, sut, "unlabelled", nil)
	createPluginStaticCredentials(t, sut, "okta-unrelated", map[string]string{"test-unrelated": "to-okta"})

	getAppsReqNoCreds := &oktav1.GetAppsRequest{
		OktaOrganizationUrl: oktaApiClient.GetOrgUrl(),
		ApiCredentials:      nil,
	}
	getGroupsReqNoCreds := &oktav1.GetGroupsRequest{
		OktaOrganizationUrl: oktaApiClient.GetOrgUrl(),
		ApiCredentials:      nil,
	}

	// Before plugin with credentials exists, both return error if API credentials are not
	// passed with the request.

	_, err := oktaClient.GetApps(ctx, getAppsReqNoCreds)
	require.Error(t, err)

	_, err = oktaClient.GetGroups(ctx, getGroupsReqNoCreds)
	require.Error(t, err)

	// Create an integration and the plugin with credentials as a result.

	_, err = oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:  durationpb.New(1 * time.Second),
		ReuseConnector:      "okta-pre-created-test",
		OktaOrganizationUrl: oktaApiClient.GetOrgUrl(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
	})
	require.NoError(t, err)

	// After plugin with credentials exists, both use plugin credentials to get apps and groups.

	getAppsResp, err := oktaClient.GetApps(ctx, getAppsReqNoCreds)
	require.NoError(t, err)
	require.Len(t, getAppsResp.GetApps(), len(oktaInfra.Apps)+1 /* +1 for the connector SAML app */)

	getGroupsResp, err := oktaClient.GetGroups(ctx, getGroupsReqNoCreds)
	require.NoError(t, err)
	require.Len(t, getGroupsResp.GetGroups(), len(oktaInfra.Groups))
}

func createPluginStaticCredentials(t *testing.T, sut *common.SUT, name string, labels map[string]string) types.PluginStaticCredentials {
	t.Helper()
	ctx := context.Background()

	c, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:   name,
			Labels: labels,
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "test-" + uuid.NewString(),
			},
		},
	)
	require.NoError(t, err, "types.NewPluginStaticCredentials")

	err = sut.Teleport.Process.GetAuthServer().PluginStaticCredentials.CreatePluginStaticCredentials(ctx, c)
	require.NoError(t, err, "PluginStaticCredentials.CreatePluginStaticCredentials")

	return c
}
