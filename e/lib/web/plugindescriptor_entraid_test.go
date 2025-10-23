package web

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

func TestEntraIDCreatePlugin(t *testing.T) {
	env := createEntraIDTEnv(t)
	connectorName := createEntraConnector(t, env.s)

	testCases := []testCase{
		{
			name:         "missing name",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("name")),
			statusCode:   http.StatusBadRequest,
			respContains: "name must be specified",
		},
		{
			name:         "missing type",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("type")),
			statusCode:   http.StatusBadRequest,
			respContains: "unknown plugin type",
		},
		{
			name:         "missing defaultOwners",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("defaultOwners")),
			statusCode:   http.StatusBadRequest,
			respContains: "default owners must be specified",
		},
		{
			name:         "missing tenantID",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("tenantId")),
			statusCode:   http.StatusBadRequest,
			respContains: "tenant ID must be specified",
		},
		{
			name:         "missing clientID",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("clientId")),
			statusCode:   http.StatusBadRequest,
			respContains: "client ID must be specified",
		},
		{
			name:         "missing authConnectorName",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("authConnectorName")),
			statusCode:   http.StatusBadRequest,
			respContains: "auth connector name must be specified",
		},
		{
			name:         "duplicate authConnectorName",
			form:         entraInstallRequestURLValues(t, withFieldOverride("authConnectorName", connectorName)),
			statusCode:   http.StatusConflict,
			respContains: "already exists",
		},
		{
			name:         "bad group filter regex",
			form:         entraInstallRequestURLValues(t, withFieldOverride("groupFilters", `{"id":["c8d8f374-1072-4adf-aa5e-75036e8ccc41"],"nameRegex":["^[)$"]}`)),
			statusCode:   http.StatusBadRequest,
			respContains: "error parsing regexp",
			cleanupFunc: func() {
				require.NoError(t, env.s.testAuthServer.Auth().DeleteSAMLConnector(t.Context(), types.PluginTypeEntraID))
			},
		},
		{
			name:       "missing groupFilters",
			form:       entraInstallRequestURLValues(t, withFieldRemoved("groupFilters")),
			statusCode: http.StatusOK,
			cleanupFunc: func() {
				require.NoError(t, env.s.testAuthServer.Auth().DeleteSAMLConnector(t.Context(), types.PluginTypeEntraID))
				require.NoError(t, env.s.testAuthServer.Auth().DeleteIntegration(t.Context(), types.PluginTypeEntraID))
				require.NoError(t, env.s.testAuthServer.Auth().DeletePlugin(t.Context(), types.PluginTypeEntraID))
			},
		},
		{
			name:       "missing groupFilters",
			form:       entraInstallRequestURLValues(t, withFieldRemoved("groupFilters")),
			statusCode: http.StatusOK,
			cleanupFunc: func() {
				require.NoError(t, env.s.testAuthServer.Auth().DeleteSAMLConnector(t.Context(), types.PluginTypeEntraID))
				require.NoError(t, env.s.testAuthServer.Auth().DeleteIntegration(t.Context(), types.PluginTypeEntraID))
				require.NoError(t, env.s.testAuthServer.Auth().DeletePlugin(t.Context(), types.PluginTypeEntraID))
			},
		},
		{
			name: "duplicate integration",
			setupFunc: func() {
				createAzureIntegration(t, env.s, "entra-id-duplicate", "123-tenant-id", "123-client-id")
			},
			form:         entraInstallRequestURLValues(t, withFieldOverride("name", "entra-id-duplicate")),
			statusCode:   http.StatusConflict,
			respContains: "already exists",
			cleanupFunc: func() {
				require.NoError(t, env.s.testAuthServer.Auth().DeleteSAMLConnector(t.Context(), types.PluginTypeEntraID))
				require.NoError(t, env.s.testAuthServer.Auth().DeleteIntegration(t.Context(), "entra-id-duplicate"))
			},
		},
		{
			name:       "valid",
			form:       entraInstallRequestURLValues(t),
			statusCode: http.StatusOK,
		},
	}

	installPluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugins", "staticauth")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupFunc != nil {
				tc.setupFunc()
			}
			form := maps.Clone(tc.form)
			resp, err := env.pack.clt.PostForm(t.Context(), installPluginEndPoint, form)
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.Code())

			if tc.respContains != "" {
				var respMessage errorResp
				err = json.Unmarshal(resp.Bytes(), &respMessage)
				require.NoError(t, err)
				require.Contains(t, respMessage.Error.Message, tc.respContains)
			}

			if tc.statusCode == http.StatusOK {
				connector, err := env.s.testAuthServer.Auth().GetSAMLConnector(t.Context(), tc.form.Get("authConnectorName"), false)
				require.NoError(t, err)
				require.NotNil(t, connector)

				integration, err := env.s.testAuthServer.Auth().GetIntegration(t.Context(), tc.form.Get("name"))
				require.NoError(t, err)
				require.NotNil(t, integration)

				plugin, err := env.s.testAuthServer.Auth().GetPlugin(t.Context(), tc.form.Get("name"), false)
				require.NoError(t, err)
				require.NotNil(t, plugin)
			}

			if tc.cleanupFunc != nil {
				tc.cleanupFunc()
			}
		})
	}
}

