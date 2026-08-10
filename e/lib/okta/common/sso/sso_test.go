package sso

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	oktaapitest "github.com/gravitational/teleport/e/lib/okta/api/apitest"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// must is a general-purpose helper that implements the Must* convention on
// any function returning a value and an error. Returns the supplied `val` if
// `err` is non-nil, and panics otherwise.
func must[T any](val T, err error) T {
	if err != nil {
		panic(err.Error())
	}
	return val
}

func Test_ValidateSAMLConnector(t *testing.T) {
	t.Parallel()

	type connectorSpecDesc struct {
		sso string
	}
	type connectorDesc struct {
		name   string
		labels map[string]string
		spec   connectorSpecDesc
	}

	testCases := []struct {
		name            string
		connectorDesc   connectorDesc
		oktaClientFuncs oktaapitest.ClientFuncs
		expectErr       error
	}{
		{
			name: "App ID in the label",
			connectorDesc: connectorDesc{
				name: "test-conn",
				labels: map[string]string{
					eteleport.OktaAppIDLabel:  "test-okta-app-id",
					eteleport.OktaOrgURLLabel: "test-okta-org",
					types.OriginLabel:         types.OriginOkta,
				},
				spec: connectorSpecDesc{
					sso: "https://okta.example.com/app/oktaAppName/any_random_stuff/sso/saml",
				},
			},
			oktaClientFuncs: oktaapitest.ClientFuncs{
				OrgURLFunc: func(_ *testing.T) string {
					return "test-okta-org"
				},
				IterateAppsFunc: func(t *testing.T, ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
					require.Equal(t, "oktaAppName", query.NewQueryParams(queryParams...).Q)
					err := fn(&okta.Application{Id: "test-okta-app-id"})
					require.NoError(t, err)
					return nil
				},
			},
			expectErr: nil,
		},
		{
			name: "fallback to SSO URL if App ID label is missing",
			connectorDesc: connectorDesc{
				name: "test-conn",
				labels: map[string]string{
					eteleport.OktaOrgURLLabel: "test-okta-org",
					types.OriginLabel:         types.OriginOkta,
				},
				spec: connectorSpecDesc{
					sso: "https://okta.example.com/app/oktaAppName/any_random_stuff/sso/saml",
				},
			},
			oktaClientFuncs: oktaapitest.ClientFuncs{
				OrgURLFunc: func(_ *testing.T) string {
					return "test-okta-org"
				},
				IterateAppsFunc: func(t *testing.T, ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
					require.Equal(t, "oktaAppName", query.NewQueryParams(queryParams...).Q)
					err := fn(&okta.Application{Id: "found-okta-app-id"})
					require.NoError(t, err)
					return nil
				},
			},
			expectErr: nil,
		},
		{
			name: "fallback to SSO URL if App ID label is missing, but fail if more than one App found",
			connectorDesc: connectorDesc{
				name: "test-conn",
				labels: map[string]string{
					eteleport.OktaOrgURLLabel: "test-okta-org",
					types.OriginLabel:         types.OriginOkta,
				},
				spec: connectorSpecDesc{
					sso: "https://oktaDomain.okta.com/app/oktaDomain_oktaAppName/any_random_stuff/sso/saml",
				},
			},
			oktaClientFuncs: oktaapitest.ClientFuncs{
				OrgURLFunc: func(_ *testing.T) string {
					return "test-okta-org"
				},
				IterateAppsFunc: func(t *testing.T, ctx context.Context, fn func(okta.App) error, queryParams ...query.ParamOptions) error {
					require.Equal(t, "oktaDomain_oktaAppName", query.NewQueryParams(queryParams...).Q)
					err := fn(&okta.Application{Id: "found-okta-app-id-111"})
					require.NoError(t, err)
					err = fn(&okta.Application{Id: "found-okta-app-id-222"})
					require.Error(t, err)
					return err
				},
			},
			expectErr: trace.BadParameter(`this is a bug: more than one Okta App ["found-okta-app-id-222", "found-okta-app-id-111"] found for App name = "oktaDomain_oktaAppName"`),
		},
		{
			name: "missing App ID label and SSO URL to retrieve it from",
			connectorDesc: connectorDesc{
				name: "test-conn",
			},
			expectErr: trace.BadParameter(`SAML connector "test-conn" has not SSO URL set`),
		},
		{
			name: "missing App ID label and has SSO URL with deleted App on Okta side",
			connectorDesc: connectorDesc{
				name: "test-conn",
				spec: connectorSpecDesc{
					sso: "https://oktaDomain.example.com/app/oktaDomain_oktaAppName/any_random_stuff/sso/saml",
				},
			},
			oktaClientFuncs: oktaapitest.ClientFuncs{
				OrgURLFunc: func(_ *testing.T) string {
					return "https://oktaDomain.example.com"
				},
				IterateAppsFunc: func(_ *testing.T, _ context.Context, _ func(okta.App) error, _ ...query.ParamOptions) error {
					// This is equivalent of not found.
					return nil
				},
			},
			expectErr: trace.NotFound(`No Okta App for Okta App name = "oktaDomain_oktaAppName" found. This could be a problem with the app excluded from the Okta resource set.`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			connector := &types.SAMLConnectorV2{
				Metadata: types.Metadata{
					Name:   tc.connectorDesc.name,
					Labels: tc.connectorDesc.labels,
				},
				Spec: types.SAMLConnectorSpecV2{
					SSO: tc.connectorDesc.spec.sso,
				},
			}

			oktaClient := oktaapitest.NewClient(t, tc.oktaClientFuncs)

			_, err := ValidateSAMLConnector(ctx, oktaClient, connector)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestExtractMetadataUrl(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         any
		expectedUrl   *url.URL
		expectedType  string
		expectedError require.ErrorAssertionFunc
	}{
		{
			name: "full",
			input: map[string]any{
				"metadata": map[string]any{
					"href": "https://test.example.com",
					"type": "text/xml",
				},
			},
			expectedError: require.NoError,
			expectedUrl:   must(url.Parse("https://test.example.com")),
			expectedType:  "text/xml",
		}, {
			name: "missing type is not an error",
			input: map[string]any{
				"metadata": map[string]any{
					"href": "https://test.example.com",
				},
			},
			expectedError: require.NoError,
			expectedUrl:   must(url.Parse("https://test.example.com")),
			expectedType:  "",
		}, {
			name: "missing href is an error",
			input: map[string]any{
				"metadata": map[string]any{
					"type": "text/xml",
				},
			},
			expectedError: require.Error,
		}, {
			name:          "badly typed link map is an error",
			input:         "potato",
			expectedError: require.Error,
		}, {
			name: "missing metadata is an error",
			input: map[string]any{
				"logo": map[string]any{
					"href": "https://test.example.com",
					"type": "image/png",
				},
			},
			expectedError: require.Error,
		}, {
			name: "badly typed inner map is an error",
			input: map[string]any{
				"metadata": 7,
			},
			expectedError: require.Error,
		}, {
			name: "malformed url is an error",
			input: map[string]any{
				"logo": map[string]any{
					"href": "I am not a properly forme URL",
					"type": "image/png",
				},
			},
			expectedError: require.Error,
		}, {
			name: "badly typed url is an error",
			input: map[string]any{
				"logo": map[string]any{
					"href": "https://test.example.com",
					"type": 42,
				},
			},
			expectedError: require.Error,
		}, {
			name: "badly typed media type is an error",
			input: map[string]any{
				"logo": map[string]any{
					"href": 7,
					"type": "image/png",
				},
			},
			expectedError: require.Error,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			url, mimeType, err := extractMetadataURL(testCase.input)
			testCase.expectedError(t, err)
			require.Equal(t, testCase.expectedUrl, url)
			require.Equal(t, testCase.expectedType, mimeType)
		})
	}
}

// fakeSAMLConnectorService is a minimal in-memory implementation of SAMLConnectorService for tests.
type fakeSAMLConnectorService struct {
	created types.SAMLConnector
}

func (f *fakeSAMLConnectorService) CreateSAMLConnector(_ context.Context, c types.SAMLConnector) (types.SAMLConnector, error) {
	f.created = c
	return c, nil
}

func (f *fakeSAMLConnectorService) GetSAMLConnector(_ context.Context, _ string, _ bool) (types.SAMLConnector, error) {
	return nil, trace.NotFound("not found")
}

// minimalSAMLMetadata is a bare-minimum SAML IdP metadata XML that satisfies
// the Teleport SAML connector entity-descriptor parser.
const minimalSAMLMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"
    entityID="https://idp.example.okta.com">
  <md:IDPSSODescriptor WantAuthnRequestsSigned="false"
      protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:SingleSignOnService
        Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
        Location="https://idp.example.okta.com/app/myapp/sso/saml"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

func TestCreateSAMLConnectorFromMetadataURL_DisplayName(t *testing.T) {
	t.Parallel()

	// Serve minimal SAML metadata over HTTP so the function can fetch it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(minimalSAMLMetadata))
	}))
	t.Cleanup(srv.Close)

	publicURL := must(url.Parse("https://teleport.example.com"))

	tests := []struct {
		name          string
		connectorName string
		displayName   string
		wantDisplay   string
	}{
		{
			name:          "DisplayName set to Okta uses that as display",
			connectorName: "okta",
			displayName:   "Okta",
			wantDisplay:   "Okta",
		},
		{
			name:          "DisplayName empty falls back to ConnectorName",
			connectorName: "okta",
			displayName:   "",
			wantDisplay:   "okta",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeSAMLConnectorService{}
			args := ConnectorArgs{
				ConnectorName:        tc.connectorName,
				DisplayName:          tc.displayName,
				SAMLConnectorService: svc,
				ClusterName:          "test-cluster",
				PublicURL:            publicURL,
				Logger:               slog.New(slog.DiscardHandler),
				MetadataURL:          srv.URL,
				HTTPClient:           http.DefaultTransport,
			}
			info, err := CreateSAMLConnectorFromMetadataURL(t.Context(), args)
			require.NoError(t, err)
			require.NotNil(t, info)
			require.Equal(t, tc.connectorName, info.Connector.GetName())
			require.Equal(t, tc.wantDisplay, info.Connector.GetDisplay())
		})
	}
}
