package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/samlsp"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictestenv "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	awsicui "github.com/gravitational/teleport/e/lib/web/ui/awsic"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

func TestAWSICCreatePlugin(t *testing.T) {
	t.Parallel()
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)

	_, err := authClient.CreateIntegration(wSuite.ctx, newOIDCIntegration(t))
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:         "missing name",
			form:         installRequestURLValues(t, withFieldRemoved("name")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing type",
			form:         installRequestURLValues(t, withFieldRemoved("type")),
			statusCode:   http.StatusBadRequest,
			respContains: "unknown plugin type",
		},
		{
			name:         "missing region",
			form:         installRequestURLValues(t, withFieldRemoved("region")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing arn",
			form:         installRequestURLValues(t, withFieldRemoved("arn")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing oidcIntegrationName",
			form:         installRequestURLValues(t, withFieldRemoved("oidcIntegrationName")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "non existent oidc integration names",
			form: func() url.Values {
				values := installRequestURLValues(t)
				values.Set("oidcIntegrationName", "non-existent-oidc-integration")
				return values
			}(),
			statusCode:   http.StatusNotFound,
			respContains: "doesn't exist",
		},
	}
	testCases = append(testCases, samlTestCases(t, wSuite, authClient)...)
	testCases = append(testCases, scimTestCases(t)...)
	testCases = append(testCases, testCase{
		name:         "valid",
		form:         installRequestURLValues(t),
		statusCode:   http.StatusOK,
		respContains: "",
		cleanupFunc: func() {
			require.NoError(t, wSuite.testAuthServer.Auth().DeletePlugin(t.Context(), types.PluginTypeAWSIdentityCenter))
		},
	})

	installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugins", "staticauth")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupFunc != nil {
				tc.setupFunc()
			}
			form := maps.Clone(tc.form)
			resp, err := aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.Code())

			if tc.respContains != "" {
				var respMessage errorResp
				err = json.Unmarshal(resp.Bytes(), &respMessage)
				require.NoError(t, err)
				require.Contains(t, respMessage.Error.Message, tc.respContains)
			}

			if tc.statusCode == http.StatusOK {
				newSP, err := authClient.GetSAMLIdPServiceProvider(wSuite.ctx, types.PluginTypeAWSIdentityCenter)
				require.NoError(t, err)
				require.Equal(t, common.OriginAWSIdentityCenter, newSP.Origin())
				require.Equal(t, samlsp.AWSIdentityCenter, newSP.GetPreset())
			}

			if tc.cleanupFunc != nil {
				tc.cleanupFunc()
			}
		})
	}
	testServer.Close()
}

func TestAWSICCreatePluginRollsBackSAMLServiceProviderOnCreatePluginFailure(t *testing.T) {
	t.Parallel()

	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	t.Cleanup(testServer.Close)

	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	_, err := authClient.CreateIntegration(wSuite.ctx, newOIDCIntegration(t))
	require.NoError(t, err)

	// Pre-create the static credential name used by the web install request.
	// This lets web validation pass, then forces auth CreatePlugin to fail
	// before the plugin record is persisted.
	cred, err := types.NewPluginStaticCredentials(
		types.Metadata{Name: types.PluginTypeAWSIdentityCenter},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "existing-scim-token",
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, wSuite.authPlugin.PluginStaticCredentialsService().CreatePluginStaticCredentials(wSuite.ctx, cred))
	t.Cleanup(func() {
		_ = wSuite.authPlugin.PluginStaticCredentialsService().DeletePluginStaticCredentials(wSuite.ctx, types.PluginTypeAWSIdentityCenter)
	})

	// Exercise the real web static-auth install route. The install should fail
	// on the injected static credential conflict.
	resp, err := aPack.clt.PostForm(
		wSuite.ctx,
		aPack.clt.Endpoint("enterprise", "plugins", "staticauth"),
		installRequestURLValues(t),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.Code())

	// The failed install must not leave behind the SAML provider that was
	// created before auth CreatePlugin returned the static credential conflict.
	_, err = authClient.GetSAMLIdPServiceProvider(wSuite.ctx, types.PluginTypeAWSIdentityCenter)
	require.True(t, trace.IsNotFound(err))
}

