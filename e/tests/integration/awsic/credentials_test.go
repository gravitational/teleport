package awsic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestCredentialUpdateTriggersPluginRestart(t *testing.T) {
	const updatedSCIMBearerToken = "this-is-an-updated-scim-token"

	mockIC := icsdk.NewClientMock(nil)
	mockSCIM := scimsdk.NewSCIMClientMock()

	var tokenMu sync.Mutex
	var bearerToken string

	setMockSCIMClientProvider(t,
		func(cfg *scimsdk.Config) (scimsdk.Client, error) {
			tokenMu.Lock()
			defer tokenMu.Unlock()
			bearerToken = cfg.Token
			return mockSCIM, nil
		})

	setMockICClientProvider(t,
		func(icsdk.Config) (icsdk.Client, error) {
			return mockIC, nil
		})

	// GIVEN a running cluster with an AWS IC plugin configured with a default
	// credential
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
	)
	alice := sut.GetClusterClientForUser(t, "alice")
	mustSetupAWSIdentityCenterIntegration(t, alice.AuthClient)

	// EXPECT that the plugin will attempt to use the old credential
	require.Eventually(t,
		func() bool {
			tokenMu.Lock()
			defer tokenMu.Unlock()
			return bearerToken == DefaultSCIMBearerToken
		},
		5*time.Second, 100*time.Millisecond,
		"SCIM client must be initialized with default token")

	// WHEN I update the SCIM client credential with a new value...
	plugin := mustGetPluginResource(t, alice.AuthClient, true /* withSecrets */)
	credRef := plugin.GetCredentials().GetStaticCredentialsRef()
	require.NotNil(t, credRef, "Plugin expected to have static credentials ref")

	_, err := alice.AuthClient.PluginsClient().UpdatePluginStaticCredentials(
		context.Background(),
		&pluginsv1.UpdatePluginStaticCredentialsRequest{
			Target: &pluginsv1.UpdatePluginStaticCredentialsRequest_Query{
				Query: &pluginsv1.CredentialQuery{
					Labels: credRef.Labels,
				},
			},
			Credential: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: updatedSCIMBearerToken,
				},
			},
		})
	require.NoError(t, err, "Updating SCIM token")

	// EXPECT that the plugin will restart to pick up the new credential
	require.Eventually(t,
		func() bool {
			tokenMu.Lock()
			defer tokenMu.Unlock()
			return bearerToken == updatedSCIMBearerToken
		},
		5*time.Second, 100*time.Millisecond,
		"SCIM client must be restarted to pick up updated token")
}
