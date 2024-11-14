package web

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/samlsp"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	ictestenv "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestAWSICCreatePlugin(t *testing.T) {
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)

	ictestenv.CreateSAMLServiceProvider(t, wSuite.ctx, authClient, existingServcieProviderName)

	awsIg, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: icOIDCIntegrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/DevTeams",
			IssuerS3URI: "s3://my-bucket/my-prefix",
		},
	)
	require.NoError(t, err)
	_, err = authClient.CreateIntegration(wSuite.ctx, awsIg)
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:         "missing name",
			form:         installRequestURLValues(t, testServer.URL, "name" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing type",
			form:         installRequestURLValues(t, testServer.URL, "type" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "unknown plugin type",
		},
		{
			name:         "missing region",
			form:         installRequestURLValues(t, testServer.URL, "region" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing arn",
			form:         installRequestURLValues(t, testServer.URL, "arn" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "missing oidcIntegrationName",
			form:         installRequestURLValues(t, testServer.URL, "oidcIntegrationName" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "non existent oidc integration names",
			form: func() url.Values {
				values := installRequestURLValues(t, testServer.URL, "" /* key to remove */)
				values.Set("oidcIntegrationName", "non-existent-oidc-integration")
				return values
			}(),
			statusCode:   http.StatusNotFound,
			respContains: "doesn't exist",
		},
	}
	testCases = append(testCases, samlTestCases(t, testServer.URL)...)
	testCases = append(testCases, scimTestCases(t, testServer.URL)...)
	testCases = append(testCases, testCase{
		name:         "valid",
		form:         installRequestURLValues(t, testServer.URL, "" /* key to remove */),
		statusCode:   http.StatusOK,
		respContains: "",
	})

	installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugin")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			form := maps.Clone(tc.form)
			form.Set("csrf_token", aPack.csrfToken)
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
				newSP, err := authClient.GetSAMLIdPServiceProvider(wSuite.ctx, newServcieProviderName)
				require.NoError(t, err)
				require.Equal(t, common.OriginAWSIdentityCenter, newSP.Origin())
				require.Equal(t, samlsp.AWSIdentityCenter, newSP.GetPreset())
			}
		})
	}
	testServer.Close()
}

func TestAWSICPluginPreValidation(t *testing.T) {
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)
	ictestenv.CreateSAMLServiceProvider(t, wSuite.ctx, authClient, existingServcieProviderName)

	installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugins", "validate")

	for _, tc := range samlTestCases(t, testServer.URL) {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}

	for _, tc := range scimTestCases(t, testServer.URL) {
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
	testServer.Close()
}

func TestAWSICDeletePluginResourceCleanup(t *testing.T) {

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
		createPluginEndpoint := aPack.clt.Endpoint("enterprise", "plugin")

		form := installRequestURLValues(t, testServer.URL, "" /* key to remove */)
		form.Set("csrf_token", aPack.csrfToken)
		resp, err := aPack.clt.PostForm(wSuite.ctx, createPluginEndpoint, form)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		// installing plugin does not immediately create identity center data. So it is safe
		// to assert with existing data created with createICResources function.
		ictestenv.CheckAllICResourcesAreConditionallyDeleted(t, ctx, ictestenv.CheckCleanupArgs{
			SAMLlServiceProviderName: newServcieProviderName,
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

		plugin, err := authClient.PluginsClient().GetPlugin(ctx, &pluginspb.GetPluginRequest{
			Name:        types.PluginTypeAWSIdentityCenter,
			WithSecrets: false,
		})
		require.NoError(t, err)
		require.NotEmpty(t, plugin)
		_, err = authClient.PluginsClient().DeletePlugin(ctx, &pluginspb.DeletePluginRequest{Name: plugin.GetName()})
		require.NoError(t, err)
	})

	t.Run("plugin deletion prevented if user does not have access to all resources that requires deletion", func(t *testing.T) {
		_, err := authClient.PluginsClient().CreatePlugin(ctx, ictestenv.NewPluginV1CreateRequest(icOIDCIntegrationName, newServcieProviderName))
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

		_, err = authClient.PluginsClient().DeletePlugin(ctx, &pluginspb.DeletePluginRequest{Name: types.PluginTypeAWSIdentityCenter})
		require.NoError(t, err)
	})

	t.Run("cleanup after plugin is deleted", func(t *testing.T) {
		ictestenv.CreateAWSOIDCIntegration(t, ctx, authClient, icOIDCIntegrationName)
		installAWSICPlugin(t, ctx, aPack.clt, testServer.URL, aPack.csrfToken)
		ictestenv.CreateICResources(t, ctx, client, testData, string(identitycenter.IdentityCenterDownstreamID))

		deletePluginEndpoint := aPack.clt.Endpoint("enterprise", "plugin", types.PluginTypeAWSIdentityCenter)
		resp, err := aPack.clt.Delete(ctx, deletePluginEndpoint)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		_, err = authClient.PluginsClient().GetPlugin(wSuite.ctx, &pluginspb.GetPluginRequest{
			Name:        types.PluginTypeAWSIdentityCenter,
			WithSecrets: false,
		})
		require.True(t, trace.IsNotFound(err))

		_, err = authClient.GetSAMLIdPServiceProvider(ctx, newServcieProviderName)
		require.True(t, trace.IsNotFound(err))
		_, err = authClient.GetIntegration(ctx, icOIDCIntegrationName)
		require.True(t, trace.IsNotFound(err))

		ictestenv.CheckAllICResourcesAreConditionallyDeleted(t, ctx, ictestenv.CheckCleanupArgs{
			SAMLlServiceProviderName: newServcieProviderName,
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

		needCleanupResp, err := authClient.PluginsClient().NeedsCleanup(ctx, &pluginspb.NeedsCleanupRequest{Type: types.PluginTypeAWSIdentityCenter})
		require.NoError(t, err)
		require.Empty(t, needCleanupResp.GetResourcesToCleanup())
	})

	t.Cleanup(func() {
		testServer.Close()
	})
}

func newAWSIdentityCenterPluginTestSuite(t *testing.T) (*webSuite, *authWebPack, *httptest.Server) {
	t.Helper()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")
	testSPServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/ServiceProviderConfig":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))

	s.webPlugin.pluginDescriptors[types.PluginTypeAWSIdentityCenter] = awsICPluginDescriptor{testSPServer.Client()}

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
			Cloud: true,
		},
	})

	return s, webPack, testSPServer
}

