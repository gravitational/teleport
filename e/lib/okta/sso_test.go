package okta

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

const entityDescriptor = `
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

// must is a general-purpose helper that implements the Must* convention on
// any function returning a value and an error. Returns the supplied `val` if
// `err` is non-nil, and panics otherwise.
func must[T any](val T, err error) T {
	if err != nil {
		panic(err.Error())
	}
	return val
}

// mockSamlConnectors is a mocked-out saml connector DB abstraction for testing
type mockSamlConnectors struct {
	mock.Mock
}

func (m *mockSamlConnectors) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	args := m.Called(ctx, connector)
	return connector, args.Error(1)
}

func (m *mockSamlConnectors) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	args := m.Called(ctx, id)
	result, _ := args.Get(0).(types.SAMLConnector)
	return result, args.Error(1)
}

// makeTestGroup constructs a minimal okta.Group instance for use with the
// testOktaClient
func makeTestGroup(id, kind, name string) *okta.Group {
	return &okta.Group{
		Id:      id,
		Type:    kind,
		Profile: &okta.GroupProfile{Name: name},
	}
}

func TestSSOConectorCreation(t *testing.T) {
	ctx := context.Background()
	log := logrus.WithField("test", t.Name())
	log.Logger.SetLevel(logrus.DebugLevel)

	t.Run("happy path", func(t *testing.T) {
		const (
			testTeleportAppId             = "TEST-OKTA-APP-ID"
			everyoneGroupId               = "EVERYONE-GROUP-ID"
			testEntityMetadataURL         = "https://example.com/some/thing/or/other"
			testEntityMetadataContentType = "vegetable/potato"
		)

		// Given a (mocked) cluster with no SAML connector named
		// ${testConnectorName}...
		samlConnectors := &mockSamlConnectors{}
		samlConnectors.
			On("GetSAMLConnector", mock.Anything, testConnectorName).
			Return(nil, trace.NotFound(testConnectorName))
		samlConnectors.
			On("CreateSAMLConnector", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				_, isContext := args.Get(0).(context.Context)
				require.True(t, isContext, "Arg should be a context")

				conn, isConnector := args.Get(1).(types.SAMLConnector)
				require.True(t, isConnector, "Arg should be a SAMLConnector")
				require.Equal(t, testConnectorName, conn.GetName())
			}).
			Return(nil, nil)

		// and a (mock) Okta client configured with some groups (including the
		// special "Everyone" group), and set to allow
		createAppCalled := false
		groupWasAssigned := false
		metadataWasFetched := false
		oktaClient := newTestClient()
		oktaClient.oktaGroups = []*okta.Group{
			makeTestGroup("NOT A BUILTIN", "OKTA_GROUP", "Everyone"),
			makeTestGroup("NOT EVERYONE", "BUILT_IN", "Bananas"),
			makeTestGroup(everyoneGroupId, "BUILT_IN", "Everyone"),
		}
		oktaClient.monkeyPatch.createApp =
			func(ctx context.Context, app okta.App) (okta.App, error) {
				// Expect that createApp was called.]
				createAppCalled = true

				// Expect that `ctx` is a Context and that `app` is a
				// `SAMLApplication` spec
				samlApp, ok := app.(*okta.SamlApplication)
				require.True(t, ok, "Unexpected okta app type: %T", app)
				require.Equal(t, "Teleport "+testClusterName, samlApp.Label)

				// Pretend to be an Okta service and assign the app an ID and
				// fill out its links field.
				samlApp.Id = testTeleportAppId
				samlApp.Links = map[string]any{
					"metadata": map[string]any{
						"href": testEntityMetadataURL,
						"type": testEntityMetadataContentType,
					},
				}
				return samlApp, nil
			}
		oktaClient.monkeyPatch.assignGroupToApplicationByID =
			func(ctx context.Context, groupId, appId string) error {
				require.Equal(t, everyoneGroupId, groupId)
				require.Equal(t, testTeleportAppId, appId)
				groupWasAssigned = true
				return nil
			}
		oktaClient.monkeyPatch.doHttp =
			func(ctx context.Context, method string, url *url.URL, accept []string) ([]byte, error) {
				require.Equal(t, http.MethodGet, method)
				require.Equal(t, testEntityMetadataURL, url.String())
				require.Contains(t, accept, testEntityMetadataContentType)
				metadataWasFetched = true
				return []byte(entityDescriptor), nil
			}

		// When I attempt to create a SAML connector and its corresponding
		// Okta app...
		err := CreateSSOConnector(ctx, ConnectorArgs{
			OktaClient:           oktaClient,
			SAMLConnectorService: samlConnectors,
			ClusterName:          testClusterName,
			ConnectorName:        testConnectorName,
			PublicURL:            must(url.Parse(testClusterURL)),
			Log:                  log,
		})

		// Expect that the operation succeeded, and all of the expected
		// interactions with our mock Okta client happened
		require.NoError(t, err)
		require.True(t, createAppCalled, "CreateApp must be called")
		require.True(t, groupWasAssigned, "Group must be signed to app")
		require.True(t, metadataWasFetched, "SAML metadata must be fetched")
		samlConnectors.AssertExpectations(t)
	})

	t.Run("existing connector prevents creation", func(t *testing.T) {
		// Given a cluster already containing an SAML connector named
		// ${testConnectorName}...
		samlConnectors := &mockSamlConnectors{}
		defer samlConnectors.AssertExpectations(t)
		samlConnectors.
			On("GetSAMLConnector", mock.Anything, testConnectorName).
			Return(types.SAMLConnectorV2{
				Metadata: types.Metadata{Name: testConnectorName},
				Spec:     types.SAMLConnectorSpecV2{},
			}, nil)

		// and an Okta client rigged to fail the test if someone tries to
		// actually create an Okta application
		oktaClient := newTestClient()
		oktaClient.monkeyPatch.createApp = func(context.Context, okta.App) (okta.App, error) {
			t.Fatal("Unexpected call to oktaClient.CreateApp")
			return nil, nil
		}

		// When I attempt to invoke the automated SSO connector creation...
		err := CreateSSOConnector(ctx, ConnectorArgs{
			OktaClient:           oktaClient,
			SAMLConnectorService: samlConnectors,
			ClusterName:          testClusterName,
			ConnectorName:        testConnectorName,
			PublicURL:            must(url.Parse(testClusterURL)),
			Log:                  log,
		})

		// Expect that the operation fails, indicating that an SSO connector
		// with that name already exists
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestExtractMetadataUrl(t *testing.T) {
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
