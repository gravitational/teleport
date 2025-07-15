package awsic

import (
	"testing"

	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// setupMockAWSICEnvironment sets the test aws mocks
// and reverts the change in the test cleanup function.
// It must not be used in parallel tests.
// )
func setupMockAWSICEnvironment(t *testing.T, icMock *icsdk.ClientMock, scimMock *scimsdk.ClientMock) {
	defaultSCIM := scimsdk.ClientProvider
	defaultIC := icsdk.ClientProvider

	t.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	t.Cleanup(func() {
		scimsdk.ClientProvider = defaultSCIM
		icsdk.ClientProvider = defaultIC
	})

	scimsdk.ClientProvider = func(config *scimsdk.Config) (scimsdk.Client, error) { return scimMock, nil }
	icsdk.ClientProvider = func(config icsdk.Config) (icsdk.Client, error) { return icMock, nil }
}

func mustSetupAWSIdentityCenterIntegration(t *testing.T, authClient authclient.ClientI) {
	t.Helper()
	_, err := authClient.CreateIntegration(t.Context(), awsOIDCIntegration())
	require.NoError(t, err)

	_, err = authClient.PluginsClient().CreatePlugin(t.Context(), awsICPluginRequest())
	require.NoError(t, err)
}

func awsOIDCIntegration() *types.IntegrationV1 {
	return &types.IntegrationV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{Name: "aws-oidc-integration"},
			Kind:     types.KindIntegration,
			SubKind:  types.IntegrationSubKindAWSOIDC,
		},
		Spec: types.IntegrationSpecV1{
			SubKindSpec: &types.IntegrationSpecV1_AWSOIDC{
				AWSOIDC: &types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::111111111111:role/oidc-role",
				},
			},
		},
	}
}

func awsICPluginRequest() *pluginspb.CreatePluginRequest {
	return &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: types.PluginTypeAWSIdentityCenter,
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_AwsIc{
					AwsIc: &types.PluginAWSICSettings{
						IntegrationName:         "aws-oidc-integration",
						Region:                  "eu-central-1",
						Arn:                     "arn:aws:sso:::instance/ssoins-1111111111111111",
						AccessListDefaultOwners: []string{"alice"},
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: "https://scim.us-east-1.amazonaws.com/11111111111-2222-3333-4444-555555555555/scim/v2",
						},
						SamlIdpServiceProviderName: "saml-provider",
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeAWSIdentityCenter,
					Labels: map[string]string{
						"aws-ic/scim-api-endpoint": "endpoint",
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "api-token-secret",
				},
			},
		},
	}
}

func mustUpdatePlugin(t *testing.T, authClient authclient.ClientI, updateFn func(plugin *types.PluginAWSICSettings)) {
	t.Helper()
	plugin, err := authClient.PluginsClient().GetPlugin(t.Context(), &pluginspb.GetPluginRequest{
		Name:        types.PluginTypeAWSIdentityCenter,
		WithSecrets: false,
	})
	require.NoError(t, err)

	settings := plugin.Spec.GetAwsIc()
	updateFn(settings)

	plugin.Spec.Settings = &types.PluginSpecV1_AwsIc{AwsIc: settings}
	_, err = authClient.PluginsClient().UpdatePlugin(t.Context(), &pluginspb.UpdatePluginRequest{Plugin: plugin})
	require.NoError(t, err)
}
