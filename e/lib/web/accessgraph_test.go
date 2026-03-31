package web

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/keys"
	accessgraphui "github.com/gravitational/teleport/e/lib/web/ui/access_graph"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestGetAccessGraph(t *testing.T) {
	const teleportUsername = "foo"

	tests := []struct {
		name                             string
		features                         string
		validation                       func(*testing.T, *webSuite)
		accessGraphHTTPHandlerValidation func(*testing.T, *http.Request)
		assertQueryError                 require.ErrorAssertionFunc
		grantAccessToTag                 bool
	}{
		{
			name: "server doesn't support HTTP: user without access graph access",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			accessGraphHTTPHandlerValidation: func(t *testing.T, req *http.Request) {
				assert.Fail(t, "unexpected HTTP request to access graph handler")
			},
			assertQueryError: require.Error,
		},
		{
			name: "server doesn't support HTTP: authenticated requests with proper role",
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.GreaterOrEqual(t, 2, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 1, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			accessGraphHTTPHandlerValidation: func(t *testing.T, req *http.Request) {
				assert.Fail(t, "unexpected HTTP request to access graph handler")
			},
			grantAccessToTag: true,
		},
		{
			name:     "server supports HTTP: user without access graph access",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 0, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.Error,
			accessGraphHTTPHandlerValidation: func(t *testing.T, req *http.Request) {
				assert.Fail(t, "unexpected HTTP request to access graph handler")
			},
		},
		{
			name:     "server supports HTTP: authenticated requests with proper role",
			features: features,
			validation: func(t *testing.T, s *webSuite) {
				// it's greater or equal because proxy can make multiple requests
				// to the access graph service to get the features
				// and ensure it's supported
				require.Equal(t, 1, int(s.accessGraphGrpcFile.Load()))
				require.Equal(t, 0, int(s.accessGraphGrpcQuery.Load()))
				require.Equal(t, 2, int(s.accessGraphHTTPFile.Load()))
				require.Equal(t, 1, int(s.accessGraphHTTPQuery.Load()))
			},
			assertQueryError: require.NoError,
			grantAccessToTag: true,
			accessGraphHTTPHandlerValidation: func(t *testing.T, req *http.Request) {
				assert.Equal(t, teleportUsername, req.Header.Get(teleport.XTeleportUsernameHeader))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := newWebSuite(t, withAccessGraphFeatures(test.features), withAccessGraphValidation(test.accessGraphHTTPHandlerValidation))
			var opts []webSuiteOpts
			if test.grantAccessToTag {
				opts = append(opts, withExtraRules(types.Rule{
					Resources: []string{types.KindAccessGraph},
					Verbs:     []string{types.ActionRead},
				}))
			}
			webPack := s.newAuthWebPack(t, teleportUsername, opts...)

			endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "static", "features.json")
			_, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
			require.NoError(t, err)

			endpoint = webPack.clt.Endpoint("enterprise", "accessgraph", "query")
			q := url.Values{}
			q.Set("query", "foo")
			_, err = webPack.clt.Get(s.ctx, endpoint, q)
			test.assertQueryError(t, err)

			test.validation(t, s)
		})
	}
}

//go:embed testdata/access_graph_integrations_response.json
var expectedListIntegrationsResponse string

