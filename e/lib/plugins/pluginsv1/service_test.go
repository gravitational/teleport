package pluginsv1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/services"
)

func TestPluginUpdate(t *testing.T) {
	suite := createSuite(t)
	ctx := context.Background()
	plugin := &types.PluginV1{
		Metadata: types.Metadata{Name: "okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://okta.com",
					SyncSettings: &types.PluginOktaSyncSettings{
						SyncUsers:      true,
						SsoConnectorId: "conn_id",
					},
				},
			},
		},
	}

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RO()}})
	_, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsAccessDenied(err))

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RW()}})

	_, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsNotFound(err))

	_, err = suite.svc.CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "okta",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "api_token",
				},
			},
		},
	})
	require.NoError(t, err)

	p, err := suite.svc.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name:        "okta",
		WithSecrets: false,
	})
	require.NoError(t, err)

	p.Spec.GetOkta().SsoConnectorId = "new_sso_id"

	got, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: p,
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(p, got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Test that disabled plugins can't be updated.
	suite.svc.disabledPlugins = []types.PluginType{plugin.GetType()}
	got, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: got,
	})
	require.Error(t, err)
	require.Nil(t, got)
	require.True(t, trace.IsBadParameter(err))
}

func TestIdentityCenterValidation(t *testing.T) {
	testCases := []struct {
		name        string
		mutate      func(*types.PluginAWSICSettings)
		expectError require.ErrorAssertionFunc
	}{
		{
			name:        "valid",
			mutate:      func(*types.PluginAWSICSettings) {},
			expectError: require.NoError,
		},
		{
			name: "malformed URL",
			mutate: func(s *types.PluginAWSICSettings) {
				s.ProvisioningSpec.BaseUrl = "https://example.com"
			},
			expectError: require.Error,
		},
		{
			name: "bad account filter",
			mutate: func(s *types.PluginAWSICSettings) {
				s.AwsAccountsFilters = append(s.AwsAccountsFilters,
					&types.AWSICResourceFilter{
						Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^[)$`},
					})
			},
			expectError: require.Error,
		},
		{
			name: "bad group filter",
			mutate: func(s *types.PluginAWSICSettings) {
				s.GroupSyncFilters = append(s.GroupSyncFilters,
					&types.AWSICResourceFilter{
						Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^[)$`},
					})
			},
			expectError: require.Error,
		},
		{
			name: "bad aws region",
			mutate: func(s *types.PluginAWSICSettings) {
				s.Region = "potato"
			},
			expectError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			p := newIdentityCenterPluginResource()
			test.mutate(p.Spec.GetAwsIc())
			test.expectError(t, validatePlugin(p))
		})
	}
}

func newIdentityCenterPluginResource() *types.PluginV1 {
	return &types.PluginV1{
		Kind:    types.KindPlugin,
		Version: types.V1,
		Metadata: types.Metadata{
			Name: apicommon.OriginAWSIdentityCenter,
			Labels: map[string]string{
				"teleport.dev/hosted-plugin": "true",
			},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_AwsIc{
				AwsIc: &types.PluginAWSICSettings{
					IntegrationName: apicommon.OriginAWSIdentityCenter,
					Region:          "ap-south-2",
					Arn:             "some:arn",
					ProvisioningSpec: &types.AWSICProvisioningSpec{
						BaseUrl: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2",
					},
					AccessListDefaultOwners: []string{"root"},
					CredentialsSource:       types.AWSICCredentialsSource_AWSIC_CREDENTIALS_SOURCE_SYSTEM,
					UserSyncFilters: []*types.AWSICUserSyncFilter{
						{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
						{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
					},
					GroupSyncFilters: []*types.AWSICResourceFilter{
						{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^Group #\\d+$`}},
						{Include: &types.AWSICResourceFilter_Id{Id: "42"}},
					},
					AwsAccountsFilters: []*types.AWSICResourceFilter{
						{Include: &types.AWSICResourceFilter_Id{Id: "314159"}},
						{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^Account #\\d+$`}},
					},
				},
			},
		},
		Status: types.PluginStatusV1{
			Code: types.PluginStatusCode_RUNNING,
			Details: &types.PluginStatusV1_AwsIc{
				AwsIc: &types.PluginAWSICStatusV1{
					GroupImportStatus: &types.AWSICGroupImportStatus{
						StatusCode: types.AWSICGroupImportStatusCode_DONE,
					},
				},
			},
		},
	}
}
