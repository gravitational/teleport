package awsic

import (
	"context"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

const (
	oidcIntegrationName = "aws-oidc-integration"
)

// setMockICClientProvider sets a custom Identity Center client provider for a
// test. The original provider is automatically restored when the test completes.
func setMockICClientProvider(t *testing.T, provider func(icsdk.Config) (icsdk.Client, error)) {
	oldProvider := icsdk.ClientProvider
	t.Cleanup(func() {
		icsdk.ClientProvider = oldProvider
	})
	t.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	icsdk.ClientProvider = provider
}

// setMockSCIMClientProvider sets a custom SCIM client provider for a test.
// The original provider is automatically restored when the test completes.
func setMockSCIMClientProvider(t *testing.T, provider func(config *scimsdk.Config) (scimsdk.Client, error)) {
	oldProvider := scimsdk.ClientProvider
	t.Cleanup(func() {
		scimsdk.ClientProvider = oldProvider
	})
	t.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	scimsdk.ClientProvider = provider
}

// setupMockAWSICEnvironment sets the test aws mocks and reverts the change in
// the test cleanup function.
// It must not be used in parallel tests.
func setupMockAWSICEnvironment(t *testing.T, icMock icsdk.Client, scimMock scimsdk.Client) {
	setMockSCIMClientProvider(t, func(config *scimsdk.Config) (scimsdk.Client, error) { return scimMock, nil })
	setMockICClientProvider(t, func(config icsdk.Config) (icsdk.Client, error) { return icMock, nil })
}

// mustSetupAWSIdentityCenterIntegration creates and installs an Identity Center
// plugin resource
func mustSetupAWSIdentityCenterIntegration(t *testing.T, authClient authclient.ClientI, options ...pluginOption) {
	t.Helper()
	mustSetupOIDCIntegration(t, authClient)
	mustCreateAWSICPlugin(t, authClient, options...)
}

type integrationCreator interface {
	CreateIntegration(context.Context, types.Integration) (types.Integration, error)
}

func mustSetupOIDCIntegration(t *testing.T, authClient integrationCreator) {
	t.Helper()
	_, err := authClient.CreateIntegration(t.Context(), awsOIDCIntegration())
	require.NoError(t, err)
}

func mustCreateAWSICPlugin(t *testing.T, authClient authclient.ClientI, options ...pluginOption) {
	t.Helper()
	request := awsICPluginRequest(options...)
	_, err := authClient.PluginsClient().CreatePlugin(t.Context(), request)
	require.NoError(t, err)
}

func awsOIDCIntegration() *types.IntegrationV1 {
	return &types.IntegrationV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{Name: oidcIntegrationName},
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

// createAWSIdentityCenterPlugin creates a new identity center plugin resource
// via the supplied auth client.
func createAWSIdentityCenterPlugin(ctx context.Context, authClient authclient.ClientI, options ...pluginOption) error {
	_, err := authClient.PluginsClient().CreatePlugin(ctx, awsICPluginRequest(options...))
	return err
}

type pluginOptions struct {
	rolesSyncMode    string
	groupSyncFilter  []*types.AWSICResourceFilter
	samlProviderName string
	defaultOwners    []string
	awsCredentials   *types.AWSICCredentials
}

// pluginOption defines a customization function for creating plugin requests
type pluginOption func(*pluginOptions)

// withRolesSyncMode sets the supplied RolesSyncMode when creating a plugin creation request
func withRolesSyncMode(m string) pluginOption {
	return func(opts *pluginOptions) {
		opts.rolesSyncMode = m
	}
}

// withRolesSyncMode adds the supplied group include regex to the AWSIC plugin
// GroupSyncFilters
func withGroupSyncFilterInclude(re string) pluginOption {
	return func(opts *pluginOptions) {
		opts.groupSyncFilter = append(opts.groupSyncFilter,
			&types.AWSICResourceFilter{
				Include: &types.AWSICResourceFilter_NameRegex{NameRegex: re},
			},
		)
	}
}

// withRolesSyncMode adds the supplied group exclude regex to the AWSIC plugin
// GroupSyncFilters
func withGroupSyncFilterExclude(re string) pluginOption {
	return func(opts *pluginOptions) {
		opts.groupSyncFilter = append(opts.groupSyncFilter,
			&types.AWSICResourceFilter{
				Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: re},
			},
		)
	}
}

// withIntegrationName sets a custom OIDC integration name
func withSAMLProviderName(n string) pluginOption {
	return func(opts *pluginOptions) {
		opts.samlProviderName = n
	}
}

// withDefaultAccessListOwners sets the default access list owners
func withDefaultAccessListOwners(owners ...string) pluginOption {
	return func(opts *pluginOptions) {
		opts.defaultOwners = owners
	}
}

// DefaultSCIMBearerToken is the value of the default SCIM bearer token created for the Identity Center integration
const DefaultSCIMBearerToken = "api-token-secret"

// awsICPluginRequest creates a potentially-customized plugin creation request
// for an AWSIC plugin
func awsICPluginRequest(options ...pluginOption) *pluginspb.CreatePluginRequest {
	opts := pluginOptions{
		rolesSyncMode:    types.AWSICRolesSyncModeAll,
		samlProviderName: "saml-provider",
		defaultOwners:    []string{"alice"},
		awsCredentials: &types.AWSICCredentials{
			Source: &types.AWSICCredentials_System{
				System: &types.AWSICCredentialSourceSystem{
					AssumeRoleArn: "arn:aws:iam::111111111111:role/teleport-ic-admin-role",
				},
			},
		},
	}
	for _, optFn := range options {
		optFn(&opts)
	}

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
						Region:                  "eu-central-1",
						Arn:                     "arn:aws:sso:::instance/ssoins-1111111111111111",
						AccessListDefaultOwners: opts.defaultOwners,
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: "https://scim.us-east-1.amazonaws.com/11111111111-2222-3333-4444-555555555555/scim/v2",
						},
						SamlIdpServiceProviderName: opts.samlProviderName,
						RolesSyncMode:              opts.rolesSyncMode,
						GroupSyncFilters:           opts.groupSyncFilter,
						Credentials:                opts.awsCredentials,
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
					APIToken: DefaultSCIMBearerToken,
				},
			},
		},
	}.Build()
}