func TestAWSICCreatePluginRollsBackSAMLServiceProviderWhenPluginExists(t *testing.T) {
	t.Parallel()

	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	t.Cleanup(testServer.Close)

	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	_, err := authClient.CreateIntegration(wSuite.ctx, newOIDCIntegration(t))
	require.NoError(t, err)

	// Pre-create the plugin record without the SAML provider used by the web
	// install request. This lets web validation pass, then forces auth
	// CreatePlugin to fail while a plugin record exists.
	_, err = authClient.PluginsClient().CreatePlugin(wSuite.ctx, ictestenv.NewPluginV1CreateRequest(
		icOIDCIntegrationName,
		"preexisting-service-provider",
	))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = authClient.PluginsClient().DeletePlugin(wSuite.ctx, pluginspb.DeletePluginRequest_builder{
			Name: types.PluginTypeAWSIdentityCenter,
		}.Build())
		_ = authClient.DeleteSAMLIdPServiceProvider(wSuite.ctx, types.PluginTypeAWSIdentityCenter)
	})

	// Exercise the same web install route. The handler should return the
	// conflict and roll back the SAML provider it created for this request.
	resp, err := aPack.clt.PostForm(
		wSuite.ctx,
		aPack.clt.Endpoint("enterprise", "plugins", "staticauth"),
		installRequestURLValues(t),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.Code())

	_, err = authClient.GetSAMLIdPServiceProvider(wSuite.ctx, types.PluginTypeAWSIdentityCenter)
	require.True(t, trace.IsNotFound(err))
}

func TestAWSICPluginPreValidation(t *testing.T) {
	t.Parallel()
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)

	installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugins", "validate")

	for _, tc := range samlTestCases(t, wSuite, authClient) {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupFunc != nil {
				tc.setupFunc()
			}
			form := maps.Clone(tc.form)
			form.Set("resourceToValidate", pluginConfigAWSICValidateSAML)
			resp, err := aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.Code())

			if tc.respContains != "" {
				var respMessage errorResp
				err = json.Unmarshal(resp.Bytes(), &respMessage)
				require.NoError(t, err)
				require.Contains(t, respMessage.Error.Message, tc.respContains)
			}
			if tc.cleanupFunc != nil {
				tc.cleanupFunc()
			}
		})
	}

	for _, tc := range scimTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			form := maps.Clone(tc.form)
			form.Set("resourceToValidate", pluginConfigAWSICValidateSCIM)
			resp, err := aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.Code())

			if tc.respContains != "" {
				var respMessage errorResp
				err = json.Unmarshal(resp.Bytes(), &respMessage)
				require.NoError(t, err)
				require.Contains(t, respMessage.Error.Message, tc.respContains)
			}
		})
	}

	t.Run("resource sync permission", func(t *testing.T) {
		form := installRequestValidURLValues(t, validICSCIMBaseURLFormat)
		form.Set("resourceToValidate", pluginConfigAWSICValidateResourceSyncCredential)
		resp, err := aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, form)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		const errMsg = "invalid credential"
		sdkClient := icsdk.NewClientMock(nil /* custom mock data */)
		sdkClient.MonkeyPatch.DescribeInstance = func(context.Context) (*icsdk.InstanceInfo, error) {
			return nil, trace.AccessDenied(errMsg)
		}
		wSuite, aPack, _ := newAWSIdentityCenterPluginTestSuite(t, withICClient(sdkClient))
		resp, err = aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, form)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.Code())
	})
	testServer.Close()
}

