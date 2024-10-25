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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/samlsp"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

func TestAWSICCreatePlugin(t *testing.T) {
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)

	installAWSICSAMLServiceProvider(t, wSuite.ctx, authClient)

	awsIg, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "existing-oidc-integration"},
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
	installAWSICSAMLServiceProvider(t, wSuite.ctx, authClient)

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

func TestAWSICDeletePlugin(t *testing.T) {
	wSuite, aPack, testServer := newAWSIdentityCenterPluginTestSuite(t)
	authClient := wSuite.newAdminAuthClient(wSuite.ctx, t)

	installAWSICOIDCIntegration(t, wSuite.ctx, authClient)
	installAWSICPlugin(t, wSuite.ctx, aPack.clt, testServer.URL, aPack.csrfToken)

	deletePluginEndPoint := aPack.clt.Endpoint("enterprise", "plugin", types.PluginTypeAWSIdentityCenter)

	resp, err := aPack.clt.Delete(wSuite.ctx, deletePluginEndPoint)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.NoError(t, err)

	// test both plugin and saml service provider is deleted
	_, err = authClient.GetSAMLIdPServiceProvider(wSuite.ctx, newServcieProviderName)
	require.True(t, trace.IsNotFound(err))
	_, err = authClient.PluginsClient().GetPlugin(wSuite.ctx, &pluginspb.GetPluginRequest{
		Name:        types.PluginTypeAWSIdentityCenter,
		WithSecrets: false,
	})
	require.True(t, trace.IsNotFound(err))

	// test a scenario where a SAML service provider may not exist but
	// the plugin deletion should still succeed.
	// manually creating the plugin in order to skip creating SAML service provider. This wont be
	// true in production but will let us test the failed cleanup message.
	_, err = authClient.PluginsClient().CreatePlugin(wSuite.ctx, newPlugin(t, testServer.URL))
	require.NoError(t, err)

	resp2, err := aPack.clt.Delete(wSuite.ctx, deletePluginEndPoint)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, resp2.Code())

	require.Contains(t, string(resp2.Bytes()), "doesn't exist")

	_, err = authClient.PluginsClient().GetPlugin(wSuite.ctx, &pluginspb.GetPluginRequest{
		Name:        types.PluginTypeAWSIdentityCenter,
		WithSecrets: false,
	})
	require.True(t, trace.IsNotFound(err))
}

func newPlugin(t *testing.T, testServerURL string) *pluginspb.CreatePluginRequest {
	inputs := installRequestValidURLValues(t, testServerURL)
	return &pluginspb.CreatePluginRequest{
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
						IntegrationName:         inputs.Get(awsICPluginOIDCIntegrationNameField),
						Region:                  inputs.Get(awsICPluginICRegionField),
						Arn:                     inputs.Get(awsICPluginICARNField),
						AccessListDefaultOwners: []string{"user1", "user2"},
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: testServerURL,
						},
					},
				},
			},
		},
	}
}

func newAWSIdentityCenterPluginTestSuite(t *testing.T) (*webSuite, *authWebPack, *httptest.Server) {
	t.Helper()
	s := newWebSuite(t)
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

const existingServcieProviderName = "existing-service-provider"
const newServcieProviderName = "saml-sp-1"

func installRequestValidURLValues(t *testing.T, testServerURL string) url.Values {
	t.Helper()
	return url.Values{
		awsICPluginNameField:                        {types.PluginTypeAWSIdentityCenter},
		"type":                                      {types.PluginTypeAWSIdentityCenter},
		awsICPluginICRegionField:                    {"ca-central-1"},
		awsICPluginICARNField:                       {"arn:aws:sso:::instance/ssoins-8893885e0d4lllka"},
		awsICPluginOIDCIntegrationNameField:         {"existing-oidc-integration"},
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

func installAWSICSAMLServiceProvider(t *testing.T, ctx context.Context, authClient authclient.ClientI) {
	t.Helper()
	sp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: existingServcieProviderName,
			Labels: map[string]string{
				types.OriginLabel: common.OriginAWSIdentityCenter,
			},
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor(existingServcieProviderName, fmt.Sprintf("https://%s/acs", existingServcieProviderName)),
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)
}

func installAWSICOIDCIntegration(t *testing.T, ctx context.Context, authClient authclient.ClientI) {
	t.Helper()
	awsOIDCIg, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "existing-oidc-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/DevTeams",
			IssuerS3URI: "s3://my-bucket/my-prefix",
		},
	)
	require.NoError(t, err)
	_, err = authClient.CreateIntegration(ctx, awsOIDCIg)
	require.NoError(t, err)
}
