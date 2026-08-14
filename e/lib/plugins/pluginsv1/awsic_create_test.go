package pluginsv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
)

const (
	testAWSICOIDCIntegrationName = "aws-oidc-integration"
	testAWSICSCIMEndpointLabel   = "aws-ic/scim-api-endpoint"
	testAWSICSCIMBaseURL         = "https://scim.us-east-1.amazonaws.com/11111111111-2222-3333-4444-555555555555/scim/v2"
	testAWSICSCIMToken           = "scim-token"
)

func TestService_CreatePlugin_AWSICRejectsInvalidAWSResourceSyncCredentials(t *testing.T) {
	t.Parallel()

	suite := createSuite(t)
	setAWSICValidationClients(t, suite,
		newFailingAWSICClient(trace.AccessDenied("invalid AWS credential")),
		scimsdk.NewSCIMClientMock(),
	)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	ctx := t.Context()
	req := newAWSICCreateRequest(newSystemAWSICCredentials(""))
	_, err := suite.svc.CreatePlugin(ctx, req)
	require.ErrorContains(t, err, "invalid AWS resource sync credentials")
	require.NotContains(t, err.Error(), "invalid SCIM credentials")
	requireAWSICCreateDidNotPersist(t, ctx, suite, req)
}

func TestService_CreatePlugin_AWSICRejectsInvalidSCIMCredentials(t *testing.T) {
	t.Parallel()

	suite := createSuite(t)
	setAWSICValidationClients(t, suite,
		icsdk.NewClientMock(nil),
		scimPingErrorClient{
			Client:  scimsdk.NewSCIMClientMock(),
			pingErr: trace.AccessDenied("invalid AWS SCIM credential"),
		},
	)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	ctx := t.Context()

	req := newAWSICCreateRequest(newSystemAWSICCredentials(""))
	_, err := suite.svc.CreatePlugin(ctx, req)
	require.ErrorContains(t, err, "invalid SCIM credentials")
	require.NotContains(t, err.Error(), "invalid AWS resource sync credentials")
	requireAWSICCreateDidNotPersist(t, ctx, suite, req)
}

type scimPingErrorClient struct {
	scimsdk.Client
	pingErr error
}

func (c scimPingErrorClient) Ping(ctx context.Context) error {
	return c.pingErr
}

func setAWSICValidationClients(t *testing.T, suite *suite, icClient icsdk.Client, scimClient scimsdk.Client) {
	t.Helper()

	handler, ok := suite.svc.handlers[types.PluginTypeAWSIdentityCenter].(awsicPluginHandler)
	require.True(t, ok)

	handler.newIdentityCenterClient = func(config icsdk.Config) (icsdk.Client, error) {
		return icClient, nil
	}
	handler.newSCIMClient = func(config *scimsdk.Config) (scimsdk.Client, error) {
		return scimClient, nil
	}
	suite.svc.handlers[types.PluginTypeAWSIdentityCenter] = handler
}

func newFailingAWSICClient(err error) *icsdk.ClientMock {
	client := icsdk.NewClientMock(nil)
	client.MonkeyPatch.DescribeInstance = func(context.Context) (*icsdk.InstanceInfo, error) {
		return nil, err
	}
	return client
}

func newAWSICCreateRequest(credentials *types.AWSICCredentials) *pluginspb.CreatePluginRequest {
	return pluginspb.CreatePluginRequest_builder{
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
						IntegrationName:         testAWSICOIDCIntegrationName,
						Region:                  "eu-central-1",
						Arn:                     "arn:aws:sso:::instance/ssoins-1111111111111111",
						AccessListDefaultOwners: []string{"alice"},
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: testAWSICSCIMBaseURL,
						},
						Credentials: credentials,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeAWSIdentityCenter,
					Labels: map[string]string{
						testAWSICSCIMEndpointLabel: testAWSICSCIMBaseURL,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: testAWSICSCIMToken,
				},
			},
		},
	}.Build()
}

func newSystemAWSICCredentials(assumeRoleARN string) *types.AWSICCredentials {
	return &types.AWSICCredentials{
		Source: &types.AWSICCredentials_System{
			System: &types.AWSICCredentialSourceSystem{
				AssumeRoleArn: assumeRoleARN,
			},
		},
	}
}

func requireAWSICCreateDidNotPersist(t *testing.T, ctx context.Context, suite *suite, req *pluginspb.CreatePluginRequest) {
	t.Helper()

	_, err := suite.pluginService.GetPlugin(ctx, req.GetPlugin().GetName(), false)
	assertNotFound(t, err)

	_, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentials(ctx, req.GetStaticCredentials().GetName())
	assertNotFound(t, err)
}