func TestAWSICPluginValidatePermissions(t *testing.T) {
	t.Parallel()
	wSuite, aPack := newAWSIdentityCenterPluginWebTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	form := url.Values{
		awsICPluginNameField: {types.PluginTypeAWSIdentityCenter},
		"type":               {types.PluginTypeAWSIdentityCenter},
	}
	validateEndpoint := aPack.clt.Endpoint("enterprise", "plugins", "validate")
	form.Set("resourceToValidate", pluginConfigAWSICValidatePermissions)
	resp, err := aPack.clt.PostForm(wSuite.ctx, validateEndpoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	fooUserRole, err := authClient.GetRole(wSuite.ctx, "editor")
	require.NoError(t, err)
	fooUserRole.SetRules(types.Deny, []types.Rule{{Resources: []string{types.KindPlugin}, Verbs: []string{types.VerbCreate}}})
	fooUserRole.SetAppLabels(types.Allow, types.Labels{"env": utils.Strings{"not-matched"}})
	_, err = authClient.UpsertRole(wSuite.ctx, fooUserRole)
	require.NoError(t, err)

	resp, err = aPack.clt.PostForm(wSuite.ctx, validateEndpoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.Code())
	var respMessage errorResp
	require.NoError(t, json.Unmarshal(resp.Bytes(), &respMessage))
	require.Contains(t, respMessage.Error.Message, `Verb "create" on resource kind "plugin"`)
	require.Contains(t, respMessage.Error.Message, "teleport.dev/origin : aws-identity-center")
}

func TestAWSICDeletePluginResourceCleanup(t *testing.T) {
	t.Parallel()
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	ctx := wSuite.ctx
	client := ictestenv.CleanupTestClient{
		ICService:                wSuite.identitycenterService.identityCenter,
		ProvisioningStateService: wSuite.identitycenterService.provisioningState,
		SAMLIdPService:           authClient,
		IntegrationService:       authClient,
		AccessListService:        authClient.AccessListClient(),
		RoleService:              authClient,
	}
	testData := ictestenv.NewDeletionData()

	t.Run("cleanup before plugin is created", func(t *testing.T) {
		ictestenv.CreateAWSOIDCIntegration(t, ctx, authClient, icOIDCIntegrationName)
		ictestenv.CreateICResources(t, ctx, client, testData, string(identitycenter.IdentityCenterDownstreamID))
		createPluginEndpoint := aPack.clt.Endpoint("enterprise", "plugins", "staticauth")

		form := installRequestURLValues(t)
		resp, err := aPack.clt.PostForm(wSuite.ctx, createPluginEndpoint, form)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		// installing plugin does not immediately create identity center data. So it is safe
		// to assert with existing data created with createICResources function.
		ictestenv.CheckAllICResourcesAreConditionallyDeleted(t, ctx, ictestenv.CheckCleanupArgs{
			SAMLlServiceProviderName: types.PluginTypeAWSIdentityCenter,
			IntegrationName:          icOIDCIntegrationName,
			IsCreateRequest:          true,
			DownstreamID:             string(identitycenter.IdentityCenterDownstreamID),
			TestData:                 testData,
			TestClient:               client,
			ListICOriginatedAccessLists: func(ctx context.Context, service services.AccessLists) (map[string]*accesslist.AccessList, error) {
				return identitycenter.ListICOriginatedAccessLists(ctx, client.AccessListService)
			},
			ListICOriginatedRoles: func(ctx context.Context, service services.Access) ([]*types.RoleV6, error) {
				return identitycenter.ListICOriginatedRoles(ctx, client.RoleService)
			},
		})

		plugin, err := authClient.PluginsClient().GetPlugin(ctx, pluginspb.GetPluginRequest_builder{
			Name:        types.PluginTypeAWSIdentityCenter,
			WithSecrets: false,
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, plugin)
		_, err = authClient.PluginsClient().DeletePlugin(ctx, pluginspb.DeletePluginRequest_builder{Name: plugin.GetName()}.Build())
		require.NoError(t, err)
	})

	t.Run("plugin deletion prevented if user does not have access to all resources that requires deletion", func(t *testing.T) {
		ictestenv.CreateAWSOIDCIntegration(t, ctx, authClient, icOIDCIntegrationName)
		_, err := authClient.PluginsClient().CreatePlugin(ctx, ictestenv.NewPluginV1CreateRequest(icOIDCIntegrationName, types.PluginTypeAWSIdentityCenter))
		require.NoError(t, err)

		// "foo" is username of a user created with aPack. This user is
		// assigned with editor role.
		fooUserRole, err := authClient.GetRole(ctx, "editor")
		require.NoError(t, err)
		fooUserRole.SetRules(types.Deny, []types.Rule{{Resources: []string{types.KindIdentityCenter}, Verbs: []string{types.VerbDelete}}})
		_, err = authClient.UpsertRole(ctx, fooUserRole)
		require.NoError(t, err)

		deletePluginEndpoint := aPack.clt.Endpoint("enterprise", "plugin", types.PluginTypeAWSIdentityCenter)
		resp, err := aPack.clt.Delete(ctx, deletePluginEndpoint)
		require.ErrorContains(t, err, "access denied")
		require.Equal(t, http.StatusForbidden, resp.Code())

		// revert role
		fooUserRole.SetRules(types.Deny, []types.Rule{})
		_, err = authClient.UpsertRole(ctx, fooUserRole)
		require.NoError(t, err)

		_, err = authClient.PluginsClient().DeletePlugin(ctx, pluginspb.DeletePluginRequest_builder{Name: types.PluginTypeAWSIdentityCenter}.Build())
		require.NoError(t, err)
	})

	t.Run("cleanup after plugin is deleted", func(t *testing.T) {
		ictestenv.CreateAWSOIDCIntegration(t, ctx, authClient, icOIDCIntegrationName)
		installAWSICPlugin(t, ctx, aPack.clt)
		ictestenv.CreateICResources(t, ctx, client, testData, string(identitycenter.IdentityCenterDownstreamID))

		deletePluginEndpoint := aPack.clt.Endpoint("enterprise", "plugin", types.PluginTypeAWSIdentityCenter)
		resp, err := aPack.clt.Delete(ctx, deletePluginEndpoint)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		_, err = authClient.PluginsClient().GetPlugin(wSuite.ctx, pluginspb.GetPluginRequest_builder{
			Name:        types.PluginTypeAWSIdentityCenter,
			WithSecrets: false,
		}.Build())
		require.True(t, trace.IsNotFound(err))

		_, err = authClient.GetSAMLIdPServiceProvider(ctx, types.PluginTypeAWSIdentityCenter)
		require.True(t, trace.IsNotFound(err))
		_, err = authClient.GetIntegration(ctx, icOIDCIntegrationName)
		require.True(t, trace.IsNotFound(err))

		ictestenv.CheckAllICResourcesAreConditionallyDeleted(t, ctx, ictestenv.CheckCleanupArgs{
			SAMLlServiceProviderName: types.PluginTypeAWSIdentityCenter,
			IntegrationName:          icOIDCIntegrationName,
			IsCreateRequest:          false,
			DownstreamID:             string(identitycenter.IdentityCenterDownstreamID),
			TestData:                 testData,
			TestClient:               client,
			ListICOriginatedAccessLists: func(ctx context.Context, service services.AccessLists) (map[string]*accesslist.AccessList, error) {
				return identitycenter.ListICOriginatedAccessLists(ctx, client.AccessListService)
			},
			ListICOriginatedRoles: func(ctx context.Context, service services.Access) ([]*types.RoleV6, error) {
				return identitycenter.ListICOriginatedRoles(ctx, client.RoleService)
			},
		})

		needCleanupResp, err := authClient.PluginsClient().NeedsCleanup(ctx, pluginspb.NeedsCleanupRequest_builder{Type: types.PluginTypeAWSIdentityCenter}.Build())
		require.NoError(t, err)
		require.Empty(t, needCleanupResp.GetResourcesToCleanup())
	})

	t.Cleanup(func() {
		testServer.Close()
	})
}

type awsICTestSuiteConfig struct {
	descriptor awsICPluginDescriptor
}

type icSuitOpt func(cfg *awsICTestSuiteConfig)

func withICClient(c icsdk.Client) icSuitOpt {
	return func(cfg *awsICTestSuiteConfig) {
		cfg.descriptor.ICSDKClient = c
	}
}

var installAWSICClientProvidersOnce sync.Once

// installAWSICClientProvidersForTests installs package-level AWS IC SDK
// providers once so parallel web tests do not overwrite each other's globals.
func installAWSICClientProvidersForTests() {
	installAWSICClientProvidersOnce.Do(func() {
		icClientProvider := icsdk.ClientProvider
		scimClientProvider := scimsdk.ClientProvider

		icsdk.ClientProvider = func(config icsdk.Config) (icsdk.Client, error) {
			if config.InstanceARN != "" {
				return icsdk.NewClientMock(nil /* custom mock data */), nil
			}
			return icClientProvider(config)
		}
		scimsdk.ClientProvider = func(config *scimsdk.Config) (scimsdk.Client, error) {
			if config.IntegrationType == types.PluginTypeAWSIdentityCenter &&
				(config.HTTPClient == nil || config.HTTPClient.Transport == nil) {
				return scimsdk.NewSCIMClientMock(), nil
			}
			return scimClientProvider(config)
		}
	})
}

func newAWSIdentityCenterPluginWebTestSuite(t *testing.T) (*webSuite, *authWebPack) {
	t.Helper()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
				Cloud: true,
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")

	return s, webPack
}

// newAWSIdentityCenterPluginTestSuite configures the web descriptor with a
// mocked IC client and a test SCIM HTTP client.
func newAWSIdentityCenterPluginTestSuite(t *testing.T, opts ...icSuitOpt) (*webSuite, *authWebPack, *httptest.Server) {
	t.Helper()

	installAWSICClientProvidersForTests()

	s, webPack := newAWSIdentityCenterPluginWebTestSuite(t)
	testSCIMServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/random-id/scim/v2/ServiceProviderConfig":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	t.Cleanup(testSCIMServer.Close)

	// ensured SCIM base URL always points to real AWS endpoint so we need a custom transport
	// to override dialer to dail test server.
	client := testSCIMServer.Client()
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return net.Dial("tcp", testSCIMServer.Listener.Addr().String())
		},
	}

	cfg := awsICTestSuiteConfig{
		descriptor: awsICPluginDescriptor{HTTPClient: client},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.descriptor.ICSDKClient == nil {
		cfg.descriptor.ICSDKClient = icsdk.NewClientMock(nil /* custom mock data */)
	}

	s.webPlugin.pluginDescriptors[types.PluginTypeAWSIdentityCenter] = cfg.descriptor
	return s, webPack, testSCIMServer
}

type testCase struct {
	name         string
	form         url.Values
	statusCode   int
	respContains string
	setupFunc    func()
	cleanupFunc  func()
}

func samlTestCases(t *testing.T, wSuite *webSuite, authClient authclient.ClientI) []testCase {
	t.Helper()
	return []testCase{
		{
			name:         "duplicate service provider name",
			form:         installRequestURLValues(t),
			statusCode:   http.StatusConflict,
			respContains: "already exists",
			setupFunc: func() {
				ictestenv.CreateSAMLServiceProvider(t, t.Context(), authClient, types.PluginTypeAWSIdentityCenter)
			},
			cleanupFunc: func() {
				require.NoError(t, wSuite.testAuthServer.Auth().DeleteSAMLIdPServiceProvider(t.Context(), types.PluginTypeAWSIdentityCenter))
			},
		},
		{
			name:         "missing samlServiceProviderMetadata",
			form:         installRequestURLValues(t, withFieldRemoved("samlServiceProviderMetadata")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "invalid samlServiceProviderMetadata",
			form: func() url.Values {
				values := installRequestURLValues(t)
				values.Set("samlServiceProviderMetadata", "<xml></xml>")
				return values
			}(),
			statusCode:   http.StatusBadRequest,
			respContains: "invalid metadata",
		},
		{
			name: "existing entity ID for entity descriptor provider in samlServiceProviderMetadata",
			form: func() url.Values {
				values := installRequestURLValues(t)
				values.Set("samlServiceProviderMetadata", newEntityDescriptor("existing-service-provider", fmt.Sprintf("https://%s/acs", "existing-service-provider")))
				return values
			}(),
			statusCode:   http.StatusConflict,
			respContains: "has the same entity ID",
			setupFunc: func() {
				ictestenv.CreateSAMLServiceProvider(t, t.Context(), authClient, "existing-service-provider")
			},
			cleanupFunc: func() {
				require.NoError(t, wSuite.testAuthServer.Auth().DeleteSAMLIdPServiceProvider(t.Context(), "existing-service-provider"))
			},
		},
	}
}

func scimTestCases(t *testing.T) []testCase {
	t.Helper()
	return []testCase{
		{
			name:         "missing scimBaseURL",
			form:         installRequestURLValues(t, withFieldRemoved("scimBaseURL")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing scimAccessToken",
			form:         installRequestURLValues(t, withFieldRemoved("scimAccessToken")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "invalid scimBaseURL",
			form:         installRequestValidURLValues(t, "https://test.example.com/scim/v2"),
			statusCode:   http.StatusBadRequest,
			respContains: "",
		},
	}
}

type errorResp struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

const (
	icOIDCIntegrationName = "ic-oidc-integration"
)

func installRequestValidURLValues(t *testing.T, scimBaseURL string) url.Values {
	t.Helper()
	return url.Values{
		awsICPluginNameField:                        {types.PluginTypeAWSIdentityCenter},
		"type":                                      {types.PluginTypeAWSIdentityCenter},
		awsICPluginICRegionField:                    {"ca-central-1"},
		awsICPluginICARNField:                       {"arn:aws:sso:::instance/ssoins-8893885e0d4lllka"},
		awsICPluginOIDCIntegrationNameField:         {icOIDCIntegrationName},
		awsICPluginAccessListDefaultOwnersField:     {`["user1", "user2"]`},
		awsICPluginSAMLServiceProviderNameField:     {types.PluginTypeAWSIdentityCenter},
		awsICPluginSAMLServiceProviderMetadataField: {newEntityDescriptor("https://example.com", "https://example.com/acs")},
		awsICPluginSCIMBaseURLField:                 {scimBaseURL},
		awsICPluginSCIMAccessTokenField:             {"abc123example"},
	}
}

const validICSCIMBaseURLFormat = "https://scim.ca-central-1.amazonaws.com/random-id/scim/v2"

type reqOpts func(urlVals url.Values)

func withFieldRemoved(name string) reqOpts {
	return func(urlVals url.Values) {
		urlVals.Del(name)
	}
}

func installRequestURLValues(t *testing.T, opts ...reqOpts) url.Values {
	t.Helper()
	urlVals := installRequestValidURLValues(t, validICSCIMBaseURLFormat)

	for _, opt := range opts {
		opt(urlVals)
	}

	return urlVals
}

func installAWSICPlugin(t *testing.T, ctx context.Context, clt *TestWebClient) {
	t.Helper()
	installPluginEndPoint := clt.Endpoint("enterprise", "plugins", "staticauth")
	form := maps.Clone(installRequestValidURLValues(t, validICSCIMBaseURLFormat))
	resp, err := clt.PostForm(ctx, installPluginEndPoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
}

func newOIDCIntegration(t *testing.T) *types.IntegrationV1 {
	t.Helper()
	oidc, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: icOIDCIntegrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeams",
		},
	)
	require.NoError(t, err)

	return oidc
}

func TestAWSICRegionValidation(t *testing.T) {
	t.Parallel()
	hasRegionValidationError := func(t *testing.T, region string, err error) {
		t.Helper()
		require.Error(t, err)
		want := fmt.Sprintf("region %q is invalid", region)
		require.Equal(t, want, err.Error())
	}
	tests := []struct {
		name string
		path string
		req  awsicui.FetchICResourceRequest
	}{
		{
			name: "empty region",
			path: "enterprise/pluginconfig/aws-ic/preview/accounts-with-permission-sets",
			req:  awsicui.FetchICResourceRequest{},
		},
		{
			name: "invalid region with domain",
			path: "enterprise/pluginconfig/aws-ic/preview/groups-with-assignments",
			req: awsicui.FetchICResourceRequest{
				Region: "us-west-2.example.com",
			},
		},
		{
			name: "invalid aws region",
			path: "enterprise/pluginconfig/aws-ic/preview/permission-sets",
			req: awsicui.FetchICResourceRequest{
				Region: "u-a-2",
			},
		},
	}

	wSuite, aPack := newAWSIdentityCenterPluginWebTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	ictestenv.CreateSAMLServiceProvider(t, wSuite.ctx, authClient, types.PluginTypeAWSIdentityCenter)
	_, err := authClient.CreateIntegration(wSuite.ctx, newOIDCIntegration(t))
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := aPack.clt.PostJSON(wSuite.ctx, aPack.clt.Endpoint(tc.path), tc.req)
			hasRegionValidationError(t, tc.req.Region, err)
		})
	}
}

