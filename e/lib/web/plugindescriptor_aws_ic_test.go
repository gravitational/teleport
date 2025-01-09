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
	awsicui "github.com/gravitational/teleport/e/lib/web/ui/awsic"
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
	testCases = append(testCases, samlTestCases(t)...)
	testCases = append(testCases, scimTestCases(t)...)
	testCases = append(testCases, testCase{
		name:         "valid",
		form:         installRequestURLValues(t),
		statusCode:   http.StatusOK,
		respContains: "",
	})

	installPluginEndPoint := aPack.clt.Endpoint("enterprise", "plugin")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

	for _, tc := range samlTestCases(t) {
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

		form := installRequestURLValues(t)
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
		installAWSICPlugin(t, ctx, aPack.clt)
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
	testSCIMServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/random-id/scim/v2/ServiceProviderConfig":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))

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
	s.webPlugin.pluginDescriptors[types.PluginTypeAWSIdentityCenter] = awsICPluginDescriptor{client}

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
			Cloud: true,
		},
	})

	return s, webPack, testSCIMServer
}

type testCase struct {
	name         string
	form         url.Values
	statusCode   int
	respContains string
}

func samlTestCases(t *testing.T) []testCase {
	t.Helper()
	return []testCase{
		{
			name:         "missing samlServiceProviderName",
			form:         installRequestURLValues(t, withFieldRemoved("samlServiceProviderName")),
			statusCode:   http.StatusBadRequest,
			respContains: "required",
		},
		{
			name: "SAML service provider with samlServiceProviderName already exists",
			form: func() url.Values {
				values := installRequestURLValues(t)
				values.Set("samlServiceProviderName", existingServcieProviderName)
				return values
			}(),
			statusCode:   http.StatusConflict,
			respContains: "already exists",
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
				values.Set("samlServiceProviderMetadata", newEntityDescriptor(existingServcieProviderName, fmt.Sprintf("https://%s/acs", existingServcieProviderName)))
				return values
			}(),
			statusCode:   http.StatusConflict,
			respContains: "has the same entity ID",
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
	existingServcieProviderName = "existing-service-provider"
	newServcieProviderName      = "saml-sp-1"
	icOIDCIntegrationName       = "ic-oidc-integration"
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
		awsICPluginSAMLServiceProviderNameField:     {newServcieProviderName},
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
	installPluginEndPoint := clt.Endpoint("enterprise", "plugin")
	form := maps.Clone(installRequestValidURLValues(t, validICSCIMBaseURLFormat))
	resp, err := clt.PostForm(ctx, installPluginEndPoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
}

func TestAWSICRegionValidation(t *testing.T) {
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

	wSuite, aPack, _ := newAWSIdentityCenterPluginTestSuite(t)
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := aPack.clt.PostJSON(wSuite.ctx, aPack.clt.Endpoint(tc.path), tc.req)
			hasRegionValidationError(t, tc.req.Region, err)
		})
	}
}
