package scim

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/generic"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/integration/helpers"
)

func TestSCIMDiscovery(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	scimToken := createGenericSCIMPlugin(t, sut)
	baseURL := url.URL{
		Scheme: "https",
		Host:   sut.ProxyAddr,
		Path:   "/v1/webapi/scim/generic",
	}

	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			Token: scimToken,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	testCases := []struct {
		path string
		want any
	}{
		{
			path: "/ResourceTypes",
			want: map[string]any{
				"itemsPerPage": 2,
				"startIndex":   1,
				"totalResults": 2,
				"schemas": []string{
					"urn:ietf:params:scim:api:messages:2.0:ListResponse",
				},
				"Resources": []map[string]any{{
					"endpoint": "/Users",
					"id":       "User",
					"meta": map[string]any{
						"location":     "/User",
						"resourceType": "ResourceType",
					},
					"name":    "User",
					"schema":  "urn:ietf:params:scim:schemas:core:2.0:User",
					"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
				},
					{
						"endpoint": "/Groups",
						"id":       "Group",
						"meta": map[string]any{
							"location":     "/Group",
							"resourceType": "ResourceType",
						},
						"name":    "Group",
						"schema":  "urn:ietf:params:scim:schemas:core:2.0:Group",
						"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
					},
				},
			},
		},
		{
			path: "/Schemas",
			want: map[string]any{
				"schemas": []string{
					"urn:ietf:params:scim:api:messages:2.0:ListResponse",
				},
				"totalResults": 2,
				"startIndex":   1,
				"itemsPerPage": 2,
				"Resources": []map[string]any{
					{
						"id": "urn:ietf:params:scim:schemas:core:2.0:User",
						"schemas": []string{
							"urn:ietf:params:scim:schemas:core:2.0:User",
						},
						"meta": map[string]any{
							"resourceType": "Schema",
							"location":     "/Schemas/urn:ietf:params:scim:schemas:core:2.0:User",
						},
						"description": "User Schema",
						"name":        "User",
						"attributes":  generic.UserSchema.Attributes,
					},
					{
						"id": "urn:ietf:params:scim:schemas:core:2.0:Group",
						"schemas": []string{
							"urn:ietf:params:scim:schemas:core:2.0:Group",
						},
						"meta": map[string]any{
							"resourceType": "Schema",
							"location":     "/Schemas/urn:ietf:params:scim:schemas:core:2.0:Group",
						},
						"name":        "Group",
						"description": "Group Schema",
						"attributes":  generic.GroupSchema.Attributes,
					},
				},
			},
		},
		{
			path: "/Schemas/urn:ietf:params:scim:schemas:core:2.0:User",
			want: map[string]any{
				"description": "User Schema",
				"id":          "urn:ietf:params:scim:schemas:core:2.0:User",
				"meta": map[string]any{
					"resourceType": "Schema",
					"location":     "/Schemas/urn:ietf:params:scim:schemas:core:2.0:User",
				},
				"name": "User",
				"schemas": []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
				},
				"attributes": generic.UserSchema.Attributes,
			},
		},
		{
			path: "/Schemas/urn:ietf:params:scim:schemas:core:2.0:Group",
			want: map[string]any{
				"description": "Group Schema",
				"id":          "urn:ietf:params:scim:schemas:core:2.0:Group",
				"meta": map[string]any{
					"resourceType": "Schema",
					"location":     "/Schemas/urn:ietf:params:scim:schemas:core:2.0:Group",
				},
				"name": "Group",
				"schemas": []string{
					"urn:ietf:params:scim:schemas:core:2.0:Group",
				},
				"attributes": generic.GroupSchema.Attributes,
			},
		},
		{
			path: "/ServiceProviderConfig",
			want: mergeJSONMaps(
				map[string]any{
					"meta": map[string]any{
						"resourceType": "ServiceProviderConfig",
						"location":     "/ServiceProviderConfig",
					},
					"schemas": []string{
						"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig",
					},
				},
				generic.ServiceProviderConfigAttribute,
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := httpClient.Get(baseURL.String() + tc.path)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NotEmpty(t, body)
			got := unmarshalToMap(t, body)
			assertEqualJSONObjects(t, tc.want, got)
		})
	}
}

