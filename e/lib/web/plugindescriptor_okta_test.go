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
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
)

const (
	oktaTestOrg         = "https://example-org.okta.com"
	oktaTestClusterName = "okta-test.teleport.com"
	oktaAPIToken        = "001ABCdefGh_IJkLmnoPQRst23UVwxyz456"
	oktaAppID           = "0oafxqCAJWWGELFTYASJ"
	oktaSCIMToken       = "Ceci n'est pas un jeton"
	oktaEveryoneGroupID = "00gb0c5lmzAl5GbZc5d7"
)

// premadeSAMLSigningKeypair is a keypair used for testing. Reduces test time by
// removing many CPU-intensive kepair creations.
var premadeSAMLSigningKeypair types.AsymmetricKeyPair

// init creates the premadeSAMLSigningKeypair
func init() {
	keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
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
	return installOktaPlugin(ctx, installOktaPluginArgs{
		validateOktaPluginInputsArgs: validateOktaPluginInputsArgs{
			form:            r.Form,
			httpClient:      d.httpClient,
			clusterFeatures: &p.h.ClusterFeatures,
			log:             p.Log,
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

// HandleValidateConfigRequest implements pluginDescriptor for the
// testOktaDescriptor type.
func (d testOktaDescriptor) HandleValidateConfigRequest(ctx context.Context, _ *web.SessionContext, form url.Values, p *Plugin) error {
	args := validateOktaPluginInputsArgs{
		form:            form,
		httpClient:      d.httpClient,
		clusterFeatures: &p.h.ClusterFeatures,
		log:             p.Log,
	}
	_, err := args.validateOktaConfig(ctx)
	return err
}

// Static assertion that testOktaDescriptor implements the pluginDescriptor
// interface
var _ pluginDescriptor = testOktaDescriptor{}

// newTestOktaPluginFixture creates a set of related
func newTestOktaPluginFixture(t *testing.T) (*webSuite, *authWebPack, *mockRoundTripper) {
	// Enable SAML/SSO for testing
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
	})

	// Set up a test version of the UI web handler and auth service
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	// And add the Role that we will want to assign to Okta users
	_, err := s.testAuthServer.Auth().CreateRole(context.Background(),
		services.NewPresetRequesterRole())
	require.NoError(t, err)

	// Patch the Web Plugin's plugin descriptor map so that any request for the
	// Okta descriptor will use our test descriptor instead
	mockta := &mockRoundTripper{}
	s.webPlugin.pluginDescriptors[types.PluginTypeOkta] =
		testOktaDescriptor{&http.Client{Transport: mockta}}

	return s, webPack, mockta
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallWithNewSAMLConnector(t *testing.T) {
	logrus.SetFormatter(logutils.NewDefaultTextFormatter(true))
	logrus.SetLevel(logrus.TraceLevel)

	appFilters := []string{"app1", "app2"}
	groupFilters := []string{"group1", "group2"}
	defaultOwners := []string{"owner1", "owner2"}

	testCases := []struct {
		name                  string
		orgURL                string
		enableIGS             bool
		enabledAccessListSync bool
		correctedOrgURL       string
		expectSCIMToken       require.ValueAssertionFunc
		expectSCIMTokenCred   require.ErrorAssertionFunc
		expectAppFilters      require.ValueAssertionFunc
		expectGroupFilters    require.ValueAssertionFunc
		expectDefaultOwners   require.ValueAssertionFunc
	}{
		{
			name:                  "full org URL",
			orgURL:                oktaTestOrg,
			enableIGS:             true,
			enabledAccessListSync: true,
			correctedOrgURL:       oktaTestOrg,
			expectSCIMToken:       requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred:   require.NoError,
			expectAppFilters:      requireEqualTo(appFilters),
			expectGroupFilters:    requireEqualTo(groupFilters),
			expectDefaultOwners:   requireEqualTo(defaultOwners),
		},
		{
			name:                  "missing URL scheme is fixed",
			orgURL:                "example-org.okta.com",
			enableIGS:             true,
			enabledAccessListSync: false,
			correctedOrgURL:       "https://example-org.okta.com",
			expectSCIMToken:       requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred:   require.NoError,
			expectAppFilters:      require.Empty,
			expectGroupFilters:    require.Empty,
			expectDefaultOwners:   require.Empty,
		},
		{
			name:                  "IGS disabled",
			orgURL:                oktaTestOrg,
			enableIGS:             false,
			enabledAccessListSync: false,
			correctedOrgURL:       oktaTestOrg,
			expectSCIMToken:       require.Empty,
			expectSCIMTokenCred:   requireNotFound,
			expectAppFilters:      require.Empty,
			expectGroupFilters:    require.Empty,
			expectDefaultOwners:   require.Empty,
		},
	}

	s, webPack, mockta := newTestOktaPluginFixture(t)
	pluginsSvc := s.authPlugin.PluginsService()
	pluginCredsSvc := s.authPlugin.PluginStaticCredentialsService()
	authSvc := s.testAuthServer.AuthServer.AuthServer.Services
	ctx := context.Background()
	_, err := authSvc.UpsertRole(ctx, services.NewSystemOktaAccessRole())
	require.NoError(t, err)
	_, err = authSvc.UpsertRole(ctx, services.NewSystemOktaRequesterRole())
	require.NoError(t, err)

	// Expect the Okta credentials test request
	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/users/me")).
		Run(requireCreds(t, oktaAPIToken)).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "00ub0c5ls7iixvj6j5d7",
			"status": "ACTIVE",
			"profile": map[string]any{
				"firstName": "Norville",
				"lastName":  "Rogers",
				"nickName":  "Shaggy",
				"login":     "shaggy@mystery-machine.org",
				"email":     "shaggy@mystery-machine.org",
			},
		}), nil)

	// Expect the Okta SAML App creation request
	mockta.
		On("RoundTrip", requestForPath(
			"POST", "/api/v1/apps")).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     oktaAppID,
			"name":   "Teleport_App_plus_index",
			"label":  "Teleport App",
			"status": "ACTIVE",
			"_links": map[string]any{
				"metadata": map[string]any{
					"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
					"type": "application/xml",
				},
			},
		}), nil)

	// Expect the Okta group listing request
	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/groups")).
		Return(jsonResponse(t, http.StatusOK, []map[string]any{
			{
				"id":   oktaEveryoneGroupID,
				"type": "BUILT_IN",
				"profile": map[string]any{
					"name": "Everyone",
				},
			},
		}), nil)

	// Expect a request assigning the Everyone group to the new App
	mockta.
		On("RoundTrip", requestForPath(
			"PUT", fmt.Sprintf("/api/v1/apps/%s/groups/%s", oktaAppID, oktaEveryoneGroupID))).
		Return(jsonResponse(t, http.StatusOK, map[string]any{}), nil)

	// Expect a request for the SAML entity metadata XML
	mockta.
		On("RoundTrip", requestForPath(
			"GET", fmt.Sprintf("/api/v1/apps/%s/sso/saml/metadata", oktaAppID))).
		Return(func(*http.Request) (*http.Response, error) {
			metadata := response(t, http.StatusOK, "application/xml", []byte(testEntityDescriptor))
			return metadata, nil
		})

	// Expect a request for the Org metadata
	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/org")).
		Return(
			jsonResponse(t, http.StatusOK, map[string]any{
				"companyName": "testOrg",
			}), nil)

	// When I invoke the installer via the web interface...
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugin")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// All of these sub-test cases re-use the same auth server over and
			// over, so we need to ensure any resources we create are destroyed
			// at the end of the test. Unfortunately we can't simply create a new
			// fixture for each test case because doing the setup 100x for each
			// case breaches the time limit on the flaky test detector.
			t.Cleanup(func() {
				pluginsSvc.DeleteAllPlugins(s.ctx)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, oktaSCIMTokenName)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeOkta)
				authSvc.DeleteSAMLConnector(s.ctx, oktaSSOConnectorName)
			})

			s.webPlugin.h.ClusterFeatures.Entitlements = map[string]*proto.EntitlementInfo{
				string(entitlements.Identity): {Enabled: testCase.enableIGS},
			}

			form := url.Values{
				"type":       {"okta"},
				"orgURL":     {testCase.orgURL},
				"apiToken":   {oktaAPIToken},
				"csrf_token": {webPack.csrfToken},
			}
			if testCase.enableIGS {
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
			require.Equal(t, http.StatusOK, response.Code())

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
			require.Equal(t, "Teleport_App_plus_index", spec.OktaAppName)
			require.Equal(t, "Teleport App", spec.OktaAppLabel)
			require.Equal(t, oktaSSOConnectorName, spec.TeleportSSOConnector)
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
			require.Equal(t, oktaSSOConnectorName, syncSettings.SsoConnectorId)

			// Expect that the Okta API token exists
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			require.NoError(t, err)

			// Expect that the Okta SCIM token cred is in the appropriate state
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, oktaSCIMTokenName)
			testCase.expectSCIMTokenCred(t, err)

			// Expect that the SAML connector was created
			ssoCtor, err := s.proxyClient.GetSAMLConnector(s.ctx, oktaSSOConnectorName, false)
			require.NoError(t, err, "failed to load expected plugin")
			require.Equal(t, types.OriginOkta, ssoCtor.Origin())
			labels := ssoCtor.GetMetadata().Labels
			require.Equal(t, oktaAppID, labels[eteleport.OktaAppIDLabel])
			require.Equal(t, oktaTestOrg, labels[eteleport.OktaOrgURLLabel])
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallFailsWithLegacySAMLConnector(t *testing.T) {
	s, webPack, mockta := newTestOktaPluginFixture(t)

	// Given a cluster with an existing SAML connector that does not have the
	// labels that identify the App it talks to
	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: oktaSSOConnectorName,
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", oktaTestClusterName, oktaSSOConnectorName),
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

	// Given an Okta service that will only expect the credential validation
	// request ...
	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/users/me")).
		Run(requireCreds(t, oktaAPIToken)).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "00ub0c5ls7iixvj6j5d7",
			"status": "ACTIVE",
			"profile": map[string]any{
				"firstName": "Norville",
				"lastName":  "Rogers",
				"nickName":  "Shaggy",
				"login":     "shaggy@mystery-machine.org",
				"email":     "shaggy@mystery-machine.org",
			},
		}), nil)

	// When I invoke the Okta installer via the WebUI
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugin")
	resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, url.Values{
		"type":       {"okta"},
		"orgURL":     {oktaTestOrg},
		"apiToken":   {oktaAPIToken},
		"scimToken":  {oktaSCIMToken},
		"csrf_token": {webPack.csrfToken},
	})

	// Expect that the HTTP round trip succeeded
	require.NoError(t, err)

	// Expect that the install failed
	require.Equal(t, http.StatusPreconditionFailed, resp.Code())

	//  Expect that the plugin was not created
	_, err = s.authPlugin.PluginsService().GetPlugin(s.ctx, types.PluginTypeOkta, false)
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func requireEqualTo(expected interface{}) require.ValueAssertionFunc {
	return func(t require.TestingT, value interface{}, msgAndArgs ...interface{}) {
		require.Equal(t, expected, value, msgAndArgs...)
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaPluginInstallWithExistingSAMLConnector(t *testing.T) {
	testCases := []struct {
		name                string
		igsEnabled          bool
		expectSCIMToken     require.ValueAssertionFunc
		expectSCIMTokenCred require.ErrorAssertionFunc
	}{
		{
			name:                "IGS Enabled",
			igsEnabled:          true,
			expectSCIMToken:     requireEqualTo(oktaSCIMToken),
			expectSCIMTokenCred: require.NoError,
		}, {
			name:                "IGS Disabled",
			igsEnabled:          false,
			expectSCIMToken:     require.Empty,
			expectSCIMTokenCred: requireNotFound,
		},
	}

	s, webPack, mockta := newTestOktaPluginFixture(t)
	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: oktaSSOConnectorName,
			Labels: map[string]string{
				types.OriginLabel:         types.OriginOkta,
				eteleport.OktaOrgURLLabel: oktaTestOrg,
				eteleport.OktaAppIDLabel:  oktaAppID,
			},
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", oktaTestClusterName, oktaSSOConnectorName),
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

	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/users/me")).
		Run(requireCreds(t, oktaAPIToken)).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "00ub0c5ls7iixvj6j5d7",
			"status": "ACTIVE",
			"profile": map[string]any{
				"firstName": "Norville",
				"lastName":  "Rogers",
				"nickName":  "Shaggy",
				"login":     "shaggy@mystery-machine.org",
				"email":     "shaggy@mystery-machine.org",
			},
		}), nil)

	mockta.
		On("RoundTrip", requestForPath(
			"GET", "/api/v1/apps/"+oktaAppID)).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     oktaAppID,
			"name":   "Teleport_App_plus_index",
			"label":  "Teleport App",
			"status": "ACTIVE",
			"_links": map[string]any{
				"metadata": map[string]any{
					"href": fmt.Sprintf("%s/api/v1/apps/%s/sso/saml/metadata", oktaTestOrg, oktaAppID),
					"type": "application/xml",
				},
			},
		}), nil)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// All of these sub-test cases re-use the same auth server over and
			// over, so we need to ensure any resources we create are destroyed
			// at the end of the test. Unfortunately we can't simply create a new
			// fixture for each test case because doing the setup 100x for each
			// case breaches the time limit on the flaky test detector.
			t.Cleanup(func() {
				pluginsSvc.DeleteAllPlugins(s.ctx)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, oktaSCIMTokenName)
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			})

			s.webPlugin.h.ClusterFeatures.Entitlements = map[string]*proto.EntitlementInfo{
				string(entitlements.Identity): {Enabled: testCase.igsEnabled},
			}

			// When I invoke the installer via the web interface...
			installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugin")
			form := url.Values{
				"type":       {"okta"},
				"orgURL":     {oktaTestOrg},
				"apiToken":   {oktaAPIToken},
				"csrf_token": {webPack.csrfToken},
			}
			if testCase.igsEnabled {
				form.Set("scimToken", oktaSCIMToken)
			}
			resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, form)

			// Expect that both the HTTP round trip AND succeeded operation succeeded
			// the request itself succeeded
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

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
			require.Equal(t, "Teleport_App_plus_index", spec.OktaAppName)
			require.Equal(t, "Teleport App", spec.OktaAppLabel)
			require.Equal(t, oktaSSOConnectorName, spec.TeleportSSOConnector)
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
			require.Equal(t, oktaSSOConnectorName, syncSettings.SsoConnectorId)

			// Expect that the Okta API token exists
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, types.PluginTypeOkta)
			require.NoError(t, err)

			// Expect that the Okta SCIM token cred is in the appropriate state
			_, err = pluginCredsSvc.GetPluginStaticCredentials(s.ctx, oktaSCIMTokenName)
			testCase.expectSCIMTokenCred(t, err)

			// Expect that the SAML connector was not touched
			backendSAMLConn, err := s.proxyClient.GetSAMLConnector(s.ctx, oktaSSOConnectorName, false)
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
			expectedPattern: "missing Okta API token",
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

	s, webPack, mockta := newTestOktaPluginFixture(t)
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugin")

	s.webPlugin.h.ClusterFeatures.Entitlements = map[string]*proto.EntitlementInfo{
		string(entitlements.Identity): {Enabled: true},
	}

	mockta.On("RoundTrip", requestForPath("GET", "/api/v1/users/me")).
		Maybe().
		Run(requireCreds(t, oktaAPIToken)).
		Return(jsonResponse(t, http.StatusOK, map[string]any{
			"id":     "00ub0c5ls7iixvj6j5d7",
			"status": "ACTIVE",
			"profile": map[string]any{
				"firstName": "Norville",
				"lastName":  "Rogers",
				"nickName":  "Shaggy",
				"login":     "shaggy@mystery-machine.org",
				"email":     "shaggy@mystery-machine.org",
			},
		}), nil)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			form := maps.Clone(testCase.form)
			form.Set("csrf_token", webPack.csrfToken)

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
		name            string
		roundTripResult []any
	}{
		{
			name: "invalid token",
			roundTripResult: []any{
				jsonResponse(t, http.StatusUnauthorized, map[string]any{
					"errorCode":    "E0000011",
					"errorSummary": "Invalid token provided",
					"errorLink":    "E0000011",
					"errorId":      "...oaeb7TuItptQcq47JYCaTB-7Q",
					"errorCauses":  []string{},
				}),
				nil,
			},
		}, {
			name: "invalid org url",
			roundTripResult: []any{
				nil,
				&net.OpError{Err: errors.New("something bad happened")},
			},
		},
	}

	s, webPack, mockta := newTestOktaPluginFixture(t)
	installPluginEndPoint := webPack.clt.Endpoint("enterprise", "plugin")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// reset the mock
			mockta.Mock = mock.Mock{}
			mockta.
				On("RoundTrip", requestForPath("GET", "/api/v1/users/me")).
				Return(testCase.roundTripResult...)

			resp, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, url.Values{
				"type":       {"okta"},
				"orgURL":     {oktaTestOrg},
				"apiToken":   {oktaAPIToken},
				"scimToken":  {oktaSCIMToken},
				"csrf_token": {webPack.csrfToken},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.Code())
		})
	}
}

