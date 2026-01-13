package okta

import (
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
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(7),
		withAppCount(4),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	createPluginStaticCredentials(t, sut, "unlabelled", nil)
	createPluginStaticCredentials(t, sut, "okta-unrelated", map[string]string{"test-unrelated": "to-okta"})

	getAppsReqNoCreds := &oktav1.GetAppsRequest{
		OktaOrganizationUrl: fakeOkta.URL(),
		ApiCredentials:      nil,
	}
	getGroupsReqNoCreds := &oktav1.GetGroupsRequest{
		OktaOrganizationUrl: fakeOkta.URL(),
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
		OktaOrganizationUrl: fakeOkta.URL(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
	})
	require.NoError(t, err)

	// After plugin with credentials exists, both use plugin credentials to get apps and groups.

	getAppsResp, err := oktaClient.GetApps(ctx, getAppsReqNoCreds)
	require.NoError(t, err)
	require.Len(t, getAppsResp.GetApps(), len(fakeOkta.provisionedApps)+1 /* +1 for the connector SAML app */)

	getGroupsResp, err := oktaClient.GetGroups(ctx, getGroupsReqNoCreds)
	require.NoError(t, err)
	require.Len(t, getGroupsResp.GetGroups(), len(fakeOkta.provisionedGroups))
}

func createPluginStaticCredentials(t *testing.T, sut *common.SUT, name string, labels map[string]string) types.PluginStaticCredentials {
	t.Helper()
	ctx := t.Context()

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