func TestSCIMPluginWebHandler(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithResources(createOIDConnector(t, "oidc-connector-for-scim")),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	webClient := sut.CreateWebClientForUser(t, "alice-admin")
	auth := sut.Teleport.Process.GetAuthServer()

	resp, err := doPluginsStaticAuth(t, webClient, "connector-that-does-not-exist", types.KindSAML)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	uiPluginResp := installSCIMPlugin(t, webClient, "okta-pre-created-test", types.KindSAML)
	assertOAuthAccess(t, sut.ProxyAddr, uiPluginResp)
	checkPluginStatus(t, webClient, types.PluginStatusCode_RUNNING)
	require.NoError(t, auth.Plugins.DeleteAllPlugins(t.Context()))

	t.Run("install should fail due the wrong connector type", func(t *testing.T) {
		resp, err := doPluginsStaticAuth(t, webClient, "oidc-connector-for-scim", types.KindSAML)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("install with OIDC connector should succeed", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, auth.Plugins.DeleteAllPlugins(context.Background())) })
		resp, err := doPluginsStaticAuth(t, webClient, "oidc-connector-for-scim", types.KindOIDC)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, auth.Plugins.DeleteAllPlugins(t.Context()))
	})

	t.Run("install missing samlConnectorName", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, auth.Plugins.DeleteAllPlugins(context.Background())) })
		form := url.Values{
			"type":              {types.PluginTypeSCIM},
			"samlConnectorName": {""},
		}
		resp, err := doPluginsStaticAuthWithForm(t, webClient, form)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("connector type should be deduced", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, auth.Plugins.DeleteAllPlugins(context.Background())) })
		form := url.Values{
			"type":          {types.PluginTypeSCIM},
			"connectorName": {"oidc-connector-for-scim"},
			"connectorKind": {""}, // empty connector kind should trigger type deduction
		}
		resp, err := doPluginsStaticAuthWithForm(t, webClient, form)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var uiResp ui.Plugin
		err = json.NewDecoder(resp.Body).Decode(&uiResp)
		require.NoError(t, err)

		gotPlugin, err := auth.Services.GetPlugin(t.Context(), uiResp.Name, false)
		require.NoError(t, err)
		vPlugin, ok := gotPlugin.(*types.PluginV1)
		require.True(t, ok)
		require.Equal(t, types.KindOIDC, vPlugin.Spec.GetScim().ConnectorInfo.Type)
	})

	// TODO(smallinsky) Remove in v18
	t.Run("install legacy form", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, auth.Plugins.DeleteAllPlugins(context.Background())) })
		form := url.Values{
			"type":              {types.PluginTypeSCIM},
			"samlConnectorName": {"okta-pre-created-test"},
		}
		resp, err := doPluginsStaticAuthWithForm(t, webClient, form)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func installSCIMPlugin(t *testing.T, webClient *helpers.WebClientPack, samlConnectorName, connectorKind string) ui.Plugin {
	resp, err := doPluginsStaticAuth(t, webClient, samlConnectorName, connectorKind)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var pluginResp ui.Plugin
	err = json.NewDecoder(resp.Body).Decode(&pluginResp)
	require.NoError(t, err)

	require.NotEmpty(t, pluginResp.Credentials.OAuthCreds.ClientSecret)
	require.NotEmpty(t, pluginResp.Credentials.OAuthCreds.ClientID)
	return pluginResp
}

func doPluginsStaticAuth(t *testing.T, webClient *helpers.WebClientPack, connectorName, connectorKind string) (*http.Response, error) {
	form := url.Values{
		"type":          {types.PluginTypeSCIM},
		"connectorName": {connectorName},
		"connectorKind": {connectorKind},
	}
	return doPluginsStaticAuthWithForm(t, webClient, form)
}

func doPluginsStaticAuthWithForm(t *testing.T, webClient *helpers.WebClientPack, form url.Values) (*http.Response, error) {
	endpoint := webClient.Endpoint("enterprise", "plugins", "staticauth")
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := webClient.Do(req)
	return resp, err
}

func assertOAuthAccess(t *testing.T, proxyAddr string, plugin ui.Plugin) {
	baseURL := buildURL(proxyAddr, "/v1/webapi/scim/"+plugin.Name)
	tokenURL := buildURL(proxyAddr, "/v1/webapi/plugin/"+plugin.Name+"/token")

	config := clientcredentials.Config{
		ClientID:     plugin.Credentials.OAuthCreds.ClientID,
		ClientSecret: plugin.Credentials.OAuthCreds.ClientSecret,
		TokenURL:     tokenURL.String(),
	}

	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: insecureTransport()})
	oauthClient := config.Client(ctx)

	resp := mustHTTPGet(t, oauthClient, baseURL.String()+"/ResourceTypes")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func checkPluginStatus(t *testing.T, webClient *helpers.WebClientPack, wantStatus types.PluginStatusCode) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		req, err := http.NewRequest(http.MethodGet, webClient.Endpoint("enterprise", "plugin"), nil)
		require.NoError(t, err)
		resp, err := webClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var uiResp []ui.Plugin
		err = json.NewDecoder(resp.Body).Decode(&uiResp)
		require.NoError(t, err)
		require.Len(t, uiResp, 1)
		scimPlugin := uiResp[0]
		require.Empty(t, scimPlugin.Credentials)
		require.Equal(t, wantStatus, scimPlugin.Status.Code)
	}, time.Second, time.Millisecond*30)
}

func createOIDConnector(t *testing.T, name string) types.OIDCConnector {
	var oidcSpec = types.OIDCConnectorSpecV3{
		IssuerURL:    "https://issuer",
		ClientID:     "client id",
		ClientSecret: "client secret",
		ClaimsToRoles: []types.ClaimMapping{{
			Claim: "claim",
			Value: "value",
			Roles: []string{"roleA"},
		}},
		RedirectURLs: []string{"https://redirect"},
		MaxAge:       &types.MaxAge{Value: types.Duration(time.Hour)},
	}
	oidc, err := types.NewOIDCConnector(name, oidcSpec)
	require.NoError(t, err)
	return oidc
}
