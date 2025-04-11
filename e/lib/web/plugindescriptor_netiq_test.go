package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	netiqclientmock "github.com/gravitational/teleport/e/lib/netiq/client/mock"
	"github.com/gravitational/teleport/e/lib/web/ui"
)

func TestNetIQPluginInstall(t *testing.T) {
	netiqmock := netiqclientmock.New()
	t.Cleanup(netiqmock.Close)

	testCases := []struct {
		name                    string
		apiURL                  string
		ospURL                  string
		identityVaultUser       string
		identityVaultPassword   string
		oAuthClientID           string
		oAuthClientSecret       string
		insecureSkipVerify      bool
		expectPluginCreationErr bool
	}{
		{
			name:                  "full org URL",
			apiURL:                netiqmock.BaseAPIURL,
			ospURL:                netiqmock.BaseOSPURL,
			identityVaultUser:     netiqmock.IdentityVaultUser,
			identityVaultPassword: netiqmock.IdentityVaultPassword,
			oAuthClientID:         netiqmock.OAuthClientID,
			oAuthClientSecret:     netiqmock.OAuthClientSecret,
			insecureSkipVerify:    true,
		},
		{
			name:                    "missing API URL",
			ospURL:                  netiqmock.BaseOSPURL,
			identityVaultUser:       netiqmock.IdentityVaultUser,
			identityVaultPassword:   netiqmock.IdentityVaultPassword,
			oAuthClientID:           netiqmock.OAuthClientID,
			oAuthClientSecret:       netiqmock.OAuthClientSecret,
			insecureSkipVerify:      true,
			expectPluginCreationErr: true,
		},
		{
			name:                    "missing OSP URL",
			apiURL:                  netiqmock.BaseAPIURL,
			identityVaultUser:       netiqmock.IdentityVaultUser,
			identityVaultPassword:   netiqmock.IdentityVaultPassword,
			oAuthClientID:           netiqmock.OAuthClientID,
			oAuthClientSecret:       netiqmock.OAuthClientSecret,
			insecureSkipVerify:      true,
			expectPluginCreationErr: true,
		},
		{
			name:                    "missing identity vault user",
			apiURL:                  netiqmock.BaseAPIURL,
			ospURL:                  netiqmock.BaseOSPURL,
			identityVaultPassword:   netiqmock.IdentityVaultPassword,
			oAuthClientID:           netiqmock.OAuthClientID,
			oAuthClientSecret:       netiqmock.OAuthClientSecret,
			insecureSkipVerify:      true,
			expectPluginCreationErr: true,
		},
		{
			name:                    "missing identity vault password",
			apiURL:                  netiqmock.BaseAPIURL,
			ospURL:                  netiqmock.BaseOSPURL,
			identityVaultUser:       netiqmock.IdentityVaultUser,
			oAuthClientID:           netiqmock.OAuthClientID,
			oAuthClientSecret:       netiqmock.OAuthClientSecret,
			insecureSkipVerify:      true,
			expectPluginCreationErr: true,
		},
		{
			name:                    "missing OAuth client ID",
			apiURL:                  netiqmock.BaseAPIURL,
			ospURL:                  netiqmock.BaseOSPURL,
			identityVaultUser:       netiqmock.IdentityVaultUser,
			identityVaultPassword:   netiqmock.IdentityVaultPassword,
			oAuthClientSecret:       netiqmock.OAuthClientSecret,
			insecureSkipVerify:      true,
			expectPluginCreationErr: true,
		},
	}

	// Set up a test version of the UI web handler and auth service
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	pluginsSvc := s.authPlugin.PluginsService()
	pluginCredsSvc := s.authPlugin.PluginStaticCredentialsService()

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
				pluginCredsSvc.DeletePluginStaticCredentials(s.ctx, types.PluginTypeNetIQ)
			})

			form := url.Values{
				"type":                  {"netiq"},
				"ospURL":                {testCase.ospURL},
				"apiURL":                {testCase.apiURL},
				"identityVaultUser":     {testCase.identityVaultUser},
				"identityVaultPassword": {testCase.identityVaultPassword},
				"oAuthClientID":         {testCase.oAuthClientID},
				"oAuthClientSecret":     {testCase.oAuthClientSecret},
			}

			if testCase.insecureSkipVerify {
				form["insecure"] = []string{"true"}
			}

			response, err := webPack.clt.PostForm(s.ctx, installPluginEndPoint, form)

			// Expect that both the HTTP round trip and actual request succeeded
			require.NoError(t, err)

			if testCase.expectPluginCreationErr {
				require.NotEqual(t, http.StatusOK, response.Code())
				return
			}

			require.Equal(t, http.StatusOK, response.Code())

			// Expect that the response is a JSON-encoded ui.Plugin with a
			// trailing ui.NetIQPluginSpec{}
			var plugin ui.Plugin
			require.NoError(t, json.Unmarshal(response.Bytes(), &plugin))
			require.Equal(t, "netiq", plugin.Name)
			require.Equal(t, types.PluginType(types.PluginTypeNetIQ), plugin.Type)
			require.Equal(t, types.PluginStatusCode_UNKNOWN, plugin.StatusCode)
			require.Contains(t, plugin.Details, "NetIQ")

			require.IsType(t, &ui.NetIQPluginSpec{}, plugin.Spec)
			spec := plugin.Spec.(*ui.NetIQPluginSpec)
			require.Equal(t, testCase.apiURL, spec.ApiEndpoint)
			require.Equal(t, testCase.ospURL, spec.OauthIssuerEndpoint)
			require.Equal(t, testCase.insecureSkipVerify, spec.InsecureSkipVerify)

			// Expect that the plugin resource was created
			plgI, err := pluginsSvc.GetPlugin(s.ctx, types.PluginTypeNetIQ, false)
			require.NoError(t, err, "failed to load expected plugin")
			plg, ok := plgI.(*types.PluginV1)
			require.True(t, ok, "expected conversion to *types.PluginV1")

			// Expect that the NetIQ plugin settings were set correctly
			netiqSettings := plg.Spec.GetNetIq()
			require.NotNil(t, netiqSettings)
		})
	}
}
