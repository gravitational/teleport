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

	"github.com/google/uuid"
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
						"attributes":  generic.UserAttribute.Attributes,
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
						"attributes":  generic.GroupAttribute.Attributes,
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
				"attributes": generic.UserAttribute.Attributes,
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
				"attributes": generic.GroupAttribute.Attributes,
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
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	webClient := createWebClientForUser(t, sut, "alice-admin")

	uiPluginResp := installSCIMPlugin(t, webClient)
	assertOAuthAccess(t, sut.ProxyAddr, uiPluginResp)
	checkPluginStatus(t, webClient, types.PluginStatusCode_RUNNING)
}

func createWebClientForUser(t *testing.T, sut *common.SUT, user string) *helpers.WebClientPack {
	pass := uuid.NewString()
	require.NoError(t, sut.Teleport.Process.GetAuthServer().UpsertPassword(user, []byte(pass)))
	return helpers.LoginWebClient(t, sut.ProxyAddr, user, pass)
}

func installSCIMPlugin(t *testing.T, webClient *helpers.WebClientPack) ui.Plugin {
	form := url.Values{
		"type":              {types.PluginTypeSCIM},
		"samlConnectorName": {"okta-pre-created-test"},
	}
	endpoint := webClient.Endpoint("enterprise", "plugins", "staticauth")
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := webClient.Do(req)
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
