package entraid

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestEntraIDTokenResolver(t *testing.T) {
	tests := []struct {
		name          string
		connector     types.SAMLConnector
		plugins       []types.Plugin
		integration   types.Integration
		assertionInfo *saml2.AssertionInfo
		assertErr     require.ErrorAssertionFunc
		assertToken   require.ValueAssertionFunc
	}{
		{
			name: "OAuth fallback",
			connector: newTestSAMLConnector(t, "test-connector", &types.OAuthClientCredentials{
				ClientId:     "test-client-id",
				ClientSecret: "test-client-secret",
			}),
			assertionInfo: newTestAssertionInfo(t, true),
			assertErr:     require.NoError,
			assertToken:   require.NotNil,
		},
		{
			name:          "plugin system credentials source",
			connector:     newTestSAMLConnector(t, "test-connector", nil),
			assertionInfo: newTestAssertionInfo(t, true),
			plugins:       []types.Plugin{newTestEntraPlugin(t, "test-plugin", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS)},
			assertErr:     require.NoError,
			assertToken:   require.NotNil,
		},
		{
			name:          "plugin OIDC integration source",
			connector:     newTestSAMLConnector(t, "test-connector", nil),
			assertionInfo: newTestAssertionInfo(t, true),
			plugins:       []types.Plugin{newTestEntraPlugin(t, "test-entra-id", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC)},
			integration:   newTestEntraIntegration(t, "test-entra-id"),
			assertErr:     require.NoError,
			assertToken:   require.NotNil,
		},
		{
			name:          "plugin unknown source",
			connector:     newTestSAMLConnector(t, "test-connector", nil),
			assertionInfo: newTestAssertionInfo(t, true),
			plugins:       []types.Plugin{newTestEntraPlugin(t, "test-entra-id", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_UNKNOWN)},
			integration:   newTestEntraIntegration(t, "test-entra-id"),
			assertErr:     require.NoError,
			assertToken:   require.NotNil,
		},
		{
			name:      "no plugin or OAuth fallback",
			connector: newTestSAMLConnector(t, "test-connector", nil),
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
				require.ErrorContains(tt, err, "no credentials")
			},
			assertToken: require.Nil,
		},
		{
			name: "OAuth fallback missing tenant ID",
			connector: newTestSAMLConnector(t, "test-connector", &types.OAuthClientCredentials{
				ClientId:     "test-client-id",
				ClientSecret: "test-client-secret",
			}),
			assertionInfo: newTestAssertionInfo(t, false),
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
				require.ErrorContains(tt, err, "no tenant ID")
			},
			assertToken: require.Nil,
		},
		{
			name:          "plugin OIDC missing integration",
			connector:     newTestSAMLConnector(t, "test-connector", nil),
			assertionInfo: newTestAssertionInfo(t, true),
			plugins:       []types.Plugin{newTestEntraPlugin(t, "test-entra-id", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC)},
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsNotFound(err), "expected not found, got: %v", err)
			},
			assertToken: require.Nil,
		},
		{
			name:          "multiple matching plugins",
			connector:     newTestSAMLConnector(t, "test-connector", nil),
			assertionInfo: newTestAssertionInfo(t, true),
			plugins: []types.Plugin{
				newTestEntraPlugin(t, "test-entra-id-1", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC),
				newTestEntraPlugin(t, "test-entra-id-2", "test-connector", types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC),
			},
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
				require.ErrorContains(tt, err, "multiple Entra plugins")
			},
			assertToken: require.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newTestFixture(t)
			ctx := t.Context()

			resolver, err := NewTokenResolver(TokenResolverConfig{
				Plugins:      fixture.authServer.Plugins,
				Integrations: fixture.authServer.Integrations,
				TokenGenerator: func(ctx context.Context) (string, error) {
					return "test-token", nil
				},
				NewSystemCredential: func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
					return &fakeTokenProvider{}, nil
				},
				Logger: slog.New(slog.DiscardHandler),
			})
			require.NoError(t, err)

			if tt.integration != nil {
				_, err := fixture.authServer.CreateIntegration(ctx, tt.integration)
				require.NoError(t, err)
			}

			for _, plugin := range tt.plugins {
				require.NoError(t, fixture.authServer.CreatePlugin(ctx, plugin))
			}

			token, err := resolver.ResolveToken(ctx, tt.connector, tt.assertionInfo)
			tt.assertErr(t, err)

			if tt.assertToken != nil {
				tt.assertToken(t, token)
			}
		})
	}
}

