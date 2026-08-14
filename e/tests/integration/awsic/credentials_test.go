package awsic

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

type unauthorizedSCIMClient struct {
	scimsdk.Client
	// The atomic bool allows tests to toggle one specific SCIM auth failure path
	// without races while the provisioning service is running in parallel with the test.
	authFailureMode atomic.Bool
}

// SetAuthFailure toggles the simulated SCIM auth failure mode.
func (c *unauthorizedSCIMClient) SetAuthFailure(enabled bool) {
	c.authFailureMode.Store(enabled)
}

// ListUsers can return an access denied error to simulate invalid SCIM credentials when auth failure is active.
func (c *unauthorizedSCIMClient) ListUsers(ctx context.Context, queryOptions ...scimsdk.QueryOption) (*scimsdk.ListUserResponse, error) {
	if c.authFailureMode.Load() {
		return nil, trace.AccessDenied("invalid AWS SCIM credential (ListUsers)")
	}
	return c.Client.ListUsers(ctx, queryOptions...)
}

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
		pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
			Query: pluginsv1.CredentialQuery_builder{
				Labels: credRef.Labels,
			}.Build(),
			Credential: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: updatedSCIMBearerToken,
				},
			},
		}.Build())
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

func TestSCIMAuthFailureMarksPluginUnauthorized(t *testing.T) {
	setAWSSyncInterval(t, 100*time.Millisecond)

	client := ictest.NewUnifiedMockClient(icsdk.NewMockedAWSState())
	scimProxy := &unauthorizedSCIMClient{Client: client.ViaSCIM()}
	setupMockAWSICEnvironment(t, client.ViaAPI(), scimProxy)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
	)
	alice := sut.GetClusterClientForUser(t, "alice")
	mustSetupAWSIdentityCenterIntegration(t, alice.AuthClient)

	// Verify that the plugin starts in a healthy running state.
	require.Eventually(t,
		func() bool {
			plugin := mustGetPluginResource(t, alice.AuthClient, false)
			return plugin.GetStatus().GetCode() == types.PluginStatusCode_RUNNING
		},
		5*time.Second, 100*time.Millisecond,
		"Plugin must reach running state before SCIM auth failure is simulated")

	// Introduce an auth failure and verify that the plugin is marked unauthorized.
	scimProxy.SetAuthFailure(true)
	require.Eventually(t,
		func() bool {
			plugin := mustGetPluginResource(t, alice.AuthClient, false)
			return plugin.GetStatus().GetCode() == types.PluginStatusCode_UNAUTHORIZED
		},
		5*time.Second, 100*time.Millisecond,
		"SCIM auth failures must mark the AWS IAM Identity Center plugin unauthorized")

	// Resolve the auth failure.
	scimProxy.SetAuthFailure(false)

	// Verify that the plugin recovers to running.
	require.Eventually(t,
		func() bool {
			plugin := mustGetPluginResource(t, alice.AuthClient, false)
			return plugin.GetStatus().GetCode() == types.PluginStatusCode_RUNNING
		},
		5*time.Second, 100*time.Millisecond,
		"Plugin must recover to running state after SCIM auth is restored")
}