func TestInstallationFailsOnInvalidAWSCredential(t *testing.T) {
	t.Parallel()
	const errorMsg = "invalid credential"
	tests := []struct {
		name     string
		icClient icsdk.Client
		wantErr  bool
	}{
		{
			name: "invalid credential (with faulty mocked client)",
			icClient: func() icsdk.Client {
				sdkClient := icsdk.NewClientMock(nil /* custom mock data */)
				sdkClient.MonkeyPatch.DescribeInstance = func(context.Context) (*icsdk.InstanceInfo, error) {
					return nil, trace.AccessDenied(errorMsg)
				}
				return sdkClient
			}(),
			wantErr: true,
		},
		{
			name:     "valid credential (with valid mocked client)",
			icClient: icsdk.NewClientMock(nil /* custom mock data */),
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wSuite, aPack, _ := newAWSIdentityCenterPluginTestSuite(t, withICClient(tc.icClient))
			authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
			_, err := authClient.CreateIntegration(wSuite.ctx, newOIDCIntegration(t))
			require.NoError(t, err)
			req := installRequestURLValues(t)
			installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugins", "staticauth")
			resp, err := aPack.clt.PostForm(wSuite.ctx, installPluginEndPoint, req)
			require.NoError(t, err)
			if tc.wantErr {
				require.Equal(t, http.StatusForbidden, resp.Code())
				var respMessage errorResp
				require.NoError(t, json.Unmarshal(resp.Bytes(), &respMessage))
				require.Contains(t, respMessage.Error.Message, errorMsg)
			} else {
				require.Equal(t, http.StatusOK, resp.Code())
			}
		})
	}
}