func TestEntraIDTokenResolverConfig(t *testing.T) {
	fixture := newTestFixture(t)

	tests := []struct {
		name      string
		config    TokenResolverConfig
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "missing plugins",
			config: TokenResolverConfig{
				Integrations: fixture.authServer.Integrations,
				Logger:       slog.New(slog.DiscardHandler),
			},
			assertErr: require.Error,
		},
		{
			name: "missing integrations",
			config: TokenResolverConfig{
				Plugins: fixture.authServer.Plugins,
				Logger:  slog.New(slog.DiscardHandler),
			},
			assertErr: require.Error,
		},
		{
			name: "missing logger",
			config: TokenResolverConfig{
				Plugins:      fixture.authServer.Plugins,
				Integrations: fixture.authServer.Integrations,
			},
			assertErr: require.NoError,
		},
		{
			name: "valid config",
			config: TokenResolverConfig{
				Plugins:      fixture.authServer.Plugins,
				Integrations: fixture.authServer.Integrations,
				Logger:       slog.New(slog.DiscardHandler),
			},
			assertErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, tt.config.checkAndSetDefaults())
		})
	}
}

func newTestAssertionInfo(t *testing.T, withTenant bool) *saml2.AssertionInfo {
	t.Helper()

	assertionInfo := &saml2.AssertionInfo{
		Values: saml2.Values{
			entraIDAttrGroupsOverageLink: samltypes.Attribute{
				Values: []samltypes.AttributeValue{{Value: "test-link"}},
			},
			entraIDAttrObjectIdentifier: samltypes.Attribute{
				Values: []samltypes.AttributeValue{{Value: "test-oid"}},
			},
		},
	}

	if withTenant {
		assertionInfo.Values[entraIDAttrTenantID] = samltypes.Attribute{
			Values: []samltypes.AttributeValue{{Value: "test-tenant-id"}},
		}
	}

	return assertionInfo
}

func newTestEntraIntegration(t *testing.T, name string) *types.IntegrationV1 {
	t.Helper()

	integration, err := types.NewIntegrationAzureOIDC(
		types.Metadata{Name: name},
		&types.AzureOIDCIntegrationSpecV1{
			TenantID: "test-tenant-id",
			ClientID: "test-client-id",
		},
	)
	require.NoError(t, err)

	return integration
}

func newTestEntraPlugin(t *testing.T, name, connectorName string, credentialsSource types.EntraIDCredentialsSource) *types.PluginV1 {
	t.Helper()

	return types.NewPluginV1(
		types.Metadata{Name: name},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_EntraId{
				EntraId: &types.PluginEntraIDSettings{
					SyncSettings: &types.PluginEntraIDSyncSettings{
						DefaultOwners:     []string{"admin"},
						SsoConnectorId:    connectorName,
						CredentialsSource: credentialsSource,
					},
				},
			},
		},
		nil,
	)
}

func newTestSAMLConnector(t *testing.T, name string, credentials *types.OAuthClientCredentials) types.SAMLConnector {
	t.Helper()

	spec := types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://localhost:65535/acs",                   // Not called.
		SSO:                      "https://localhost.com/sso",                     // Not called.
		EntityDescriptorURL:      "https://localhost/saml/v2/identity_descriptor", // Not called.
		AttributesToRoles: []types.AttributeMapping{
			{Name: "group", Value: "devs", Roles: []string{"$1"}},
		},
	}

	if credentials != nil {
		spec.Credentials = &types.SAMLConnectorCredentials{Oauth: credentials}
	}

	connector, err := types.NewSAMLConnector(name, spec)
	require.NoError(t, err)
	return connector
}

type testFixture struct {
	authServer *auth.Server
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()

	ctx := t.Context()
	clock := clockwork.NewFakeClockAt(time.Now())

	b, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	keygen, err := testauthority.NewKeygen(modules.BuildEnterprise, clock.Now)
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		ClusterName:            clusterName,
		Backend:                b,
		VersionStorage:         authtest.NewFakeTeleportVersion(),
		Authority:              keygen,
		SkipPeriodicOperations: true,
		HostUUID:               uuid.NewString(),
	}

	authServer, err := auth.NewServer(authConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	return &testFixture{authServer}
}