//nolint:bodyclose // The http.Requests created in this function are cleaned up by the request consumers
func TestOktaConfigValidate(t *testing.T) {
	testCases := []struct {
		name           string
		form           url.Values
		credTestResult []any
		expectedStatus int
	}{
		{
			name: "valid-config",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
				"orgURL":   {oktaTestOrg},
			},
			credTestResult: []any{
				jsonResponse(t, http.StatusOK, map[string]any{
					"id":     "00ub0c5ls7iixvj6j5d7",
					"status": "ACTIVE",
					"profile": map[string]any{
						"firstName": "Norville",
						"lastName":  "Rogers",
						"nickName":  "Shaggy",
						"login":     "shaggy@mystery-machine.org",
						"email":     "shaggy@mystery-machine.org",
					},
				}),
				nil,
			},
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
			credTestResult: []any{
				jsonResponse(t, http.StatusUnauthorized, map[string]any{
					"errorCode":    "E0000011",
					"errorSummary": "Invalid token provided",
					"errorLink":    "E0000011",
					"errorId":      "...oaeb7TuItptQcq47JYCaTB-7Q",
					"errorCauses":  []string{},
				}),
				nil,
			},
			expectedStatus: http.StatusBadRequest,
		}, {
			name: "bad network",
			form: url.Values{
				"type":     {"okta"},
				"apiToken": {oktaAPIToken},
				"orgURL":   {oktaTestOrg},
			},
			credTestResult: []any{
				nil,
				&net.OpError{Err: errors.New("something bad happened")},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	s, webPack, mockta := newTestOktaPluginFixture(t)
	validateEndPoint := webPack.clt.Endpoint("enterprise", "plugins", "validate")

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockta.Mock = mock.Mock{}

			if len(testCase.credTestResult) > 0 {
				mockta.
					On("RoundTrip", requestForPath("GET", "/api/v1/users/me")).
					Run(requireCreds(t, oktaAPIToken)).
					Return(testCase.credTestResult...)
			}

			form := maps.Clone(testCase.form)
			form.Set("csrf_token", webPack.csrfToken)

			resp, err := webPack.clt.PostForm(s.ctx, validateEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, testCase.expectedStatus, resp.Code())
		})
	}
}

