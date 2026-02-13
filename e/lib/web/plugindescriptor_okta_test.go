package web

import (
	"bytes"
	"context"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

const (
	oktaTestOrg            = "https://test-okta-org.example.com"
	oktaSSOMetadataURLPath = "/app/123487988/sso/saml/metadata"
	oktaSSOMetadataURL     = oktaTestOrg + oktaSSOMetadataURLPath
	oktaSAMLAppName        = "okta_app_name_1" // taken from testEntityDescriptor
	oktaTestClusterName    = "okta-test.teleport.com"
	oktaAPIToken           = "001ABCdefGh_IJkLmnoPQRst23UVwxyz456"
	oktaAppID              = "0oafxqCAJWWGELFTYASJ"
	oktaSCIMToken          = "Ce n'est pas un jeton, pas du tout"
	oktaEveryoneGroupID    = "00gb0c5lmzAl5GbZc5d7"
)

// premadeSAMLSigningKeypair is a keypair used for testing. Reduces test time by
// removing many CPU-intensive kepair creations.
var premadeSAMLSigningKeypair types.AsymmetricKeyPair

// init creates the premadeSAMLSigningKeypair
func init() {
	keyPEM, certPEM, err := utils.GenerateRSASelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport OSS"},
		CommonName:   "teleport.localhost.localdomain",
	}, nil, 10*365*24*time.Hour)
	if err != nil {
		panic(err.Error())
	}
	premadeSAMLSigningKeypair = types.AsymmetricKeyPair{
		PrivateKey: string(keyPEM),
		Cert:       string(certPEM),
	}
}

// testOktaDescriptor is a pluginDescriptor (i.e. plugin install handler) that
// injects resources for testing into the Okta plugin installation process.
type testOktaDescriptor struct {
	httpClient *http.Client
}

// HandleInstallRequest implements pluginDescriptor for the testOktaDescriptor
// type.
func (d testOktaDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	clusterFeatures := p.h.GetClusterFeatures()
	return installOktaPlugin(ctx, installOktaPluginArgs{
		validateOktaPluginInputsArgs: validateOktaPluginInputsArgs{
			form:            r.Form,
			httpClient:      d.httpClient,
			clusterFeatures: &clusterFeatures,
			logger:          p.Logger,
			bcryptCost:      bcrypt.MinCost,
		},
		sessCtx:        sessCtx,
		signingKeypair: &premadeSAMLSigningKeypair,
		plugin:         p,
	})
}

// TranslateCallbackCookie implements pluginDescriptor for the
// testOktaDescriptor type. Always returns NotImplemented.
func (d testOktaDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("testOktaDescriptor.TranslateCallbackCookie")
}

// HandleOAuthStart implements testOktaDescriptor for the
// testOktaDescriptor type. Always returns NotImplemented.
func (d testOktaDescriptor) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	return nil, trace.NotImplemented("testOktaDescriptor.HandleOAuthStart")
}

// HandleValidateConfigRequest implements pluginDescriptor for the
// testOktaDescriptor type.
func (d testOktaDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	clusterFeatures := p.h.GetClusterFeatures()
	args := validateOktaPluginInputsArgs{
		form:            form,
		httpClient:      d.httpClient,
		clusterFeatures: &clusterFeatures,
		logger:          p.Logger,
	}
	_, err := args.validateOktaConfig(ctx, sessCtx)
	return err
}

// HandleUpdateRequest implements pluginUpdateHandler for the testOktaDescriptor type.
func (d testOktaDescriptor) HandleUpdateRequest(ctx context.Context, sessCtx *web.SessionContext, req *ui.PluginUpdateRequest) (*ui.Plugin, error) {
	return updateOktaPlugin(ctx, sessCtx, req)
}

// Static assertion that testOktaDescriptor implements the pluginDescriptor
// interface
var _ pluginDescriptor = testOktaDescriptor{}
var _ pluginUpdateHandler = testOktaDescriptor{}