type testCase struct {
	name         string
	form         url.Values
	statusCode   int
	respContains string
}

func samlTestCases(t *testing.T, testServerURL string) []testCase {
	t.Helper()
	return []testCase{
		{
			name:         "missing samlServiceProviderName",
			form:         installRequestURLValues(t, testServerURL, "samlServiceProviderName" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "SAML service provider with samlServiceProviderName already exists",
			form: func() url.Values {
				values := installRequestURLValues(t, testServerURL, "" /* key to remove */)
				values.Set("samlServiceProviderName", existingServcieProviderName)
				return values
			}(),
			statusCode:   http.StatusConflict,
			respContains: "already exists",
		},
		{
			name:         "missing samlServiceProviderMetadata",
			form:         installRequestURLValues(t, testServerURL, "samlServiceProviderMetadata" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "invalid samlServiceProviderMetadata",
			form: func() url.Values {
				values := installRequestURLValues(t, testServerURL, "" /* key to remove */)
				values.Set("samlServiceProviderMetadata", "<xml></xml>")
				return values
			}(),
			statusCode:   http.StatusBadRequest,
			respContains: "invalid metadata",
		},
		{
			name: "existing entity ID for entity descriptor provider in samlServiceProviderMetadata",
			form: func() url.Values {
				values := installRequestURLValues(t, testServerURL, "" /* key to remove */)
				values.Set("samlServiceProviderMetadata", newEntityDescriptor(existingServcieProviderName, fmt.Sprintf("https://%s/acs", existingServcieProviderName)))
				return values
			}(),
			statusCode:   http.StatusConflict,
			respContains: "has the same entity ID",
		},
	}
}

func scimTestCases(t *testing.T, testServerURL string) []testCase {
	t.Helper()
	return []testCase{{
		name:         "missing scimBaseURL",
		form:         installRequestURLValues(t, testServerURL, "scimBaseURL" /* key to remove */),
		statusCode:   http.StatusBadRequest,
		respContains: "required",
	},
		{
			name:         "missing scimAccessToken",
			form:         installRequestURLValues(t, testServerURL, "scimAccessToken" /* key to remove */),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name:         "invalid scimBaseURL",
			form:         installRequestValidURLValues(t, testServerURL+"/status-unauthorized"),
			statusCode:   http.StatusForbidden,
			respContains: "unauthorized",
		},
	}
}

type errorResp struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

const (
	existingServcieProviderName = "existing-service-provider"
	newServcieProviderName      = "saml-sp-1"
	icOIDCIntegrationName       = "ic-oidc-integration"
)

func installRequestValidURLValues(t *testing.T, testServerURL string) url.Values {
	t.Helper()
	return url.Values{
		awsICPluginNameField:                        {types.PluginTypeAWSIdentityCenter},
		"type":                                      {types.PluginTypeAWSIdentityCenter},
		awsICPluginICRegionField:                    {"ca-central-1"},
		awsICPluginICARNField:                       {"arn:aws:sso:::instance/ssoins-8893885e0d4lllka"},
		awsICPluginOIDCIntegrationNameField:         {icOIDCIntegrationName},
		awsICPluginAccessListDefaultOwnersField:     {`["user1", "user2"]`},
		awsICPluginSAMLServiceProviderNameField:     {newServcieProviderName},
		awsICPluginSAMLServiceProviderMetadataField: {newEntityDescriptor("https://example.com", "https://example.com/acs")},
		awsICPluginSCIMBaseURLField:                 {testServerURL},
		awsICPluginSCIMAccessTokenField:             {"abc123example"},
	}
}

func installRequestURLValues(t *testing.T, testServerURL, keyToRemove string) url.Values {
	t.Helper()
	urlVals := installRequestValidURLValues(t, testServerURL)

	if keyToRemove != "" {
		urlVals.Del(keyToRemove)
	}

	return urlVals
}

func installAWSICPlugin(t *testing.T, ctx context.Context, clt *TestWebClient, testServerURL, csrfToken string) {
	t.Helper()
	installPluginEndPoint := clt.Endpoint("enterprise", "plugin")
	form := maps.Clone(installRequestValidURLValues(t, testServerURL))
	form.Set("csrf_token", csrfToken)
	resp, err := clt.PostForm(ctx, installPluginEndPoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
}