func TestGetAccessGraphIntegrations(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()

	disConfig1, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "discovery-config-1"}, discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-1",
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Regions: []string{"us-west-1"},
						AssumeRole: &types.AssumeRole{
							RoleARN: "arn:aws:iam::123456789012:role/role-name",
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)

	_, err = authClient.DiscoveryConfigClient().UpsertDiscoveryConfig(ctx, disConfig1)
	require.NoError(t, err)

	disConfig2, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "discovery-config-2"}, discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-1",
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{
					{
						Integration: "integration-1",
						Regions:     []string{"us-west-1"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	_, err = authClient.DiscoveryConfigClient().UpsertDiscoveryConfig(ctx, disConfig2)
	require.NoError(t, err)

	_, err = authClient.CreateIntegration(ctx, &types.IntegrationV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "integration-1",
			},
			SubKind: types.IntegrationSubKindAWSOIDC,
		},
		Spec: types.IntegrationSpecV1{
			SubKindSpec: &types.IntegrationSpecV1_AWSOIDC{
				AWSOIDC: &types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::0987654321:role/role-name",
				},
			},
		},
	})

	require.NoError(t, err)
	_, err = authClient.PluginsClient().CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Kind:    types.KindPlugin,
			SubKind: types.PluginSubkindAccessGraph,
			Metadata: types.Metadata{
				Name:   "gitlab",
				Labels: map[string]string{},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Gitlab{
					Gitlab: &types.PluginGitlabSettings{
						ApiEndpoint: "https://gitlab.com",
					},
				},
			},
			Status: types.PluginStatusV1{
				Code:         types.PluginStatusCode_RUNNING,
				ErrorMessage: "fake error",
				LastSyncTime: s.clock.Now(),
				LastRawError: "fake raw error",
				Details: &types.PluginStatusV1_Gitlab{
					Gitlab: &types.PluginGitlabStatusV1{
						ImportedGroups:   100,
						ImportedUsers:    200,
						ImportedProjects: 500,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "integration-1",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "token",
				},
			},
		},
	})

	require.NoError(t, err)

	_, err = authClient.PluginsClient().CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Kind: types.KindPlugin,
			Metadata: types.Metadata{
				Name:   "okta",
				Labels: map[string]string{},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl: "https://ultraorg.okta.com",
					},
				},
			},
			Status: types.PluginStatusV1{
				LastSyncTime: s.clock.Now(),
				Code:         types.PluginStatusCode_RUNNING,
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "integration-2",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "token",
				},
			},
		},
	})

	require.NoError(t, err)

	_, err = authClient.PluginsClient().CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Kind: types.KindPlugin,
			Metadata: types.Metadata{
				Name:   "entra-id",
				Labels: map[string]string{},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_EntraId{
					EntraId: &types.PluginEntraIDSettings{
						SyncSettings: &types.PluginEntraIDSyncSettings{
							DefaultOwners:  []string{"admin"},
							SsoConnectorId: "foo",
							TenantId:       "bar",
							EntraAppId:     "baz",
						},
					},
				},
			},
			Status: types.PluginStatusV1{
				LastSyncTime: s.clock.Now(),
				Code:         types.PluginStatusCode_RUNNING,
				ErrorMessage: "fake error",
				Details: &types.PluginStatusV1_EntraId{
					EntraId: &types.PluginEntraIDStatusV1{
						ImportedGroups: 100,
						ImportedUsers:  200,
					},
				},
			},
		},
	})

	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "integrations")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.JSONEq(t, expectedListIntegrationsResponse, string(resp.Bytes()))
}