// newTestOktaPluginFixture creates a set of related
func newTestOktaPluginFixture(t *testing.T, opts ...webSuiteOption) (*webSuite, *authWebPack) {
	// Enable SAML/SSO for testing
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	// Set up a test version of the UI web handler and auth service
	s := newWebSuite(t, append(opts, withModules(testModules))...)
	webPack := s.newAuthWebPack(t, "foo")

	// And add the Role that we will want to assign to Okta users
	_, err := s.testAuthServer.Auth().CreateRole(context.Background(),
		services.NewPresetRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	// Patch the Web Plugin's plugin descriptor map so that any request for the
	// Okta descriptor will use our test descriptor instead
	s.webPlugin.pluginDescriptors[types.PluginTypeOkta] = testOktaDescriptor{}

	return s, webPack
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginUpdate(t *testing.T) {
	const (
		mockOauthToken = "FAKE_OAUTH_TOKEN"
		mockClientID   = "SOME_CLIENT_ID"
	)

	mockta := newRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {

		// Expect the Okta credentials test request
		case withPath(req, "GET", "/api/v1/users"):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			})

		// Expect SSO metadata request
		case withPath(req, "GET", oktaSSOMetadataURLPath):
			return response(http.StatusOK, "application/xml", []byte(testEntityDescriptor))

		// Expect a request to list apps to find info about the SAML app
		case withPath(req, "GET", "/api/v1/apps") && withURLParam(req, "q", oktaSAMLAppName):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     oktaAppID,
					"name":   oktaSAMLAppName,
					"label":  "Teleport App",
					"status": "ACTIVE",
					"_links": map[string]any{
						"metadata": map[string]any{
							"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
							"type": "application/xml",
						},
					},
				},
			})

		// Expect the OAuth token request
		case withPath(req, "POST", "/oauth2/v1/token"):
			scopes := req.URL.Query().Get("scope")
			resp := okta.RequestAccessToken{
				AccessToken: mockOauthToken,
				Scope:       scopes,
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			}
			return jsonResponse(http.StatusOK, resp)

		default:
			return nil, nil
		}
	})

	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OktaSCIM: {Enabled: true},
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	// Set up a test version of the UI web handler and auth service
	s := newWebSuite(t, withRoundTripper(mockta), withModules(testModules))
	webPack := s.newAuthWebPack(t, "foo")

	// And add the Role that we will want to assign to Okta users
	_, err := s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(testModules.TestBuildType))
	require.NoError(t, err)

	// Patch the Web Plugin's plugin descriptor map so that any request for the
	// Okta descriptor will use our test descriptor instead
	s.webPlugin.pluginDescriptors[types.PluginTypeOkta] = testOktaDescriptor{}

	pluginsSvc := s.authPlugin.PluginsService()
	pluginCredsSvc := s.authPlugin.PluginStaticCredentialsService()
	authSvc := s.testAuthServer.AuthServer.AuthServer.Services

	t.Cleanup(func() {
		pluginsSvc.DeleteAllPlugins(s.ctx)
		pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, common.OktaSCIMTokenName)
		pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeOkta)
		authSvc.DeleteSAMLConnector(s.ctx, common.OktaSSOConnectorName)
	})

	// Set entitlements
	features := s.webPlugin.h.GetClusterFeatures()
	features.Entitlements = map[string]*proto.EntitlementInfo{
		string(entitlements.OktaSCIM): {Enabled: true},
		string(entitlements.Identity): {Enabled: true},
	}
	s.webPlugin.h.SetClusterFeatures(features)
	// Set up okta-requester role
	_, err = authSvc.UpsertRole(s.ctx, services.NewSystemOktaAccessRole(modules.BuildEnterprise))
	require.NoError(t, err)
	_, err = authSvc.UpsertRole(s.ctx, services.NewSystemOktaRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	// Set up an existing Okta plugin
	createPluginEndpoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")
	form := url.Values{
		"type":                  {"okta"},
		"name":                  {"okta"},
		"metadataURL":           {oktaSSOMetadataURL},
		"enableUserSync":        {"false"},
		"enableAppGroupsSync":   {"false"},
		"enableAccessListSync":  {"false"},
		"enableSystemLogExport": {"false"},
	}

	// Minimal install – no SCIM token, filters, etc.
	installResp, err := webPack.clt.PostForm(s.ctx, createPluginEndpoint, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, installResp.Code(), "body = %s", installResp.Bytes())

	var installed ui.Plugin
	require.NoError(t, json.Unmarshal(installResp.Bytes(), &installed))
	require.IsType(t, &ui.OktaPluginSpec{}, installed.Spec)

	expectedOktaSpec := &ui.OktaPluginSpec{
		OktaOrgURL:              oktaTestOrg,
		TeleportSSOConnector:    common.OktaSSOConnectorName,
		EnableUserSync:          false,
		AssignDefaultRoles:      true,
		EnableAppGroupSync:      false,
		EnableAccessListSync:    false,
		EnableBidirectionalSync: false,
		EnableSystemLogExport:   false,
		CredentialInfo: &ui.OktaCredentialInfo{
			HasConfiguredOauthCredentials: false,
			HasConfiguredSCIMToken:        false,
			HasConfiguredSSMSToken:        false,
		},
	}
	require.Equal(t, expectedOktaSpec, installed.Spec.(*ui.OktaPluginSpec))

	// Confirm the plugin was created in the backend
	_, err = pluginsSvc.GetPlugin(s.ctx, types.PluginTypeOkta, false)
	require.NoError(t, err)

	// Update req to set ClientID and enable User Sync
	updateReq := ui.PluginUpdateRequest{
		Plugin: types.PluginTypeOkta,
		Okta: &ui.OktaPluginUpdate{
			ClientID:              mockClientID,
			EnableUserSync:        true,
			AssignDefaultRoles:    true,
			EnableAppGroupSync:    false,
			EnableAccessListSync:  false,
			EnableSystemLogExport: true,
		},
	}
	updatePluginEndpoint := webPack.clt.Endpoint("enterprise", "plugin")
	updateResp, err := webPack.clt.PutJSON(s.ctx, updatePluginEndpoint, updateReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updateResp.Code())

	var updated ui.Plugin
	require.NoError(t, json.Unmarshal(updateResp.Bytes(), &updated))
	require.IsType(t, &ui.OktaPluginSpec{}, updated.Spec)
	require.Equal(t, "okta", updated.Name)

	expectedOktaSpec = &ui.OktaPluginSpec{
		OktaOrgURL:            oktaTestOrg,
		OktaAppID:             oktaAppID,
		OktaAppName:           oktaSAMLAppName,
		TeleportSSOConnector:  common.OktaSSOConnectorName,
		EnableUserSync:        true,
		AssignDefaultRoles:    true,
		EnableAppGroupSync:    false,
		EnableAccessListSync:  false,
		EnableSystemLogExport: true,
		CredentialInfo: &ui.OktaCredentialInfo{
			HasConfiguredOauthCredentials: true,
		},
	}
	require.Equal(t, expectedOktaSpec, updated.Spec.(*ui.OktaPluginSpec))

	// Verify the backend plugin resource has the same values
	plg, err := pluginsSvc.GetPlugin(s.ctx, types.PluginTypeOkta, true /* with secrets */)
	require.NoError(t, err)
	oktaPlugin, ok := plg.(*types.PluginV1)
	require.True(t, ok, "Expected *types.PluginV1 after update")

	expectedSettings := &types.PluginOktaSettings{
		OrgUrl: oktaTestOrg,
		SyncSettings: &types.PluginOktaSyncSettings{
			AppId:                     oktaAppID,
			AppName:                   oktaSAMLAppName,
			SyncUsers:                 true,
			DisableAssignDefaultRoles: false,
			SyncAccessLists:           false,
			DisableSyncAppGroups:      true,
			DisableBidirectionalSync:  true,
			EnableSystemLogExport:     true,
			SsoConnectorId:            common.OktaSSOConnectorName,
			UserSyncSource:            string(types.OktaUserSyncSourceSamlApp),
		},
		CredentialsInfo: &types.PluginOktaCredentialsInfo{
			HasOauthCredentials: true,
		},
	}
	require.Equal(t, expectedSettings, oktaPlugin.Spec.GetOkta())

	pluginCreds, err := pluginCredsSvc.GetPluginStaticCredentialsByLabels(s.ctx, oktaPlugin.GetCredentials().GetStaticCredentialsRef().Labels)
	require.NoError(t, err)
	storedClientId, _ := pluginCreds[0].GetOAuthClientSecret()
	require.Equal(t, mockClientID, storedClientId)
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallWithNewSAMLConnector(t *testing.T) {
	appFilters := []string{"app1", "app2"}
	groupFilters := []string{"group1", "group2"}
	defaultOwners := []string{"owner1", "owner2"}

	testCases := []struct {
		name                      string
		orgURL                    string
		enableOktaSCIMEntitlement bool
		enabledAccessListSync     bool
		correctedOrgURL           string
		expectSCIMToken           require.ValueAssertionFunc
		expectSCIMTokenCred       require.ErrorAssertionFunc
		expectAppFilters          require.ValueAssertionFunc
		expectGroupFilters        require.ValueAssertionFunc
		expectDefaultOwners       require.ValueAssertionFunc
	}{
		{
			name:                      "full org URL",
			orgURL:                    oktaTestOrg,
			enableOktaSCIMEntitlement: true,
			enabledAccessListSync:     true,
			correctedOrgURL:           oktaTestOrg,
			expectSCIMToken:           requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred:       require.NoError,
			expectAppFilters:          requireEqualTo(appFilters),
			expectGroupFilters:        requireEqualTo(groupFilters),
			expectDefaultOwners:       requireEqualTo(defaultOwners),
		},
		{
			name:                      "missing URL scheme is fixed",
			orgURL:                    "test-okta-org.example.com",
			enableOktaSCIMEntitlement: true,
			enabledAccessListSync:     false,
			correctedOrgURL:           oktaTestOrg,
			expectSCIMToken:           requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred:       require.NoError,
			expectAppFilters:          require.Empty,
			expectGroupFilters:        require.Empty,
			expectDefaultOwners:       require.Empty,
		},
		{
			name:                      "IGS disabled",
			orgURL:                    oktaTestOrg,
			enableOktaSCIMEntitlement: false,
			enabledAccessListSync:     false,
			correctedOrgURL:           oktaTestOrg,
			expectSCIMToken:           require.Empty,
			expectSCIMTokenCred:       requireNotFound,
			expectAppFilters:          require.Empty,
			expectGroupFilters:        require.Empty,
			expectDefaultOwners:       require.Empty,
		},
	}

	mockta := newRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {

		// Expect the Okta credentials test request
		case withPath(req, "GET", "/api/v1/users"):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			})

		// Expect SSO metadata request
		case withPath(req, "GET", oktaSSOMetadataURLPath):
			return response(http.StatusOK, "application/xml", []byte(testEntityDescriptor))

		// Expect a request to list apps to find info about the SAML app
		case withPath(req, "GET", "/api/v1/apps") && withURLParam(req, "q", oktaSAMLAppName):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     oktaAppID,
					"name":   oktaSAMLAppName,
					"label":  "Teleport App",
					"status": "ACTIVE",
					"_links": map[string]any{
						"metadata": map[string]any{
							"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
							"type": "application/xml",
						},
					},
				},
			})

		default:
			return nil, nil
		}
	})

	s, webPack := newTestOktaPluginFixture(t, withRoundTripper(mockta))
	pluginsSvc := s.authPlugin.PluginsService()
	pluginCredsSvc := s.authPlugin.PluginStaticCredentialsService()
	authSvc := s.testAuthServer.AuthServer.AuthServer.Services
	ctx := context.Background()
	_, err := authSvc.UpsertRole(ctx, services.NewSystemOktaAccessRole(modules.BuildEnterprise))
	require.NoError(t, err)
	_, err = authSvc.UpsertRole(ctx, services.NewSystemOktaRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	// When I invoke the installer via the web interface...
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// All of these sub-test cases re-use the same auth server over and
			// over, so we need to ensure any resources we create are destroyed
			// at the end of the test. Unfortunately we can't simply create a new
			// fixture for each test case because doing the setup 100x for each
			// case breaches the time limit on the flaky test detector.
			t.Cleanup(func() {
				pluginsSvc.DeleteAllPlugins(s.ctx)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, common.OktaSCIMTokenName)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeOkta)
				authSvc.DeleteSAMLConnector(s.ctx, common.OktaSSOConnectorName)
			})

			features := s.webPlugin.h.GetClusterFeatures()
			features.Entitlements = map[string]*proto.EntitlementInfo{
				string(entitlements.OktaSCIM): {Enabled: testCase.enableOktaSCIMEntitlement},
			}
			s.webPlugin.h.SetClusterFeatures(features)

			modulestest.SetTestModules(t, modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.OktaSCIM: {Enabled: testCase.enableOktaSCIMEntitlement},
					},
				},
			})

			form := url.Values{
				"type":        {"okta"},
				"orgURL":      {testCase.orgURL},
				"metadataURL": {oktaSSOMetadataURL},
				"apiToken":    {oktaAPIToken},
			}
			if testCase.enableOktaSCIMEntitlement {
				form.Set("scimToken", oktaSCIMToken)

				appFilterB, err := json.Marshal(appFilters)
				require.NoError(t, err)
				form.Set("appFilters", string(appFilterB))

				groupFilterB, err := json.Marshal(groupFilters)
				require.NoError(t, err)
				form.Set("groupFilters", string(groupFilterB))

				if testCase.enabledAccessListSync {
					defaultOwnerB, err := json.Marshal(defaultOwners)
					require.NoError(t, err)
					form.Set("defaultOwners", string(defaultOwnerB))
				}
			}
			response, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, form)

			// Expect that both the HTTP round trip and actual request succeeded
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.Code(), "body = %s", response.Bytes())

			// Expect that the response is a JSON-encoded ui.Plugin with a
			// trailing io.OktaPluginSpec{}
			var plugin ui.Plugin
			require.NoError(t, json.Unmarshal(response.Bytes(), &plugin))
			require.Equal(t, "okta", plugin.Name)
			require.Equal(t, types.PluginType(types.PluginTypeOkta), plugin.Type)
			require.Equal(t, types.PluginStatusCode_UNKNOWN, plugin.StatusCode)
			require.Contains(t, plugin.Details, "Okta")

			require.IsType(t, &ui.OktaPluginSpec{}, plugin.Spec)
			spec := plugin.Spec.(*ui.OktaPluginSpec)
			require.Equal(t, oktaAppID, spec.OktaAppID)
			require.Equal(t, oktaSAMLAppName, spec.OktaAppName)
			require.Equal(t, "Teleport App", spec.OktaAppLabel)
			require.Equal(t, common.OktaSSOConnectorName, spec.TeleportSSOConnector)
			testCase.expectSCIMToken(t, spec.SCIMBearerToken)

			// Expect that the plugin resource was created
			plg, err := pluginsSvc.GetPlugin(s.ctx, types.PluginTypeOkta, false)
			require.NoError(t, err, "failed to load expected plugin")
			oktaPlg, ok := plg.(*types.PluginV1)
			require.True(t, ok, "expected conversion to *types.PluginV1")

			// Expect that the Okta plugin settings were set correctly
			oktaSettings := oktaPlg.Spec.GetOkta()
			require.NotNil(t, oktaSettings)
			require.Equal(t, testCase.correctedOrgURL, oktaSettings.OrgUrl)

			testCase.expectAppFilters(t, oktaSettings.SyncSettings.AppFilters)
			testCase.expectGroupFilters(t, oktaSettings.SyncSettings.GroupFilters)
			testCase.expectDefaultOwners(t, oktaSettings.SyncSettings.DefaultOwners)
			require.Equal(t, testCase.enabledAccessListSync, oktaSettings.SyncSettings.SyncAccessLists)

			syncSettings := oktaPlg.Spec.GetOkta().SyncSettings
			require.NotNil(t, syncSettings)
			require.True(t, syncSettings.SyncUsers)
			require.Equal(t, oktaAppID, syncSettings.AppId)
			require.Equal(t, common.OktaSSOConnectorName, syncSettings.SsoConnectorId)

			// Expect that the Okta API token exists
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			require.NoError(t, err)

			// Expect that the Okta SCIM token cred is in the appropriate state
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, common.OktaSCIMTokenName)
			testCase.expectSCIMTokenCred(t, err)

			// Expect that the SAML connector was created
			ssoCtor, err := s.proxyClient.GetSAMLConnector(s.ctx, common.OktaSSOConnectorName, false)
			require.NoError(t, err, "failed to load expected plugin")
			require.Equal(t, types.OriginOkta, ssoCtor.Origin())
			labels := ssoCtor.GetMetadata().Labels
			require.Equal(t, oktaTestOrg, labels[eteleport.OktaOrgURLLabel])
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallWorksWithLegacySAMLConnector(t *testing.T) {
	mockta := newRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {

		// Expect the Okta credentials test request
		case withPath(req, "GET", "/api/v1/users"):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			})

		// Expect a request to list apps to find info about the SAML app
		case withPath(req, "GET", "/api/v1/apps") && withURLParam(req, "q", oktaSAMLAppName):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     oktaAppID,
					"name":   oktaSAMLAppName,
					"label":  "Teleport App",
					"status": "ACTIVE",
					"_links": map[string]any{
						"metadata": map[string]any{
							"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
							"type": "application/xml",
						},
					},
				},
			})

		default:
			return nil, nil
		}
	})

	s, webPack := newTestOktaPluginFixture(t, withRoundTripper(mockta))

	// Given a cluster with an existing SAML connector that does not have the
	// labels that identify the App it talks to
	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: common.OktaSSOConnectorName,
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", oktaTestClusterName, common.OktaSSOConnectorName),
			Display:                  "Test SAML Connector",
			EntityDescriptor:         testEntityDescriptor,
			SigningKeyPair:           &premadeSAMLSigningKeypair,
			AttributesToRoles: []types.AttributeMapping{
				{
					Name:  "groups",
					Value: "testers",
					Roles: []string{teleport.PresetRequesterRoleName},
				},
			},
		},
	}
	require.NoError(t, samlConnector.CheckAndSetDefaults())
	_, err := s.testAuthServer.Auth().CreateSAMLConnector(s.ctx, samlConnector)
	require.NoError(t, err)

	// When I invoke the Okta installer via the WebUI
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")
	resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, url.Values{
		"type":      {"okta"},
		"orgURL":    {oktaTestOrg},
		"apiToken":  {oktaAPIToken},
		"scimToken": {oktaSCIMToken},
	})

	// Expect that the HTTP round trip succeeded
	require.NoError(t, err)

	// Expect that the install went through
	require.Equal(t, http.StatusOK, resp.Code())

	//  Expect that the plugin was created
	_, err = s.authPlugin.PluginsService().GetPlugin(s.ctx, types.PluginTypeOkta, false)
	require.NoError(t, err)
}