func mustGetPluginResource(t *testing.T, authClient authclient.ClientI, withSecrets bool) *types.PluginV1 {
	t.Helper()
	plugin, err := authClient.PluginsClient().GetPlugin(context.Background(), pluginspb.GetPluginRequest_builder{
		Name:        types.PluginTypeAWSIdentityCenter,
		WithSecrets: withSecrets,
	}.Build())
	require.NoError(t, err)
	return plugin
}

func mustUpdatePlugin(t *testing.T, authClient authclient.ClientI, updateFn func(plugin *types.PluginAWSICSettings)) {
	t.Helper()
	plugin := mustGetPluginResource(t, authClient, false /* withoutSecrets */)

	settings := plugin.Spec.GetAwsIc()
	updateFn(settings)

	plugin.Spec.Settings = &types.PluginSpecV1_AwsIc{AwsIc: settings}
	_, err := authClient.PluginsClient().UpdatePlugin(t.Context(), pluginspb.UpdatePluginRequest_builder{Plugin: plugin}.Build())
	require.NoError(t, err)
}

func getAccessListByTitle(ctx context.Context, lister iciter.AccessListLister, title string) (*accesslist.AccessList, error) {
	for acl, err := range iciter.AllAccessLists(ctx, lister) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if acl.Spec.Title == title {
			return acl, nil
		}
	}
	return nil, trace.NotFound("no Access List with title %q", title)
}

func mustGetAccessListByTitle(ctx context.Context, t *testing.T, lister iciter.AccessListLister, title string) *accesslist.AccessList {
	acl, err := getAccessListByTitle(ctx, lister, title)
	require.NoError(t, err)
	return acl
}

func mustUpdateUser(ctx context.Context, t *testing.T, usersSvc services.UsersService, username string, mutateFn func(types.User)) {
	require.NoError(t, common.UpdateUser(ctx, usersSvc, username, mutateFn))
}

func isAllowedResourceNameChar(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	if r >= 'a' && r <= 'z' {
		return true
	}
	return r == '-' || r == '@' || r == ':'
}

func normalizeResourceName(name string) string {
	name = strings.ToLower(name)

	var sb strings.Builder
	var lastChar rune
	for _, r := range name {
		if isAllowedResourceNameChar(r) {
			sb.WriteRune(r)
			lastChar = r
			continue
		}
		// Replace runs of disallowed characters with '_'
		if sb.Len() > 0 && lastChar != '_' {
			sb.WriteRune('_')
			lastChar = '_'
		}
	}
	return strings.Trim(sb.String(), "_")
}
