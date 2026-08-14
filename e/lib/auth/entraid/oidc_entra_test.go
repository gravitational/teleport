package entraid

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/msgraph"
)

func TestMaybeFetchEntraIDGroups(t *testing.T) {
	mockGroups := []string{
		"45d8d3c5-c802-45c6-b32a-1d70b5e1e86e",
		"843318fb-79a6-4168-9e6f-aa9a07481cc4",
	}
	claimSourceEndpoint := "https://graph.windows.net/0054ef6e-19bb-452f-9094-6953b2137d50/users/3905d6d5-663d-48b1-b28b-9bcb87360949/getMemberObjects"
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf(userGroups, mockGroups[0], mockGroups[1])
		fmt.Fprintln(w, resp)
	}))
	defer ts.Close()

	testClient := ts.Client()
	testClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return net.Dial("tcp", ts.Listener.Addr().String())
		},
	}

	tests := []struct {
		name           string
		connector      types.OIDCConnector
		claim          map[string]any
		entraConfig    *types.EntraIDGroupsProvider
		expectedGroups []string
		httpClient     *http.Client
		errAssertion   require.ErrorAssertionFunc
	}{
		{
			name: "valid claim source",
			claim: map[string]any{
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949",
				"tid": "0054ef6e-19bb-452f-9094-6953b2137d50",
			},
			expectedGroups: mockGroups,
			httpClient:     testClient,
			errAssertion:   require.NoError,
		},
		{
			name: "with groups claim",
			claim: map[string]any{
				"groups": mockGroups,
				"oid":    "3905d6d5-663d-48b1-b28b-9bcb87360949",
			},
			expectedGroups: mockGroups,
			errAssertion:   require.NoError,
		},
		{
			name:         "empty groups claim and groups source claim should not trigger error",
			claim:        map[string]any{"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949"},
			httpClient:   http.DefaultClient, /* custom http client causes graph client to return error */
			errAssertion: require.NoError,
		},
		{
			name: "faulty groups provider should not override valid groups claim",
			claim: map[string]any{
				"groups": mockGroups,
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949",
				"tid": "0054ef6e-19bb-452f-9094-6953b2137d50",
			},
			expectedGroups: mockGroups,
			httpClient:     http.DefaultClient, /* custom http client causes graph client to return error */
			errAssertion:   require.NoError,
		},
		{
			name: "disabled groups provider, valid groups source claim",
			claim: map[string]any{
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949",
				"tid": "0054ef6e-19bb-452f-9094-6953b2137d50",
			},
			entraConfig: &types.EntraIDGroupsProvider{
				Disabled: true,
			},
			expectedGroups: nil,
			errAssertion:   require.NoError,
		},
		{
			name: "claim missing oid",
			claim: map[string]any{
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"tid": "0054ef6e-19bb-452f-9094-6953b2137d50",
			},
			errAssertion: require.Error,
		},
		{
			name: "claim missing tid",
			claim: map[string]any{
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949",
			},
			errAssertion: require.Error,
		},
		{
			name: "valid group source claim with error from graph client",
			claim: map[string]any{
				"_claim_names": map[string]string{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": claimSourceEndpoint,
					},
				},
				"oid": "3905d6d5-663d-48b1-b28b-9bcb87360949",
				"tid": "0054ef6e-19bb-452f-9094-6953b2137d50",
			},
			httpClient:   http.DefaultClient, /* custom http client causes graph client to return error */
			errAssertion: require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := &oidc.IDTokenClaims{
				Claims: test.claim,
			}
			idToken := &oidc.Tokens[*oidc.IDTokenClaims]{IDTokenClaims: claims}
			connector := newOIDCConnector(t, test.entraConfig)
			graphClient, err := msgraph.NewClient(msgraph.Config{
				TokenProvider: &fakeTokenProvider{},
				HTTPClient:    test.httpClient,
			})
			require.NoError(t, err)
			provider := OIDCEntraIDGroupsProvider{
				Connector:  connector,
				IDToken:    idToken,
				Logger:     slog.Default().With("test", test.name),
				HTTPClient: test.httpClient,
			}
			err = provider.MaybeFetchEntraIDGroups(t.Context(), graphClient)
			test.errAssertion(t, err)
			require.ElementsMatch(t, test.expectedGroups, claims.Claims["groups"])
		})
	}
}

type fakeTokenProvider struct{}

func (t *fakeTokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: "foo",
	}, nil
}

func newOIDCConnector(t *testing.T, entra *types.EntraIDGroupsProvider) *types.OIDCConnectorV3 {
	t.Helper()
	url := "https://login.microsoftonline.com/c1acbc61-75c6-4eb9-a8aa-b5b4e5c8ab79/v2.0"
	connector := &types.OIDCConnectorV3{
		Kind:    types.KindOIDCConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name: "entra-id",
		},
		Spec: types.OIDCConnectorSpecV3{
			IssuerURL:    url,
			ClientID:     "test",
			ClientSecret: "secret",
			ClaimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "*",
					Roles: []string{"access"},
				},
			},
			RedirectURLs: wrappers.Strings{
				url + "/proxy/oidc/callback",
			},
			EntraIdGroupsProvider: entra,
		},
	}
	require.NoError(t, connector.CheckAndSetDefaults())
	return connector
}

var userGroups = `
{
  "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#directoryObjects",
  "@odata.nextLink": "",
  "value": [
    {
      "id": "%s"
    },
    {
      "id": "%s"
    }
  ]
}
`
