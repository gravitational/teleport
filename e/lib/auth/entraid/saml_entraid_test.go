package entraid

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestSAMLEntraIDGroupsProvider(t *testing.T) {
	mockGroups := []string{
		"45d8d3c5-c802-45c6-b32a-1d70b5e1e86e",
		"843318fb-79a6-4168-9e6f-aa9a07481cc4",
	}

	userGroups := `{
			"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#directoryObjects",
			"@odata.nextLink": "",
			"value": [{"id": "%s"}, {"id": "%s"}]
	}`

	defaultResponseWithGroups := fmt.Sprintf(userGroups, mockGroups[0], mockGroups[1])

	defaultSAMLAssertionInfo := func() *saml2.AssertionInfo {
		return &saml2.AssertionInfo{
			Values: saml2.Values{
				entraIDAttrGroupsOverageLink: samltypes.Attribute{},
				entraIDAttrObjectIdentifier: samltypes.Attribute{
					Values: []samltypes.AttributeValue{{Value: "test-oid"}},
				},
				entraIDAttrTenantID: samltypes.Attribute{
					Values: []samltypes.AttributeValue{{Value: "test-tenant"}},
				},
			},
		}
	}

	defaultEntraIDCredentials := &types.OAuthClientCredentials{
		ClientId:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	tests := []struct {
		name                  string
		entraIDGroupsProvider *types.EntraIDGroupsProvider
		entraIDCredentials    *types.OAuthClientCredentials
		assertionInfo         *saml2.AssertionInfo
		newGraphClient        func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error)
		serverResponse        string
		expectedGroups        []string
		assertErr             require.ErrorAssertionFunc
	}{
		{
			name:                  "provider disabled",
			entraIDGroupsProvider: &types.EntraIDGroupsProvider{Disabled: true},
			entraIDCredentials:    defaultEntraIDCredentials,
			assertionInfo:         &saml2.AssertionInfo{},
			expectedGroups:        []string{},
			assertErr:             require.NoError,
		},
		{
			name:               "groups attribute present",
			entraIDCredentials: defaultEntraIDCredentials,
			assertionInfo: &saml2.AssertionInfo{
				Values: saml2.Values{
					entraIDAttrGroups: samltypes.Attribute{},
				},
			},
			expectedGroups: []string{},
			assertErr:      require.NoError,
		},
		{
			name: "objectidentifier attribute not present",
			assertionInfo: &saml2.AssertionInfo{
				Values: saml2.Values{
					entraIDAttrGroupsOverageLink: samltypes.Attribute{},
				},
			},
			entraIDCredentials: defaultEntraIDCredentials,
			expectedGroups:     []string{},
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsNotFound(err), "expected not found, got: %v", err)
				require.ErrorContains(tt, err, "objectidentifier attribute not found")
			},
		},
		{
			name:               "objectidentifier attribute empty",
			entraIDCredentials: defaultEntraIDCredentials,
			assertionInfo: &saml2.AssertionInfo{
				Values: saml2.Values{
					entraIDAttrObjectIdentifier:  samltypes.Attribute{},
					entraIDAttrGroupsOverageLink: samltypes.Attribute{},
				},
			},
			expectedGroups: []string{},
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.True(tt, trace.IsNotFound(err), "expected not found, got: %v", err)
				require.ErrorContains(tt, err, "objectidentifier attribute is empty")
			},
		},
		{
			name:               "token resolver returns groups",
			entraIDCredentials: defaultEntraIDCredentials,
			serverResponse:     defaultResponseWithGroups,
			assertionInfo:      defaultSAMLAssertionInfo(),
			expectedGroups:     slices.Clone(mockGroups),
			assertErr:          require.NoError,
		},
		{
			name:               "graph client returns error",
			entraIDCredentials: defaultEntraIDCredentials,
			assertionInfo:      defaultSAMLAssertionInfo(),
			expectedGroups:     []string{},
			newGraphClient: func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error) {
				return nil, fmt.Errorf("graph client error")
			},
			assertErr: func(tt require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorContains(tt, err, "graph client error")
			},
		},
		{
			name:               "no groups in response",
			entraIDCredentials: defaultEntraIDCredentials,
			assertionInfo:      defaultSAMLAssertionInfo(),
			serverResponse: `{
				"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#directoryObjects",
				"@odata.nextLink": "",
				"value": []
			}`,
			expectedGroups: []string{},
			assertErr:      require.NoError,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, err := types.NewSAMLConnector(fmt.Sprintf("test-connector-%d", i), types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://localhost:65535/acs",                   // Not called.
				SSO:                      "https://localhost.com/sso",                     // Not called.
				EntityDescriptorURL:      "https://localhost/saml/v2/identity_descriptor", // Not called.
				AttributesToRoles: []types.AttributeMapping{
					{Name: "group", Value: "devs", Roles: []string{"$1"}},
				},
				EntraIdGroupsProvider: tt.entraIDGroupsProvider,
				Credentials: &types.SAMLConnectorCredentials{
					Oauth: tt.entraIDCredentials,
				},
			})
			require.NoError(t, err)

			var testClient *http.Client
			if tt.serverResponse != "" {
				var testServer *httptest.Server
				testClient, testServer = newTestHTTPClientAndServer(t, tt.serverResponse)
				t.Cleanup(testServer.Close)
			}

			if tt.newGraphClient == nil {
				tt.newGraphClient = newTestGraphClient(t, testClient)
			}

			provider, err := NewSAMLEntraIDGroupsProvider(SAMLEntraIDGroupsProviderConfig{
				Connector:      connector,
				AssertionInfo:  tt.assertionInfo,
				Logger:         slog.New(slog.DiscardHandler),
				NewGraphClient: tt.newGraphClient,
				ResolveToken:   stubResolveToken,
			})
			require.NoError(t, err)

			tt.assertErr(t, provider.MaybeFetchEntraIDGroups(t.Context()))
			attrs := tt.assertionInfo.Values[entraIDAttrGroups]
			require.ElementsMatch(t, tt.expectedGroups, sliceutils.Map(attrs.Values, func(v samltypes.AttributeValue) string {
				return v.Value
			}))
		})
	}
}

func TestSAMLEntraIDGroupsProviderConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    SAMLEntraIDGroupsProviderConfig
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "missing connector",
			config: SAMLEntraIDGroupsProviderConfig{
				AssertionInfo:  &saml2.AssertionInfo{},
				Logger:         slog.New(slog.DiscardHandler),
				NewGraphClient: stubGraphClient,
				ResolveToken:   stubResolveToken,
			},
			assertErr: require.Error,
		},
		{
			name: "missing assertionInfo",
			config: SAMLEntraIDGroupsProviderConfig{
				Connector:      &types.SAMLConnectorV2{},
				Logger:         slog.New(slog.DiscardHandler),
				NewGraphClient: stubGraphClient,
				ResolveToken:   stubResolveToken,
			},
			assertErr: require.Error,
		},
		{
			name: "missing resolveToken",
			config: SAMLEntraIDGroupsProviderConfig{
				Connector:      &types.SAMLConnectorV2{},
				AssertionInfo:  &saml2.AssertionInfo{},
				Logger:         slog.New(slog.DiscardHandler),
				NewGraphClient: stubGraphClient,
			},
			assertErr: require.Error,
		},
		{
			name: "missing logger",
			config: SAMLEntraIDGroupsProviderConfig{
				Connector:      &types.SAMLConnectorV2{},
				AssertionInfo:  &saml2.AssertionInfo{},
				NewGraphClient: stubGraphClient,
				ResolveToken:   stubResolveToken,
			},
			assertErr: require.NoError,
		},
		{
			name: "missing graphClient",
			config: SAMLEntraIDGroupsProviderConfig{
				Connector:     &types.SAMLConnectorV2{},
				AssertionInfo: &saml2.AssertionInfo{},
				Logger:        slog.New(slog.DiscardHandler),
				ResolveToken:  stubResolveToken,
			},
			assertErr: require.NoError,
		},
		{
			name: "valid config",
			config: SAMLEntraIDGroupsProviderConfig{
				Connector:      &types.SAMLConnectorV2{},
				AssertionInfo:  &saml2.AssertionInfo{},
				Logger:         slog.New(slog.DiscardHandler),
				NewGraphClient: stubGraphClient,
				ResolveToken:   stubResolveToken,
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

func newTestHTTPClientAndServer(t *testing.T, response string) (*http.Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, response)
	}))

	client := server.Client()
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return net.Dial("tcp", server.Listener.Addr().String())
		},
	}

	return client, server
}

func newTestGraphClient(t *testing.T, httpClient *http.Client) func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error) {
	t.Helper()

	return func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error) {
		// Since we're necessarily passing a 'fake' token provider to the MS API Graph client,
		// let's at least verify that implementation is passing a non-nil token provider,
		// i.e. from either the integration or credentials flow.
		require.NotNil(t, tokenProvider)
		require.NotEmpty(t, graphEndpoint)
		return msgraph.NewClient(msgraph.Config{TokenProvider: &fakeTokenProvider{}, HTTPClient: httpClient})
	}
}

func stubGraphClient(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error) {
	return nil, nil
}

func stubResolveToken(ctx context.Context, connector types.SAMLConnector, assertionInfo *saml2.AssertionInfo) (azcore.TokenCredential, error) {
	return &fakeTokenProvider{}, nil
}