func requireCreds(innerT *testing.T, token string) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		req, ok := args.Get(0).(*http.Request)
		require.True(innerT, ok, "Unexpected request type: %T", args.Get(0))
		require.Equal(innerT, "SSWS "+token, req.Header.Get("Authorization"))
	}
}

type mockRoundTripper struct {
	mock.Mock
}

// RoundTrip implements the RoundTripper interface for the mockRoundTripper
func (m *mockRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	args := m.Called(request)

	// Sometimes we want to invoke a function and return the result of that
	// function as the mock call result. Testify doesn't let us do that out of
	// the box but, by convention, we simulate it by allowing the test to supply
	// a function with the same signature as the mocked-put method as a `Return()`
	// value.
	fn, isDelegate := args.Get(0).(func(request *http.Request) (*http.Response, error))
	if isDelegate {
		return fn(request)
	}

	maybeResponse := args.Get(0)
	if maybeResponse == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// requestForPath returns a testify mock argument matcher that will match any
// HTTP request with the given method and path
func requestForPath(method, path string) interface{} {
	return mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == method && req.URL.Path == path
	})
}

func jsonResponse(t *testing.T, statusCode int, body any) *http.Response {
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err, "formatting response body")
	return response(t, statusCode, "application/json", bodyBytes)
}

func response(t *testing.T, statusCode int, contentType string, body []byte) *http.Response {
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
	return resp
}

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
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
	<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="http://example.com/saml/acs/example"/>
	</md:IDPSSODescriptor>
</md:EntityDescriptor>`
