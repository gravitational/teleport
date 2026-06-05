package pluginsv1

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestIdentityCenterValidation(t *testing.T) {
	t.Parallel()
	handler := awsicPluginHandler{modules: modulestest.EnterpriseModules()}
	suite := createSuite(t)

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
		{
			name: "Role Sync Mode NONE with exclusive group filter is allowed",
			mutate: func(s *types.PluginAWSICSettings) {
				s.RolesSyncMode = types.AWSICRolesSyncModeNone
				s.GroupSyncFilters = []*types.AWSICResourceFilter{
					{Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: `*`}},
				}
			},
			expectError: require.NoError,
		},
		{
			name: "Role Sync Mode NONE with inclusive group filter is not allowed",
			mutate: func(s *types.PluginAWSICSettings) {
				s.RolesSyncMode = types.AWSICRolesSyncModeNone
			},
			expectError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			p := newIdentityCenterPluginResource()
			test.mutate(p.Spec.GetAwsIc())
			test.expectError(t, handler.validatePlugin(t.Context(), pluginValidationInput{plugin: p}, suite.svc.authServer))
		})
	}
}

func TestIdentityCenterValidation_SystemCredentialsByRuntime(t *testing.T) {
	t.Parallel()

	explicitSystemCredentials := func(s *types.PluginAWSICSettings) {
		s.Credentials = newSystemAWSICCredentials("arn:aws:iam::123456789012:role/TeleportAWSICRole")
		s.CredentialsSource = types.AWSICCredentialsSource_AWSIC_CREDENTIALS_SOURCE_SYSTEM
	}

	testCases := []struct {
		name        string
		cloud       bool
		mutate      func(*types.PluginAWSICSettings)
		expectError require.ErrorAssertionFunc
	}{
		{
			name:        "self-hosted allows implicit system credentials",
			mutate:      func(*types.PluginAWSICSettings) {},
			expectError: require.NoError,
		},
		{
			name:        "self-hosted allows explicit system credentials",
			mutate:      explicitSystemCredentials,
			expectError: require.NoError,
		},
		{
			name:        "cloud rejects implicit system credentials",
			cloud:       true,
			mutate:      func(*types.PluginAWSICSettings) {},
			expectError: require.Error,
		},
		{
			name:        "cloud rejects explicit system credentials",
			cloud:       true,
			mutate:      explicitSystemCredentials,
			expectError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			plugin := newIdentityCenterPluginResource()
			test.mutate(plugin.Spec.GetAwsIc())

			err := awsicPluginHandler{modules: awsicTestModules(test.cloud)}.validatePlugin(t.Context(), pluginValidationInput{plugin: plugin}, nil)
			test.expectError(t, err)
			if err == nil {
				return
			}

			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, awsicSystemCredentialsCloudError)
			require.ErrorContains(t, err, awsicAWSOIDCIntegrationDocsURL)
		})
	}
}

func TestIdentityCenterValidation_CloudSystemCredentialsIntegrationListRBAC(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		rules         []types.Rule
		expectDetails bool
	}{
		{
			name: "omits existing integrations without integration list access",
			rules: []types.Rule{
				{Resources: []string{types.KindPlugin}, Verbs: []string{types.VerbCreate}},
			},
		},
		{
			name: "lists existing integrations with integration list access",
			rules: []types.Rule{
				{Resources: []string{types.KindPlugin}, Verbs: []string{types.VerbCreate}},
				{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbRead, types.VerbList}},
			},
			expectDetails: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			suite := createSuiteWithModules(t, awsicTestModules(true))
			suite.setRules(test.rules)
			_, err := suite.svc.authServer.CreateIntegration(t.Context(), mustAWSOIDCIntegration(t, "zeta"))
			require.NoError(t, err)

			_, err = suite.svc.CreatePlugin(t.Context(), pluginspb.CreatePluginRequest_builder{
				Plugin: newIdentityCenterPluginResource(),
			}.Build())
			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, awsicSystemCredentialsCloudError)
			if test.expectDetails {
				require.ErrorContains(t, err, "Existing AWS OIDC integrations: zeta.")
				require.NotContains(t, err.Error(), awsicAWSOIDCIntegrationDocsURL)
			} else {
				require.NotContains(t, err.Error(), "Existing AWS OIDC")
				require.ErrorContains(t, err, awsicAWSOIDCIntegrationDocsURL)
			}
		})
	}
}

func awsicTestModules(cloud bool) modules.Modules {
	return &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: cloud,
		},
	}
}

func TestAWSICSystemCredentialsCloudErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("omits existing integration details when no AWS OIDC integrations are found", func(t *testing.T) {
		t.Parallel()

		msg := awsicSystemCredentialsCloudErrorMessage(t.Context(), awsicIntegrationListerFunc(
			func(context.Context, int, string) ([]types.Integration, string, error) {
				return []types.Integration{nonAWSOIDCIntegration("azure-oidc")}, "", nil
			},
		))
		require.Contains(t, msg, awsicSystemCredentialsCloudError)
		require.NotContains(t, msg, "Existing AWS OIDC")
		require.Contains(t, msg, awsicAWSOIDCIntegrationDocsURL)
	})

	t.Run("lists existing AWS OIDC integrations", func(t *testing.T) {
		t.Parallel()

		msg := awsicSystemCredentialsCloudErrorMessage(t.Context(), awsicIntegrationListerFunc(
			func(ctx context.Context, pageSize int, nextToken string) ([]types.Integration, string, error) {
				switch nextToken {
				case "":
					return []types.Integration{mustAWSOIDCIntegration(t, "zeta")}, "next", nil
				case "next":
					return []types.Integration{
						nonAWSOIDCIntegration("azure-oidc"),
						mustAWSOIDCIntegration(t, "alpha"),
					}, "", nil
				default:
					t.Fatalf("unexpected next token %q", nextToken)
					return nil, "", nil
				}
			},
		))
		require.Contains(t, msg, "Existing AWS OIDC integrations: zeta, alpha.")
		require.NotContains(t, msg, "azure-oidc")
		require.NotContains(t, msg, awsicAWSOIDCIntegrationDocsURL)
	})

	t.Run("falls back to base error when listing fails", func(t *testing.T) {
		t.Parallel()

		msg := awsicSystemCredentialsCloudErrorMessage(t.Context(), awsicIntegrationListerFunc(
			func(context.Context, int, string) ([]types.Integration, string, error) {
				return nil, "", errors.New("backend unavailable")
			},
		))
		require.Contains(t, msg, awsicSystemCredentialsCloudError)
		require.Contains(t, msg, awsicAWSOIDCIntegrationDocsURL)
		require.NotContains(t, msg, "backend unavailable")
		require.NotContains(t, msg, "Existing AWS OIDC")
		require.NotContains(t, msg, "No AWS OIDC")
	})
}

