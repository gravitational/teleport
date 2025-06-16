package scim

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/scim/service/provider/generic"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
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