func requireEqualTo(expected any) require.ValueAssertionFunc {
	return func(t require.TestingT, value any, msgAndArgs ...any) {
		require.Equal(t, expected, value, msgAndArgs...)
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallWithExistingSAMLConnector(t *testing.T) {
	testCases := []struct {
		name                      string
		enableOktaSCIMEntitlement bool
		expectSCIMToken           require.ValueAssertionFunc
		expectSCIMTokenCred       require.ErrorAssertionFunc
	}{
		{
			name:                      "OktaSCIM Enabled",
			enableOktaSCIMEntitlement: true,
			expectSCIMToken:           requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred:       require.NoError,
		}, {
			name:                      "OktaSCIM Disabled",
			enableOktaSCIMEntitlement: false,
			expectSCIMToken:           require.Empty,
			expectSCIMTokenCred:       requireNotFound,
		},
	}

	mockta := newRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {

		// Expect the Okta credentials test request
		case withPath(req, "GET", "/api/v1/users"):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			})

		// Expect a request to list apps to find info about the SAML app
		case withPath(req, "GET", "/api/v1/apps") && withURLParam(req, "q", oktaSAMLAppName):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     oktaAppID,
					"name":   oktaSAMLAppName,
					"label":  "Teleport App",
					"status": "ACTIVE",
					"_links": map[string]any{
						"metadata": map[string]any{
							"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
							"type": "application/xml",
						},
					},
				},
			})

		default:
			err := fmt.Errorf("unmatched HTTP call method=%q url=%q req=%v", req.Method, req.URL, req)
			t.Log(err.Error())
			return nil, err
		}
	})
	s, webPack := newTestOktaPluginFixture(t, withRoundTripper(mockta))
	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: common.OktaSSOConnectorName,
			Labels: map[string]string{
				types.OriginLabel:         types.OriginOkta,
				eteleport.OktaOrgURLLabel: oktaTestOrg,
				eteleport.OktaAppIDLabel:  oktaAppID,
			},
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", oktaTestClusterName, common.OktaSSOConnectorName),
			Display:                  "Test SAML Connector",
			EntityDescriptor:         testEntityDescriptor,
			SigningKeyPair:           &premadeSAMLSigningKeypair,
			AttributesToRoles: []types.AttributeMapping{
				{
					Name:  "groups",
					Value: "testers",
					Roles: []string{teleport.PresetRequesterRoleName},
				},
			},
		},
	}
	require.NoError(t, samlConnector.CheckAndSetDefaults())
	pluginsSvc := s.authPlugin.PluginsService()
	pluginCredsSvc := s.authPlugin.PluginStaticCredentialsService()
	authSvc := s.testAuthServer.AuthServer.AuthServer.Services
	createdSAMLConn, err := authSvc.CreateSAMLConnector(s.ctx, samlConnector)
	require.NoError(t, err)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// All of these sub-test cases re-use the same auth server over and
			// over, so we need to ensure any resources we create are destroyed
			// at the end of the test. Unfortunately we can't simply create a new
			// fixture for each test case because doing the setup 100x for each
			// case breaches the time limit on the flaky test detector.
			t.Cleanup(func() {
				pluginsSvc.DeleteAllPlugins(s.ctx)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, common.OktaSCIMTokenName)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			})

			features := s.webPlugin.h.GetClusterFeatures()
			features.Entitlements = map[string]*proto.EntitlementInfo{
				string(entitlements.OktaSCIM): {Enabled: testCase.enableOktaSCIMEntitlement},
			}
			s.webPlugin.h.SetClusterFeatures(features)
			modulestest.SetTestModules(t, modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.OktaSCIM: {Enabled: testCase.enableOktaSCIMEntitlement},
					},
				},
			})

			// When I invoke the installer via the web interface...
			installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")
			form := url.Values{
				"type":     {"okta"},
				"orgURL":   {oktaTestOrg},
				"apiToken": {oktaAPIToken},
			}
			if testCase.enableOktaSCIMEntitlement {
				form.Set("scimToken", oktaSCIMToken)
			}
			resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, form)

			// Expect that both the HTTP round trip AND succeeded operation succeeded
			// the request itself succeeded
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code(), "resp.Body = %s", resp.Bytes())

			// Expect that the response is a JSON-encoded ui.Plugin with a trailing
			// ui.OktaPluginSpec
			var plugin ui.Plugin
			require.NoError(t, json.Unmarshal(resp.Bytes(), &plugin))
			require.Equal(t, "okta", plugin.Name)
			require.Equal(t, types.PluginType(types.PluginTypeOkta), plugin.Type)
			require.Equal(t, types.PluginStatusCode_UNKNOWN, plugin.StatusCode)
			require.Contains(t, plugin.Details, "Okta")

			require.IsType(t, &ui.OktaPluginSpec{}, plugin.Spec)
			spec := plugin.Spec.(*ui.OktaPluginSpec)
			require.Equal(t, oktaAppID, spec.OktaAppID)
			require.Equal(t, oktaSAMLAppName, spec.OktaAppName)
			require.Equal(t, "Teleport App", spec.OktaAppLabel)
			require.Equal(t, common.OktaSSOConnectorName, spec.TeleportSSOConnector)
			testCase.expectSCIMToken(t, spec.SCIMBearerToken)

			// Expect that the backend plugin resource was created
			plg, err := pluginsSvc.GetPlugin(s.ctx, types.PluginTypeOkta, false)
			require.NoError(t, err, "failed to load expected plugin")
			oktaPlg, ok := plg.(*types.PluginV1)
			require.True(t, ok, "expected conversion to *types.PluginV1")

			oktaSettings := oktaPlg.Spec.GetOkta()
			require.NotNil(t, oktaSettings)
			require.Equal(t, oktaTestOrg, oktaSettings.OrgUrl)

			syncSettings := oktaPlg.Spec.GetOkta().SyncSettings
			require.NotNil(t, syncSettings)
			require.True(t, syncSettings.SyncUsers)
			require.Equal(t, oktaAppID, syncSettings.AppId)
			require.Equal(t, common.OktaSSOConnectorName, syncSettings.SsoConnectorId)

			// Expect that the Okta API token exists
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			require.NoError(t, err)

			// Expect that the Okta SCIM token cred is in the appropriate state
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, common.OktaSCIMTokenName)
			testCase.expectSCIMTokenCred(t, err)

			// Expect that the SAML connector was not touched
			backendSAMLConn, err := s.proxyClient.GetSAMLConnector(s.ctx, common.OktaSSOConnectorName, false)
			require.NoError(t, err, "failed to load expected connector")
			require.Equal(t, createdSAMLConn.GetRevision(), backendSAMLConn.GetRevision())
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallFailsWithInvalidFormValues(t *testing.T) {
	testCases := []struct {
		name            string
		form            url.Values
		expectedPattern string
	}{
		{
			name: "missing-org-url",
			form: url.Values{
				"type":      {"okta"},
				"apiToken":  {oktaAPIToken},
				"scimToken": {oktaSCIMToken},
			},
			expectedPattern: "missing Okta organization URL",
		}, {
			name: "missing-api-token",
			form: url.Values{
				"type":      {"okta"},
				"orgURL":    {oktaTestOrg},
				"scimToken": {oktaSCIMToken},
			},
			expectedPattern: "Okta API credentials not provided in the request and Okta plugin does not exist",
		}, {
			name: "missing-scim-token",
			form: url.Values{
				"type":     {"okta"},
				"orgURL":   {oktaTestOrg},
				"apiToken": {oktaAPIToken},
			},
			expectedPattern: "missing SCIM bearer token",
		}, {
			name: "malformed-org-url",
			form: url.Values{
				"type":      {"okta"},
				"orgURL":    {"someutterrubsh\a\r\nthatisNotAnURL"},
				"apiToken":  {oktaAPIToken},
				"scimToken": {oktaSCIMToken},
			},
			expectedPattern: "malformed",
		},
	}

	mockta := newRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {

		// Expect the Okta credentials test request
		case withPath(req, "GET", "/api/v1/users"):
			return jsonResponse(http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			})

		default:
			return nil, fmt.Errorf("unmatched HTTP call method=%q url=%q req=%v", req.Method, req.URL, req)
		}
	})
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OktaSCIM: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	// Set up a test version of the UI web handler and auth service
	s := newWebSuite(t, withRoundTripper(mockta), withModules(testModules))
	webPack := s.newAuthWebPack(t, "foo")

	// And add the Role that we will want to assign to Okta users
	_, err := s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(testModules.TestBuildType))
	require.NoError(t, err)

	// Patch the Web Plugin's plugin descriptor map so that any request for the
	// Okta descriptor will use our test descriptor instead
	s.webPlugin.pluginDescriptors[types.PluginTypeOkta] = testOktaDescriptor{}

	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")

	features := s.webPlugin.h.GetClusterFeatures()
	features.Entitlements = map[string]*proto.EntitlementInfo{
		string(entitlements.OktaSCIM): {Enabled: true},
	}
	s.webPlugin.h.SetClusterFeatures(features)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			form := maps.Clone(testCase.form)

			resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.Code())
			require.Contains(t, string(resp.Bytes()), testCase.expectedPattern)
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallInvalidOktaConfig(t *testing.T) {
	testCases := []struct {
		name              string
		roundTripResponse *http.Response
		roundTripErr      error
	}{
		{
			name: "invalid token",
			roundTripResponse: mustJSONResponse(t, http.StatusUnauthorized, map[string]any{
				"errorCode":    "E0000011",
				"errorSummary": "Invalid token provided",
				"errorLink":    "E0000011",
				"errorId":      "...oaeb7TuItptQcq47JYCaTB-7Q",
				"errorCauses":  []string{},
			}),
			roundTripErr: nil,
		}, {
			name:              "invalid org url",
			roundTripResponse: nil,
			roundTripErr:      &net.OpError{Err: errors.New("something bad happened")},
		},
	}
	mockta := newRoundTripper(nil)
	s, webPack := newTestOktaPluginFixture(t, withRoundTripper(mockta))
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "staticauth")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// setup the mock
			mockta.RoundTripFn = func(req *http.Request) (*http.Response, error) {
				switch {
				// Expect the Okta credentials test request
				case withPath(req, "GET", "/api/v1/users"):
					return testCase.roundTripResponse, testCase.roundTripErr
				default:
					return nil, nil
				}
			}

			resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, url.Values{
				"type":      {"okta"},
				"orgURL":    {oktaTestOrg},
				"apiToken":  {oktaAPIToken},
				"scimToken": {oktaSCIMToken},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.Code())
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaConfigValidate(t *testing.T) {
	testCases := []struct {
		name             string
		form             url.Values
		credTestResponse *http.Response
		credTestErr      error
		expectedStatus   int
	}{
		{
			name: "valid-config",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
				"orgURL":   {oktaTestOrg},
			},
			credTestResponse: mustJSONResponse(t, http.StatusOK, []map[string]any{
				{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				},
			}),
			credTestErr:    nil,
			expectedStatus: http.StatusOK,
		}, {
			name: "missing-org-url",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
			},
			expectedStatus: http.StatusBadRequest,
		}, {
			name: "missing-api-token",
			form: url.Values{
				"type":   {"okta"},
				"orgURL": {oktaTestOrg},
			},
			expectedStatus: http.StatusBadRequest,
		}, {
			name: "malformed-org-url",
			form: url.Values{
				"type":     {"okta"},
				"orgURL":   {"someutterrubsh\a\r\nthatisNotAnURL"},
				"apiToken": {oktaAPIToken},
			},
			expectedStatus: http.StatusBadRequest,
		}, {
			name: "token rejected",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
				"orgURL":   {oktaTestOrg},
			},
			credTestResponse: mustJSONResponse(t, http.StatusUnauthorized, map[string]any{
				"errorCode":    "E0000011",
				"errorSummary": "Invalid token provided",
				"errorLink":    "E0000011",
				"errorId":      "...oaeb7TuItptQcq47JYCaTB-7Q",
				"errorCauses":  []string{},
			}),
			credTestErr:    nil,
			expectedStatus: http.StatusBadRequest,
		}, {
			name: "bad network",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
				"orgURL":   {oktaTestOrg},
			},
			credTestResponse: nil,
			credTestErr:      &net.OpError{Err: errors.New("something bad happened")},
			expectedStatus:   http.StatusBadRequest,
		},
	}

	mockta := newRoundTripper(nil)
	s, webPack := newTestOktaPluginFixture(t, withRoundTripper(mockta))
	validateEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "validate")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// setup the mock
			mockta.RoundTripFn = func(req *http.Request) (*http.Response, error) {
				switch {
				// Expect the Okta credentials test request
				case withPath(req, "GET", "/api/v1/users"):
					return testCase.credTestResponse, testCase.credTestErr
				default:
					return nil, nil
				}
			}

			form := maps.Clone(testCase.form)

			resp, err := webPack.clt.PostForm(s.ctx, validateEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, testCase.expectedStatus, resp.Code())
		})
	}
}

