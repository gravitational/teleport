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
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/services"
)

func TestEntraIDCreatePlugin(t *testing.T) {
	t.Parallel()
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
			name:         "invalid accessListOwnersSource",
			form:         entraInstallRequestURLValues(t, withFieldOverride("accessListOwnersSource", "unknown")),
			statusCode:   http.StatusBadRequest,
			respContains: "unexpected Access List owners source",
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
	t.Parallel()
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

func TestEntraIDUpdatePlugin(t *testing.T) {
	t.Parallel()
	env := createEntraIDTEnv(t)
	_, err := env.s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	installPluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugins", "staticauth")
	resp, err := env.pack.clt.PostForm(t.Context(), installPluginEndPoint, entraInstallRequestURLValues(t))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	updatePluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugin")
	testCases := []struct {
		name         string
		req          *ui.PluginUpdateRequest
		errAssertion require.ErrorAssertionFunc
		statusCode   int
	}{
		{
			name: "missing name",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					DefaultOwners: []string{"user1", "user2"},
					GroupFilters: filter.Inputs{
						ID: []string{"c8d8f374-1072-4adf-aa5e-75036e8ccc41"},
					},
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "name is required")
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "empty default owners",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					Name:          types.PluginTypeEntraID,
					DefaultOwners: []string{},
					GroupFilters: filter.Inputs{
						ID: []string{"c8d8f374-1072-4adf-aa5e-75036e8ccc41"},
					},
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "owners cannot be empty")
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "invalid group filters",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					Name:          types.PluginTypeEntraID,
					DefaultOwners: []string{"user1", "user2"},
					GroupFilters: filter.Inputs{
						ID:        []string{"c8d8f374-1072-4adf-aa5e-75036e8ccc41"},
						NameRegex: []string{"^[)$"},
					},
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid group filter")
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "invalid owner source",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					Name:                   types.PluginTypeEntraID,
					DefaultOwners:          []string{"user3"},
					AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_UNSPECIFIED.String(),
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unexpected Access List owners source")
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "invalid sync interval",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					Name:                   types.PluginTypeEntraID,
					DefaultOwners:          []string{"user3"},
					AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID.String(),
					SyncIntervals:          &types.PluginEntraIDSyncIntervals{Delta: "4h", Full: "2h"}, // Delta greater than full not allowed.
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "delta sync interval value should be less than")
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "valid",
			req: &ui.PluginUpdateRequest{
				Plugin: types.PluginTypeEntraID,
				EntraID: &ui.EntraIDPluginUpdate{
					Name:          types.PluginTypeEntraID,
					DefaultOwners: []string{"user3"},
					GroupFilters: filter.Inputs{
						ID: []string{"abc-id"},
					},
					AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID.String(),
					SyncIntervals:          &types.PluginEntraIDSyncIntervals{Delta: "2m", Full: "1h"},
				},
			},
			errAssertion: require.NoError,
			statusCode:   http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err = env.pack.clt.PutJSON(t.Context(), updatePluginEndPoint, tc.req)
			tc.errAssertion(t, err)
			require.Equal(t, tc.statusCode, resp.Code())
		})
	}
}