func TestAccessGraphSettings(t *testing.T) {
	unmarshal := func(t require.TestingT, resp *roundtrip.Response) accessgraphui.AccessGraphSettings {
		var got accessgraphui.AccessGraphSettings
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		return got
	}

	tests := []struct {
		name                string
		change              bool
		initialSpec         *clusterconfigpb.AccessGraphSettingsSpec
		initialSyncComplete bool
		want                accessgraphui.AccessGraphSettings
	}{
		{
			name:   "enable secrets scan",
			change: true,
			initialSpec: &clusterconfigpb.AccessGraphSettingsSpec{
				SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
			},
			want: accessgraphui.AccessGraphSettings{
				EnableSecretsScan: true,
			},
		},
		{
			name: "disable secrets scan",
			initialSpec: &clusterconfigpb.AccessGraphSettingsSpec{
				SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
			},
			change: false,
			want: accessgraphui.AccessGraphSettings{
				EnableSecretsScan: false,
			},
		},
		{
			name:   "enable demo mode",
			change: true,
			initialSpec: &clusterconfigpb.AccessGraphSettingsSpec{
				DemoMode: clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED,
			},
			want: accessgraphui.AccessGraphSettings{
				EnableSecretsScan: true,
			},
		},
		{
			name: "disable demo mode",
			initialSpec: &clusterconfigpb.AccessGraphSettingsSpec{
				DemoMode: clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_DISABLED,
			},
			change: false,
			want: accessgraphui.AccessGraphSettings{
				EnableSecretsScan: false,
			},
		},
		{
			name:                "initial sync complete",
			initialSpec:         &clusterconfigpb.AccessGraphSettingsSpec{},
			initialSyncComplete: true,
			change:              false,
			want: accessgraphui.AccessGraphSettings{
				Status: accessgraphui.AccessGraphSettingsStatus{
					InitialSyncComplete: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newWebSuite(t)
			_, err := s.testAuthServer.Auth().UpsertAccessGraphSettings(s.ctx, &clusterconfigpb.AccessGraphSettings{
				Kind:    types.KindAccessGraphSettings,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: tt.initialSpec,
				Status: &clusterconfigpb.AccessGraphSettingsStatus{
					InitialSyncComplete: tt.initialSyncComplete,
				},
			})
			require.NoError(t, err)

			webPack := s.newAuthWebPack(t, "foo", withExtraRules(types.Rule{
				Resources: []string{types.KindAccessGraphSettings},
				Verbs:     []string{types.VerbRead, types.VerbUpdate},
			}))
			endpoint := webPack.clt.Endpoint("enterprise", "accessgraphsettings")

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
				require.NoError(t, err)

				got := unmarshal(t, resp)
				expectedValue := cmp.Equal(tt.initialSpec.SecretsScanConfig, clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED)
				require.Equal(t, expectedValue, got.EnableSecretsScan)
			}, 5*time.Second, 1*time.Second)

			resp, err := webPack.clt.PostJSON(s.ctx, endpoint, accessgraphui.AccessGraphSettings{
				EnableSecretsScan: tt.change,
			})
			require.NoError(t, err)

			got := unmarshal(t, resp)
			require.Equal(t, tt.want, got)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{})
				require.NoError(t, err)

				got = unmarshal(t, resp)
				require.Equal(t, tt.want, got)
			}, 5*time.Second, 1*time.Second)
		})
	}
}

// TestAccessGraphCertAuth verifies the mTLS certificate-based authentication
// flow for the Access Graph API (Usage=usage:access_graph_api certs).
func TestAccessGraphCertAuth(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				entitlements.App:    {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err, "GenerateKeyWithAlgorithm failed")
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err, "MarshalPublicKey failed")

	privateKeyPem, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err, "MarshalPrivateKey failed")

	const username = "certuser"
	tests := []struct {
		name                   string
		pathPrefix             string
		extraRules             []types.Rule
		wantCode               int
		certUsage              proto.UserCertsRequest_CertUsage
		validate               func(*testing.T, *http.Request)
		generateCertAssertFunc require.ErrorAssertionFunc
		errorMessage           string
	}{
		{
			name: "authorized user request is forwarded with username header",
			extraRules: []types.Rule{
				{Resources: []string{types.KindAccessGraph}, Verbs: []string{types.VerbRead}},
			},
			certUsage:              proto.UserCertsRequest_AccessGraphAPI,
			wantCode:               http.StatusOK,
			generateCertAssertFunc: require.NoError,
			validate: func(t *testing.T, r *http.Request) {
				assert.Equal(t, username, r.Header.Get(teleport.XTeleportUsernameHeader))
			},
		},
		{
			name:       "v1-prefixed path is accepted and forwarded",
			pathPrefix: "/v1",
			extraRules: []types.Rule{
				{Resources: []string{types.KindAccessGraph}, Verbs: []string{types.VerbRead}},
			},
			certUsage:              proto.UserCertsRequest_AccessGraphAPI,
			wantCode:               http.StatusOK,
			generateCertAssertFunc: require.NoError,
			validate: func(t *testing.T, r *http.Request) {
				assert.Equal(t, username, r.Header.Get(teleport.XTeleportUsernameHeader))
			},
		},
		{
			name:                   "user without KindAccessGraph/read is denied",
			certUsage:              proto.UserCertsRequest_AccessGraphAPI,
			generateCertAssertFunc: require.Error,
		},
		{
			name: "cert with incorrect usage fallback to app",
			extraRules: []types.Rule{
				{Resources: []string{types.KindAccessGraph}, Verbs: []string{types.VerbRead}},
			},
			generateCertAssertFunc: require.NoError,
			errorMessage:           "invalid session",
			wantCode:               http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clock := clockwork.NewRealClock()
			s := newWebSuite(t,
				withClock(clock),
				withAccessGraphFeatures(features),
				withModules(testModules),
				withAccessGraphValidation(tt.validate),
			)

			// Create a user with the requested RBAC rules.
			s.createUser(t, username, s.user, s.testPassword(), s.testOtpSecret(), tt.extraRules...)

			u := authtest.TestUser(username)

			userClient, err := s.testAuthServer.NewClient(u)
			require.NoError(t, err, "NewClient failed")

			signResp, err := userClient.GenerateUserCerts(t.Context(), proto.UserCertsRequest{
				TLSPublicKey: publicKeyPEM,
				Username:     username,
				Expires:      clock.Now().Add(1 * time.Hour),
				Usage:        tt.certUsage,
			})
			tt.generateCertAssertFunc(t, err)
			if err != nil {
				return
			}

			cert, err := tls.X509KeyPair(signResp.TLS, privateKeyPem)
			require.NoError(t, err)

			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						GetClientCertificate: func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
							return &cert, nil
						},
						InsecureSkipVerify: true,
					},
				},
			}

			resp, err := httpClient.Get(s.webServerURL.String() + tt.pathPrefix + "/enterprise/accessgraph/graph/test")
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tt.wantCode, resp.StatusCode)
			if tt.wantCode == http.StatusOK {
				return
			}
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(data), tt.errorMessage)
		})
	}
}

func TestAccessGraphEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		rbacVerbs   []string
		makeRequest func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error)
		verify      func(t *testing.T, resp *roundtrip.Response, err error)
	}{
		{
			name:      "GET is allowed",
			rbacVerbs: []string{types.VerbRead},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.Get(context.Background(), endpoint, url.Values{})
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			},
		},
		{
			name:      "GET is denied",
			rbacVerbs: []string{types.VerbCreate},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.Get(context.Background(), endpoint, url.Values{})
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.Error(t, err)
				require.Equal(t, http.StatusForbidden, resp.Code())
			},
		},
		{
			name:      "POST is allowed",
			rbacVerbs: []string{types.VerbCreate},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PostJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			},
		},
		{
			name:      "POST is denied",
			rbacVerbs: []string{types.VerbRead},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PostJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.Error(t, err)
				require.Equal(t, http.StatusForbidden, resp.Code())
			},
		},
		{
			name:      "DELETE is denied",
			rbacVerbs: []string{types.VerbRead},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.Delete(context.Background(), endpoint)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.Error(t, err)
				require.Equal(t, http.StatusForbidden, resp.Code())
			},
		},
		{
			name:      "DELETE is allowed",
			rbacVerbs: []string{types.VerbDelete},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.Delete(context.Background(), endpoint)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			},
		},
		{
			name:      "PUT is allowed",
			rbacVerbs: []string{types.VerbUpdate},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PutJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			},
		},
		{
			name:      "PUT is denied",
			rbacVerbs: []string{types.VerbRead},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PutJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.Error(t, err)
				require.Equal(t, http.StatusForbidden, resp.Code())
			},
		},
		{
			name:      "PATCH is allowed",
			rbacVerbs: []string{types.VerbUpdate},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PatchJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
			},
		},
		{
			name:      "PATCH is denied",
			rbacVerbs: []string{types.VerbRead},
			makeRequest: func(clt *TestWebClient, endpoint string) (*roundtrip.Response, error) {
				return clt.PatchJSON(context.Background(), endpoint, nil)
			},
			verify: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)                             // Note: I've no idea why our client responds with no error here
				require.Equal(t, http.StatusForbidden, resp.Code()) // HTTP error code is correct
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newWebSuite(t, withAccessGraphFeatures(features))
			webPack := s.newAuthWebPack(t, "foo", withExtraRules(types.Rule{
				Resources: []string{types.KindAccessGraph},
				Verbs:     tt.rbacVerbs,
			}))

			endpoint := webPack.clt.Endpoint("enterprise", "accessgraph", "graph", "test")
			resp, err := tt.makeRequest(webPack.clt, endpoint)
			tt.verify(t, resp, err)
		})
	}
}