func TestMissingIntegrationCreateAccess(t *testing.T) {
	t.Parallel()
	wSuite, aPack := newAWSIdentityCenterPluginWebTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	// "foo" is username of a user created with aPack. This user is
	// assigned with editor role.
	fooUserRole, err := authClient.GetRole(wSuite.ctx, "editor")
	require.NoError(t, err)
	fooUserRole.SetRules(types.Deny, []types.Rule{{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbCreate}}})
	_, err = authClient.UpsertRole(wSuite.ctx, fooUserRole)
	require.NoError(t, err)

	const errMsg = "access denied"
	jsonReq := awsicui.FetchICResourceRequest{
		IntegrationName: icOIDCIntegrationName,
		Region:          "ca-central-1",
		Arn:             "arn:aws:sso:::instance/ssoins-8824xxxxxd4dd99a",
	}
	// all test cases include valid request data.
	tests := []struct {
		name         string
		path         string
		jsonReq      awsicui.FetchICResourceRequest
		formReq      url.Values
		errAssertion require.ErrorAssertionFunc
		respContains string
	}{
		{
			name:    "fetch groups with assignment",
			path:    aPack.clt.Endpoint("enterprise/pluginconfig/aws-ic/preview/groups-with-assignments"),
			jsonReq: jsonReq,
			errAssertion: func(t require.TestingT, err error, v ...any) {
				require.ErrorContains(t, err, errMsg)
			},
		},
		{
			name:    "fetch accounts with perm sets",
			path:    aPack.clt.Endpoint("enterprise/pluginconfig/aws-ic/preview/accounts-with-permission-sets"),
			jsonReq: jsonReq,
			errAssertion: func(t require.TestingT, err error, v ...any) {
				require.ErrorContains(t, err, errMsg)
			},
		},
		{
			name:    "fetch perm sets",
			path:    aPack.clt.Endpoint("enterprise/pluginconfig/aws-ic/preview/permission-sets"),
			jsonReq: jsonReq,
			errAssertion: func(t require.TestingT, err error, v ...any) {
				require.ErrorContains(t, err, errMsg)
			},
		},
		{
			name: "validate scim config",
			path: aPack.clt.Endpoint("enterprise/plugins/validate"),
			formReq: func() url.Values {
				form := installRequestValidURLValues(t, validICSCIMBaseURLFormat)
				form.Set("resourceToValidate", pluginConfigAWSICValidateSCIM)
				return form
			}(),
			respContains: errMsg,
		},
		{
			name: "validate resource sync credential config",
			path: aPack.clt.Endpoint("enterprise/plugins/validate"),
			formReq: func() url.Values {
				form := installRequestValidURLValues(t, validICSCIMBaseURLFormat)
				form.Set("resourceToValidate", pluginConfigAWSICValidateResourceSyncCredential)
				return form
			}(),
			respContains: errMsg,
		},
		{
			name: "validate install plugin",
			path: aPack.clt.Endpoint("enterprise/plugins/staticauth"),
			formReq: func() url.Values {
				form := installRequestValidURLValues(t, validICSCIMBaseURLFormat)
				return form
			}(),
			errAssertion: func(t require.TestingT, err error, v ...any) {
				require.ErrorContains(t, err, errMsg)
			},
			respContains: `Verb "create" on resource kind "integration"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.formReq) > 0 {
				resp, err := aPack.clt.PostForm(wSuite.ctx, tc.path, tc.formReq)
				require.NoError(t, err)
				require.Equal(t, http.StatusForbidden, resp.Code())
				var respMessage errorResp
				require.NoError(t, json.Unmarshal(resp.Bytes(), &respMessage))
				require.Contains(t, respMessage.Error.Message, tc.respContains)
			} else {
				_, err = aPack.clt.PostJSON(wSuite.ctx, tc.path, tc.jsonReq)
				tc.errAssertion(t, err)
			}
		})
	}
}