type roundTripper struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func newRoundTripper(fn func(req *http.Request) (*http.Response, error)) *roundTripper {
	return &roundTripper{fn}
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.RoundTripFn(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp == nil {
		return nil, fmt.Errorf("unmatched HTTP call method=%q url=%q req=%v", req.Method, req.URL, req)
	}
	return resp, nil
}

func withPath(req *http.Request, method, path string) bool {
	return req.Method == method && req.URL.Path == path
}

func withURLParam(req *http.Request, name string, value ...string) bool {
	return slices.Equal(req.URL.Query()[name], value)
}

func jsonResponse(statusCode int, body any) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r, err := response(statusCode, "application/json", bodyBytes)
	return r, trace.Wrap(err)
}

func mustJSONResponse(t *testing.T, statusCode int, body any) *http.Response {
	t.Helper()
	r, err := jsonResponse(statusCode, body)
	require.NoError(t, err, "jsonResponse")
	return r
}

func response(statusCode int, contentType string, body []byte) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type":   {contentType},
			"Content-Length": {strconv.Itoa(len(body))},
		},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	return resp, nil
}

func requireNotFound(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

const testEntityDescriptor = `
<?xml version="1.0"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2021-02-26T15:57:24Z" cacheDuration="PT1614787044S" entityID="http://some.entity.id">
	<md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
	<md:KeyDescriptor use="signing">
		<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
		<ds:X509Data>
			<ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
		</ds:X509Data>
		</ds:KeyInfo>
	</md:KeyDescriptor>
	<md:KeyDescriptor use="encryption">
		<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
		<ds:X509Data>
			<ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
		</ds:X509Data>
		</ds:KeyInfo>
	</md:KeyDescriptor>
	<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
	<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://test-okta-org.example.com/app/okta_app_name_1/random_stuff/sso/saml"/>
	</md:IDPSSODescriptor>
</md:EntityDescriptor>`
