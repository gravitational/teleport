package scim

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
)

func createGenericSCIMPlugin(t *testing.T, sut *common.SUT) string {
	t.Helper()
	var pluginClient = pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: "generic",
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					SamlConnectorName: "okta-pre-created-test",
				},
			},
		},
	}
	rawToken := uuid.NewString()
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.DefaultCost)
	require.NoError(t, err)

	var req = &pluginsv1.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			buildSCIMCredentials(string(hashedToken)),
		},
	}
	_, err = pluginClient.CreatePlugin(t.Context(), req)
	require.NoError(t, err)
	return rawToken
}

func buildSCIMCredentials(scimToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "scim-generic-token-name" + uuid.NewString(),
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{APIToken: scimToken}},
	}
}

// bearerAuthTransport wraps an existing http.RoundTripper and injects the Authorization header.
type bearerAuthTransport struct {
	Token     string
	Transport http.RoundTripper
}

// RoundTrip adds the Authorization header and delegates to the underlying transport.
func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	clonedReq := req.Clone(req.Context())
	clonedReq.Header.Set("Authorization", "Bearer "+t.Token)
	// Use the provided transport, or fall back to the default
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(clonedReq)
}

// assertEqualJSONObjects asserts that two JSON-like values are semantically equal.
// It normalizes the values by marshaling and unmarshaling them into map form before comparison.
func assertEqualJSONObjects(t *testing.T, expected, actual any) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	actualJSON, err := json.Marshal(actual)
	require.NoError(t, err)

	var expectedMap, actualMap map[string]any
	require.NoError(t, json.Unmarshal(expectedJSON, &expectedMap))
	require.NoError(t, json.Unmarshal(actualJSON, &actualMap))

	require.Equal(t, expectedMap, actualMap)
}

// unmarshalToMap parses a JSON byte slice into a map and fails the test on error.
func unmarshalToMap(t *testing.T, jsonBytes []byte) map[string]any {
	t.Helper()

	var result map[string]any
	err := json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	return result
}

// mergeJSONMaps returns a new map containing keys from both base and overrides.
// If the same key exists, the value from overrides takes precedence.
func mergeJSONMaps(base, overrides map[string]any) map[string]any {
	merged := maps.Clone(base)
	maps.Copy(merged, overrides)
	return merged
}

func createGenericSCIMPluginOauth(t *testing.T, sut *common.SUT) (string, string) {
	t.Helper()
	var pluginClient = pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: "generic",
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					SamlConnectorName: "okta-pre-created-test",
				},
			},
		},
	}
	clientID := uuid.NewString()
	clientSecret := uuid.NewString()

	var req = &pluginsv1.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			buildOauthCreds(clientID, clientSecret),
		},
	}
	_, err := pluginClient.CreatePlugin(t.Context(), req)
	require.NoError(t, err)
	return clientID, clientSecret
}

func buildOauthCreds(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "oauth-generic-token-name" + uuid.NewString(),
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     clientID,
				ClientSecret: clientSecret,
			},
		}},
	}
}

func buildURL(host, path string) url.URL {
	return url.URL{Scheme: "https", Host: host, Path: path}
}

func insecureTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func newBearerClient(token string) *http.Client {
	return &http.Client{
		Transport: &bearerAuthTransport{
			Token:     token,
			Transport: insecureTransport(),
		},
	}
}

func mustHTTPGet(t *testing.T, client *http.Client, url string) *http.Response {
	resp, err := client.Get(url)
	require.NoError(t, err)
	return resp
}

func mustGetToken(t *testing.T, config clientcredentials.Config, ctx context.Context) *oauth2.Token {
	token, err := config.Token(ctx)
	require.NoError(t, err)
	return token
}

func createPluginSCIMClient(t *testing.T, sut *common.SUT, scimToken string, plugin string) scimsdk.Client {
	t.Helper()
	u := url.URL{
		Scheme: "https",
		Path:   "/v1/webapi/scim/" + plugin,
		Host:   sut.ProxyAddr,
	}
	scimClient, err := scimsdk.New(&scimsdk.Config{
		Endpoint:        u.String(),
		Token:           scimToken,
		IntegrationType: "generic",
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	})
	require.NoError(t, err)
	return scimClient
}