func TestEntraIDPluginUpdatePreservesStatus(t *testing.T) {
	t.Parallel()
	env := createEntraIDTEnv(t)
	_, err := env.s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	// install plugin
	installPluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugins", "staticauth")
	resp, err := env.pack.clt.PostForm(t.Context(), installPluginEndPoint, entraInstallRequestURLValues(t))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	existingPlugin, err := env.s.testAuthServer.AuthServer.AuthServer.Plugins.GetPlugin(t.Context(), types.PluginTypeEntraID, false)
	require.NoError(t, err)
	require.NotNil(t, existingPlugin)
	require.Nil(t, existingPlugin.GetStatus().GetEntraId())

	// update plugin with status
	existingPlugin.SetStatus(&types.PluginStatusV1{
		Code: types.PluginStatusCode_RUNNING,
		Details: &types.PluginStatusV1_EntraId{
			EntraId: &types.PluginEntraIDStatusV1{
				ImportedUsers:  10,
				ImportedGroups: 12,
			},
		},
	})
	pluginWithStatus, err := env.s.testAuthServer.AuthServer.AuthServer.Plugins.UpdatePlugin(t.Context(), existingPlugin)
	require.NoError(t, err)

	// update plugin settings
	updateluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugin")
	resp, err = env.pack.clt.PutJSON(t.Context(), updateluginEndPoint, &ui.PluginUpdateRequest{
		Plugin: types.PluginTypeEntraID,
		EntraID: &ui.EntraIDPluginUpdate{
			Name:                   types.PluginTypeEntraID,
			DefaultOwners:          []string{"user3"}, // new owner
			AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID.String(),
			GroupFilters:           filter.Inputs{}, // empty filters
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// check settings update does not override plugin status
	updatedPlugin, err := env.s.testAuthServer.AuthServer.AuthServer.Plugins.GetPlugin(t.Context(), types.PluginTypeEntraID, false)
	require.NoError(t, err)

	updatedPluginV1, ok := updatedPlugin.(*types.PluginV1)
	require.True(t, ok, "expected plugin type to be PluginV1")

	expectedSettings := updatedPluginV1.Spec.GetEntraId().SyncSettings
	require.ElementsMatch(t, expectedSettings.DefaultOwners, []string{"user3"})
	require.Equal(t, filter.ToInputs(expectedSettings.GroupFilters), filter.ToInputs([]*types.PluginSyncFilter{}))

	expectedStatus := pluginWithStatus.GetStatus().GetEntraId()
	wantStatus := updatedPluginV1.GetStatus().GetEntraId()
	require.Equal(t, expectedStatus.ImportedUsers, wantStatus.ImportedUsers)
	require.Equal(t, expectedStatus.ImportedGroups, wantStatus.ImportedGroups)
}

// TestEntraIDPluginUpdatePreservesSyncIntervals checks existing sync intervals are preserved
// when an older Web UI client hits the proxy endpoint without sync interval fields.
func TestEntraIDPluginUpdatePreservesSyncIntervals(t *testing.T) {
	t.Parallel()
	env := createEntraIDTEnv(t)
	_, err := env.s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(modules.BuildEnterprise))
	require.NoError(t, err)

	// Install plugin, web endpoint does not currently support sync interval config on create request.
	installPluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugins", "staticauth")
	resp, err := env.pack.clt.PostForm(t.Context(), installPluginEndPoint, entraInstallRequestURLValues(t))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	installedPlugin, err := env.s.testAuthServer.AuthServer.AuthServer.Plugins.GetPlugin(t.Context(), types.PluginTypeEntraID, false)
	require.NoError(t, err)

	// Update plugin out-of-band from the proxy to mimic sync interval updated using tctl.
	pluginV1, ok := installedPlugin.(*types.PluginV1)
	require.True(t, ok, "expected plugin type to be PluginV1")
	settings := pluginV1.Spec.GetEntraId()
	settings.SyncSettings.SyncIntervals = &types.PluginEntraIDSyncIntervals{Delta: "2m", Full: "1h"}
	pluginV1.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: settings,
	}
	updatedPlugin, err := env.s.testAuthServer.AuthServer.AuthServer.Plugins.UpdatePlugin(t.Context(), pluginV1)
	require.NoError(t, err)

	updatedPluginV1, ok := updatedPlugin.(*types.PluginV1)
	require.True(t, ok, "expected plugin type to be PluginV1")
	require.Equal(t, &types.PluginEntraIDSyncIntervals{Delta: "2m", Full: "1h"}, updatedPluginV1.Spec.GetEntraId().SyncSettings.SyncIntervals)

	// Update plugin without the sync interval, mimicking plugin update request from an older UI clients.
	updatePluginEndPoint := env.pack.clt.Endpoint("enterprise", "plugin")
	resp, err = env.pack.clt.PutJSON(t.Context(), updatePluginEndPoint, &ui.PluginUpdateRequest{
		Plugin: types.PluginTypeEntraID,
		EntraID: &ui.EntraIDPluginUpdate{
			Name:                   types.PluginTypeEntraID,
			DefaultOwners:          []string{"user3"}, // new owner
			AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID.String(),
			// missing sync intervals
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// Check settings update does not override sync interval
	updatedPlugin, err = env.s.testAuthServer.AuthServer.AuthServer.Plugins.GetPlugin(t.Context(), types.PluginTypeEntraID, false)
	require.NoError(t, err)
	updatedPluginV1, ok = updatedPlugin.(*types.PluginV1)
	require.True(t, ok, "expected plugin type to be PluginV1")

	gotSettings := updatedPluginV1.Spec.GetEntraId().SyncSettings
	// Verify new owner was updated.
	require.ElementsMatch(t, []string{"user3"}, gotSettings.DefaultOwners)
	require.Equal(t, filter.ToInputs([]*types.PluginSyncFilter{}), filter.ToInputs(gotSettings.GroupFilters))
	// Verify sync interval remains the same.
	require.Equal(t, &types.PluginEntraIDSyncIntervals{Delta: "2m", Full: "1h"}, updatedPluginV1.Spec.GetEntraId().SyncSettings.SyncIntervals)

	// Update sync interval using web API.
	resp, err = env.pack.clt.PutJSON(t.Context(), updatePluginEndPoint, &ui.PluginUpdateRequest{
		Plugin: types.PluginTypeEntraID,
		EntraID: &ui.EntraIDPluginUpdate{
			Name:                   types.PluginTypeEntraID,
			DefaultOwners:          []string{"user3"}, // new owner
			AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID.String(),
			SyncIntervals:          &types.PluginEntraIDSyncIntervals{Delta: "1m", Full: "0s"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// Verify the web API updated the sync intervals.
	updatedPlugin, err = env.s.testAuthServer.AuthServer.AuthServer.Plugins.GetPlugin(t.Context(), types.PluginTypeEntraID, false)
	require.NoError(t, err)
	updatedPluginV1, ok = updatedPlugin.(*types.PluginV1)
	require.True(t, ok, "expected plugin type to be PluginV1")

	gotSettings = updatedPluginV1.Spec.GetEntraId().SyncSettings
	require.Equal(t, &types.PluginEntraIDSyncIntervals{Delta: "1m", Full: "0s"}, gotSettings.SyncIntervals)
}

func createEntraConnector(t *testing.T, s *webSuite) string {
	t.Helper()
	_, err := s.testAuthServer.Auth().CreateRole(t.Context(), services.NewPresetRequesterRole(modules.BuildEnterprise))
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
		"name":                   {types.PluginTypeEntraID},
		"type":                   {types.PluginTypeEntraID},
		"authConnectorName":      {types.PluginTypeEntraID},
		"defaultOwners":          {`["user1", "user2"]`},
		"tenantId":               {"eae3bfbd-3246-47fe-8a37-c20b9eb433a2"},
		"clientId":               {"e1b69fdf-8e18-47f0-864c-94ca7f8d0e1f"},
		"groupFilters":           {`{"id":["c8d8f374-1072-4adf-aa5e-75036e8ccc41"],"nameRegex":[],"excludeId":[],"excludeNameRegex":["finance-*"]}`},
		"accessListOwnersSource": {types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN.String()},
		"syncIntervals":          {`{"full": "1h", "delta": "2m"}`},
	}
}

type entraIDTEnv struct {
	s    *webSuite
	pack *authWebPack
}

func createEntraIDTEnv(t *testing.T) entraIDTEnv {
	t.Helper()
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
				entitlements.SAML:     {Enabled: true},
			},
		},
	}))
	// Set entitlements
	features := s.webPlugin.h.GetClusterFeatures()
	features.Entitlements = map[string]*proto.EntitlementInfo{
		string(entitlements.Identity): {Enabled: true},
	}
	s.webPlugin.h.SetClusterFeatures(features)

	cfg := entraIDPluginDescriptor{testEntityDescriptor: testEntityDescriptor}
	s.webPlugin.pluginDescriptors[types.PluginTypeEntraID] = cfg

	return entraIDTEnv{
		s:    s,
		pack: s.newAuthWebPack(t, "foo"),
	}
}