func TestEntraIDValidatePlugin(t *testing.T) {
	env := createEntraIDTEnv(t)
	connectorName := createEntraConnector(t, env.s)

	testCases := []testCase{
		{
			name:         "missing name",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("name")),
			statusCode:   http.StatusBadRequest,
			respContains: "name must be specified",
		},
		{
			name:         "missing type",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("type")),
			statusCode:   http.StatusBadRequest,
			respContains: "unknown plugin type",
		},
		{
			name:         "missing defaultOwners",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("defaultOwners")),
			statusCode:   http.StatusBadRequest,
			respContains: "default owners must be specified",
		},
		{
			name:         "bad group filter regex",
			form:         entraInstallRequestURLValues(t, withFieldOverride("groupFilters", `{"id":["c8d8f374-1072-4adf-aa5e-75036e8ccc41"],"nameRegex":["^[)$"]}`)),
			statusCode:   http.StatusBadRequest,
			respContains: "error parsing regexp",
		},
		{
			name:       "missing groupFilters",
			form:       entraInstallRequestURLValues(t, withFieldRemoved("groupFilters")),
			statusCode: http.StatusOK,
		},
		{
			name:         "missing authConnectorName",
			form:         entraInstallRequestURLValues(t, withFieldRemoved("authConnectorName")),
			statusCode:   http.StatusBadRequest,
			respContains: "auth connector name must be specified",
		},
		{
			name:         "existing authConnectorName",
			form:         entraInstallRequestURLValues(t, withFieldOverride("authConnectorName", connectorName)),
			statusCode:   http.StatusBadRequest,
			respContains: "already exists",
		},
		{
			name:       "valid",
			form:       entraInstallRequestURLValues(t),
			statusCode: http.StatusOK,
		},
	}

	installPluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugins", "validate")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			form := maps.Clone(tc.form)
			resp, err := env.pack.clt.PostForm(t.Context(), installPluginEndPoint, form)
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
}

func createEntraConnector(t *testing.T, s *webSuite) string {
	t.Helper()
	_, err := s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole())
	require.NoError(t, err)
	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: types.PluginTypeEntraID + "-custom",
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", "localhost", types.PluginTypeEntraID),
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
	_, err = s.testAuthServer.Auth().CreateSAMLConnector(t.Context(), samlConnector)
	require.NoError(t, err)

	return samlConnector.GetName()
}

func createAzureIntegration(t *testing.T, s *webSuite, name, tenantID, clientID string) {
	t.Helper()
	spec, err := types.NewIntegrationAzureOIDC(
		types.Metadata{Name: name},
		&types.AzureOIDCIntegrationSpecV1{
			TenantID: tenantID,
			ClientID: clientID,
		},
	)
	require.NoError(t, err)

	_, err = s.testAuthServer.Auth().CreateIntegration(t.Context(), spec)
	require.NoError(t, err)
}

func withFieldOverride(key, val string) reqOpts {
	return func(urlVals url.Values) {
		urlVals.Set(key, val)
	}
}

func entraInstallRequestURLValues(t *testing.T, opts ...reqOpts) url.Values {
	t.Helper()
	req := entraInstallRequestValidURLValues(t)
	for _, opt := range opts {
		opt(req)
	}
	return req
}

func entraInstallRequestValidURLValues(t *testing.T) url.Values {
	t.Helper()
	return url.Values{
		"name":              {types.PluginTypeEntraID},
		"type":              {types.PluginTypeEntraID},
		"authConnectorName": {types.PluginTypeEntraID},
		"defaultOwners":     {`["user1", "user2"]`},
		"tenantId":          {"eae3bfbd-3246-47fe-8a37-c20b9eb433a2"},
		"clientId":          {"e1b69fdf-8e18-47f0-864c-94ca7f8d0e1f"},
		"groupFilters":      {`{"id":["c8d8f374-1072-4adf-aa5e-75036e8ccc41"],"nameRegex":[],"excludeId":[],"excludeNameRegex":["finance-*"]}`},
	}
}

type entraIDTEnv struct {
	s    *webSuite
	pack *authWebPack
}

func createEntraIDTEnv(t *testing.T) entraIDTEnv {
	t.Helper()
	s := newWebSuite(t)
	// Set entitlements
	features := s.webPlugin.h.GetClusterFeatures()
	features.Entitlements = map[string]*proto.EntitlementInfo{
		string(entitlements.Identity): {Enabled: true},
	}
	s.webPlugin.h.SetClusterFeatures(features)
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
				entitlements.SAML:     {Enabled: true},
			},
		},
	})

	cfg := entraIDPluginDescriptor{testEntityDescriptor: testEntityDescriptor}
	s.webPlugin.pluginDescriptors[types.PluginTypeEntraID] = cfg

	return entraIDTEnv{
		s:    s,
		pack: s.newAuthWebPack(t, "foo"),
	}
}