// TestIdentityCenterUpdatePlugin tests that changes between the old and new
// plugin resources are valid, and that the plugin status is updated as expected
func TestIdentityCenterUpdate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                 string
		makePlugin           func() *types.PluginV1
		mutateExistingPlugin func(*types.PluginV1)
		mutateNewPlugin      func(*types.PluginV1)
		expectResult         require.ErrorAssertionFunc
		expectValue          func(*testing.T, *types.PluginV1)
	}{
		{
			name:       "AWSIC rejects status change",
			makePlugin: newIdentityCenterPluginResource,
			mutateNewPlugin: func(p *types.PluginV1) {
				status := p.GetStatus().(*types.PluginStatusV1)
				status.Code = types.PluginStatusCode_OTHER_ERROR

				details := status.GetAwsIc()
				details.GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED
				details.GroupImportStatus.ErrorMessage = "some new error message to be ignored"
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				require.Equal(t, newIdentityCenterPluginResource(), value)
			},
		},
		{
			name:       "AWSIC sets REIMPORT_REQUESTED if import filters change",
			makePlugin: newIdentityCenterPluginResource,
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().GroupSyncFilters = append(p.Spec.GetAwsIc().GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				expected := newIdentityCenterPluginResource()
				settings := expected.Spec.GetAwsIc()
				settings.GroupSyncFilters = append(settings.GroupSyncFilters,
					&types.AWSICResourceFilter{
						Include: &types.AWSICResourceFilter_NameRegex{
							NameRegex: `Another Group`,
						},
					},
				)
				expected.GetStatus().GetAwsIc().GroupImportStatus.StatusCode =
					types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED

				require.Equal(t, expected, value)
			},
		},
		{
			name:       "AWSIC handles nil status in existing plugin on filter change",
			makePlugin: newIdentityCenterPluginResource,
			mutateExistingPlugin: func(p *types.PluginV1) {
				p.SetStatus(nil)
			},
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().GroupSyncFilters = append(p.Spec.GetAwsIc().GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				expected := newIdentityCenterPluginResource()
				settings := expected.Spec.GetAwsIc()
				settings.GroupSyncFilters = append(settings.GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)

				status := expected.GetStatus().(*types.PluginStatusV1)
				status.Code = types.PluginStatusCode_UNKNOWN
				status.GetAwsIc().GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED

				require.Equal(t, expected, value)
			},
		},
		{
			name:       "AWSIC handles nil status in new plugin on filter change",
			makePlugin: newIdentityCenterPluginResource,
			mutateNewPlugin: func(p *types.PluginV1) {
				p.SetStatus(nil)
				p.Spec.GetAwsIc().GroupSyncFilters = append(p.Spec.GetAwsIc().GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				expected := newIdentityCenterPluginResource()
				settings := expected.Spec.GetAwsIc()
				settings.GroupSyncFilters = append(settings.GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)

				status := expected.GetStatus().(*types.PluginStatusV1)
				status.GetAwsIc().GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED

				require.Equal(t, expected, value)
			},
		},
		{
			name:       "AWSIC handles nil group import status in existing plugin",
			makePlugin: newIdentityCenterPluginResource,
			mutateExistingPlugin: func(p *types.PluginV1) {
				p.GetStatus().GetAwsIc().GroupImportStatus = nil
			},
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().GroupSyncFilters = append(p.Spec.GetAwsIc().GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				expected := newIdentityCenterPluginResource()
				settings := expected.Spec.GetAwsIc()
				settings.GroupSyncFilters = append(settings.GroupSyncFilters,
					&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `Another Group`}},
				)

				status := expected.GetStatus().(*types.PluginStatusV1)
				status.GetAwsIc().GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED

				require.Equal(t, expected, value)
			},
		},
		{
			name:       "AWSIC handles nil group import status in new plugin",
			makePlugin: newIdentityCenterPluginResource,
			mutateNewPlugin: func(p *types.PluginV1) {
				p.GetStatus().GetAwsIc().GroupImportStatus = nil
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				require.Equal(t, newIdentityCenterPluginResource(), value)
			},
		},
		{
			name:       "AWSIC allows Role Sync Mode change from NONE to ALL",
			makePlugin: newIdentityCenterPluginResource,
			mutateExistingPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().RolesSyncMode = types.AWSICRolesSyncModeNone
			},
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().RolesSyncMode = types.AWSICRolesSyncModeAll
			},
			expectResult: require.NoError,
			expectValue: func(t *testing.T, value *types.PluginV1) {
				require.Equal(t, types.AWSICRolesSyncModeAll, value.Spec.GetAwsIc().RolesSyncMode)
			},
		},
		{
			name:       "AWSIC disallows Role Sync Mode change from ALL to NONE",
			makePlugin: newIdentityCenterPluginResource,
			mutateExistingPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().RolesSyncMode = types.AWSICRolesSyncModeAll
			},
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().RolesSyncMode = types.AWSICRolesSyncModeNone
			},
			expectResult: require.Error,
			expectValue:  func(*testing.T, *types.PluginV1) { /* we don't care about the value on error */ },
		},
		{
			name:       "AWSIC disallows Role Sync Mode change from empty to NONE",
			makePlugin: newIdentityCenterPluginResource,
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Spec.GetAwsIc().RolesSyncMode = types.AWSICRolesSyncModeNone
			},
			expectResult: require.Error,
			expectValue:  func(*testing.T, *types.PluginV1) { /* we don't care about the value on error */ },
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			existingPlugin := test.makePlugin()
			if test.mutateExistingPlugin != nil {
				test.mutateExistingPlugin(existingPlugin)
			}

			newPlugin := test.makePlugin()
			if test.mutateNewPlugin != nil {
				test.mutateNewPlugin(newPlugin)
			}

			err := awsicPluginHandler{}.updatePlugin(newPlugin, existingPlugin)
			test.expectResult(t, err)
			test.expectValue(t, newPlugin)
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

type awsicIntegrationListerFunc func(context.Context, int, string) ([]types.Integration, string, error)

func (f awsicIntegrationListerFunc) ListIntegrations(ctx context.Context, pageSize int, nextToken string) ([]types.Integration, string, error) {
	return f(ctx, pageSize, nextToken)
}

func mustAWSOIDCIntegration(t *testing.T, name string) types.Integration {
	t.Helper()

	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: name},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/TeleportAWSICRole",
		},
	)
	require.NoError(t, err)
	return ig
}

func nonAWSOIDCIntegration(name string) types.Integration {
	return &types.IntegrationV1{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindIntegration,
			SubKind: types.IntegrationSubKindAzureOIDC,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		},
	}
}

func TestFilterEquality(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		a      *types.AWSICResourceFilter
		b      *types.AWSICResourceFilter
		expect require.BoolAssertionFunc
	}{
		{
			name: "exclusion",
			a: &types.AWSICResourceFilter{
				Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{
					ExcludeNameRegex: "*",
				},
			},
			b: &types.AWSICResourceFilter{
				Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{
					ExcludeNameRegex: "*",
				},
			},
			expect: require.True,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.expect(t, icFiltersEq(test.a, test.b))
		})
	}
}
