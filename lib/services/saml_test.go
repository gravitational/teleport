/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/listener"
)

func TestParseFromMetadata(t *testing.T) {
	t.Parallel()

	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	require.Equal(t, "http://www.okta.com/exkafftca6RqPVgyZ0h7", oc.GetIssuer())
	require.Equal(t, "https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml", oc.GetSSO())
	require.Equal(t, "https://localhost:3080/v1/webapi/saml/acs", oc.GetAssertionConsumerService())
	require.Equal(t, "https://localhost:3080/v1/webapi/saml/acs", oc.GetAudience())
	require.NotNil(t, oc.GetSigningKeyPair())
	require.Equal(t, []string{"groups"}, oc.GetAttributes())
}

func TestCheckSAMLEntityDescriptor(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		resource  string
		wantCerts int
	}{
		"without certificate padding": {resource: fixtures.SAMLOktaConnectorV2, wantCerts: 1},
		"with certificate padding":    {resource: fixtures.SAMLOktaConnectorV2WithPadding, wantCerts: 1},
		"missing IDPSSODescriptor":    {resource: fixtures.SAMLConnectorMissingIDPSSODescriptor, wantCerts: 0},
	} {
		t.Run(name, func(t *testing.T) {
			decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(tt.resource), defaults.LookaheadBufSize)
			var raw UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)

			oc, err := UnmarshalSAMLConnector(raw.Raw)
			require.NoError(t, err)

			ed := oc.GetEntityDescriptor()
			certs, err := CheckSAMLEntityDescriptor(ed)
			require.NoError(t, err)
			require.Len(t, certs, tt.wantCerts)
		})
	}
}

func TestValidateRoles(t *testing.T) {
	t.Parallel()

	// Create a roleSet with <nil> role values as ValidateSAMLRole does
	// not care what the role value is, just that it exists and that
	// the RoleGetter does not return an error.
	var validRoles roleSet = map[string]types.Role{
		"foo":  nil,
		"bar":  nil,
		"big$": nil,
	}

	testCases := []struct {
		desc        string
		roles       []string
		expectedErr error
	}{
		{
			desc:  "valid roles",
			roles: []string{"foo", "bar"},
		},
		{
			desc:  "role templates",
			roles: []string{"foo", "$1", "$baz", "admin_${baz}"},
		},
		{
			desc:        "missing role",
			roles:       []string{"baz"},
			expectedErr: trace.BadParameter(`role "baz" specified in attributes_to_roles not found`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			connector, err := types.NewSAMLConnector(
				"test_connector",
				types.SAMLConnectorSpecV2{
					AssertionConsumerService: "http://localhost:65535/acs", // not called
					Issuer:                   "test",
					SSO:                      "https://localhost:65535/sso", // not called
					AttributesToRoles: []types.AttributeMapping{
						// not used. can be any name, value but role must exist
						{Name: "groups", Value: "admin", Roles: tc.roles},
					},
				},
			)
			require.NoError(t, err)

			err = ValidateSAMLConnector(connector, validRoles)
			require.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

type mockSAMLGetter map[string]types.SAMLConnector

func (m mockSAMLGetter) GetSAMLConnector(_ context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	connector, ok := m[id]
	if !ok {
		return nil, trace.NotFound("%s not found", id)
	}
	return connector, nil
}

func (m mockSAMLGetter) GetSAMLConnectorWithValidationOptions(ctx context.Context, id string, withSecrets bool, _ ...types.SAMLConnectorValidationOption) (types.SAMLConnector, error) {
	return m.GetSAMLConnector(ctx, id, withSecrets)
}

func TestFillSAMLSigningKeyFromExisting(t *testing.T) {
	t.Parallel()

	// Test setup: generate the fixtures
	existingKeyPEM, existingCertPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport OSS"},
		CommonName:   "teleport.localhost.localdomain",
	}, nil, 10*365*24*time.Hour)
	require.NoError(t, err)

	existingSkp := &types.AsymmetricKeyPair{
		PrivateKey: string(existingKeyPEM),
		Cert:       string(existingCertPEM),
	}

	existingConnectorName := "existing"
	existingConnectors := mockSAMLGetter{
		existingConnectorName: &types.SAMLConnectorV2{
			Spec: types.SAMLConnectorSpecV2{
				SigningKeyPair: existingSkp,
			},
		},
	}

	_, unrelatedCertPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport OSS"},
		CommonName:   "teleport.localhost.localdomain",
	}, nil, 10*365*24*time.Hour)
	require.NoError(t, err)

	// Test setup: define test cases
	testCases := []struct {
		name          string
		connectorName string
		connectorSpec types.SAMLConnectorSpecV2
		assertErr     require.ErrorAssertionFunc
		assertResult  require.ValueAssertionFunc
	}{
		{
			name:          "should read signing key from existing connector with matching cert",
			connectorName: existingConnectorName,
			connectorSpec: types.SAMLConnectorSpecV2{
				SigningKeyPair: &types.AsymmetricKeyPair{
					PrivateKey: "",
					Cert:       string(existingCertPEM),
				},
			},
			assertErr: require.NoError,
			assertResult: func(t require.TestingT, value any, args ...any) {
				require.Implements(t, (*types.SAMLConnector)(nil), value)
				connector := value.(types.SAMLConnector)
				skp := connector.GetSigningKeyPair()
				require.Equal(t, existingSkp, skp)
			},
		},
		{
			name:          "should error when there's no existing connector",
			connectorName: "non-existing",
			connectorSpec: types.SAMLConnectorSpecV2{
				SigningKeyPair: &types.AsymmetricKeyPair{
					PrivateKey: "",
					Cert:       string(unrelatedCertPEM),
				},
			},
			assertErr: require.Error,
		},
		{
			name:          "should error when existing connector cert is not matching",
			connectorName: existingConnectorName,
			connectorSpec: types.SAMLConnectorSpecV2{
				SigningKeyPair: &types.AsymmetricKeyPair{
					PrivateKey: "",
					Cert:       string(unrelatedCertPEM),
				},
			},
			assertErr: require.Error,
		},
	}

	// Test execution
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			connector := &types.SAMLConnectorV2{
				Metadata: types.Metadata{
					Name: tc.connectorName,
				},
				Spec: tc.connectorSpec,
			}

			err := FillSAMLSecretFieldsFromExistingConnector(ctx, connector, existingConnectors)
			tc.assertErr(t, err)
			if tc.assertResult != nil {
				tc.assertResult(t, connector)
			}
		})
	}
}

func TestFillSAMLOAuthClientSecretFromExisting(t *testing.T) {
	t.Parallel()

	existingOAuthCreds := &types.OAuthClientCredentials{
		ClientId:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	existingConnectorName := "existing"
	existingConnectorNoCredsName := "existing-no-creds"
	existingConnectors := mockSAMLGetter{
		existingConnectorName: &types.SAMLConnectorV2{
			Spec: types.SAMLConnectorSpecV2{
				Credentials: &types.SAMLConnectorCredentials{Oauth: existingOAuthCreds},
			},
		},
		existingConnectorNoCredsName: &types.SAMLConnectorV2{
			Spec: types.SAMLConnectorSpecV2{},
		},
	}

	testCases := []struct {
		name          string
		connectorName string
		connectorSpec types.SAMLConnectorSpecV2
		assertErr     require.ErrorAssertionFunc
		assertResult  require.ValueAssertionFunc
	}{
		{
			name:          "existing connector with matching ClientId",
			connectorName: existingConnectorName,
			connectorSpec: types.SAMLConnectorSpecV2{
				Credentials: &types.SAMLConnectorCredentials{Oauth: &types.OAuthClientCredentials{
					ClientId:     "test-client-id",
					ClientSecret: "",
				}},
			},
			assertErr: require.NoError,
			assertResult: func(t require.TestingT, value any, args ...any) {
				require.Implements(t, (*types.SAMLConnector)(nil), value)
				connector := value.(types.SAMLConnector)
				oauthCreds := connector.GetOAuthClientCredentials()
				require.Equal(t, existingOAuthCreds, oauthCreds)
			},
		},
		{
			name:          "connector not found",
			connectorName: "non-existing",
			connectorSpec: types.SAMLConnectorSpecV2{
				Credentials: &types.SAMLConnectorCredentials{Oauth: &types.OAuthClientCredentials{
					ClientId:     "test-client-id",
					ClientSecret: "",
				}},
			},
			assertErr: require.Error,
		},
		{
			name:          "incoming connector has no OAuth credentials",
			connectorName: existingConnectorName,
			connectorSpec: types.SAMLConnectorSpecV2{},
			assertErr:     require.NoError,
		},
		{
			name:          "existing connector has no OAuth credentials",
			connectorName: existingConnectorNoCredsName,
			connectorSpec: types.SAMLConnectorSpecV2{
				Credentials: &types.SAMLConnectorCredentials{Oauth: &types.OAuthClientCredentials{
					ClientId:     "another-client-id",
					ClientSecret: "",
				}},
			},
			assertErr: require.Error,
		},
		{
			name:          "existing connector OAuth ClientId doesn't match",
			connectorName: existingConnectorName,
			connectorSpec: types.SAMLConnectorSpecV2{
				Credentials: &types.SAMLConnectorCredentials{Oauth: &types.OAuthClientCredentials{
					ClientId:     "another-client-id",
					ClientSecret: "",
				}},
			},
			assertErr: require.Error,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			connector := &types.SAMLConnectorV2{
				Metadata: types.Metadata{Name: tc.connectorName},
				Spec:     tc.connectorSpec,
			}

			err := FillSAMLSecretFieldsFromExistingConnector(ctx, connector, existingConnectors)
			tc.assertErr(t, err)
			if tc.assertResult != nil {
				tc.assertResult(t, connector)
			}
		})
	}
}

// roleSet is a basic set of roles keyed by role name. It implements the
// RoleGetter interface, returning the role if it exists, or a trace.NotFound
// error if it does not exist.
type roleSet map[string]types.Role

func (rs roleSet) GetRole(ctx context.Context, name string) (types.Role, error) {
	if r, ok := rs[name]; ok {
		return r, nil
	}
	return nil, trace.NotFound("unknown role: %s", name)
}

func Test_ValidateSAMLConnector(t *testing.T) {
	t.Parallel()

	const entityDescriptor = `
		<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2024-07-04T11:40:17.481Z" cacheDuration="PT48H" entityID="teleport.example.com/metadata">
			<IDPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
				<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://first.in.the.descriptor.example.com/sso/saml"></SingleSignOnService>
				<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://second.in.the.descriptor.example.com/sso/saml"></SingleSignOnService>
			</IDPSSODescriptor>
		</EntityDescriptor>`

	// Create a roleSet with <nil> role values as ValidateSAMLConnector only checks if the role
	// in the connector role mapping exists.
	var existingRoles roleSet = map[string]types.Role{
		"existing-test-role": nil,
	}

	testCases := []struct {
		name           string
		connectorDesc  types.SAMLConnectorSpecV2
		expectedSsoUrl string
	}{
		{
			name: "matching set SSO and entity descriptor URLs",
			connectorDesc: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://example.com/acs",
				EntityDescriptor:         entityDescriptor,
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "Everyone", Roles: []string{"existing-test-role"}},
				},
			},
			expectedSsoUrl: "https://first.in.the.descriptor.example.com/sso/saml",
		},
		{
			name: "if set SSO URL and entity descriptor do not match, overwrite with the one from the descriptor",
			connectorDesc: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://example.com/acs",
				SSO:                      "https://i.do.not.match.example.com/sso",
				EntityDescriptor:         entityDescriptor,
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "Everyone", Roles: []string{"existing-test-role"}},
				},
			},
			expectedSsoUrl: "https://first.in.the.descriptor.example.com/sso/saml",
		},
	}
	for _, tc := range testCases {
		connector, err := types.NewSAMLConnector("test-connector", tc.connectorDesc)
		require.NoError(t, err)

		err = ValidateSAMLConnector(connector, existingRoles)
		require.NoError(t, err)

		require.Equal(t, tc.expectedSsoUrl, connector.GetSSO())
	}
}

func Test_ValidateSAMLConnector_populatesLoginAndMFASettingsFromEntityDescriptor(t *testing.T) {
	t.Parallel()

	const role = "test_role"
	const loginEntityDescriptor = `
		<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://login.idp.test/metadata">
			<IDPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
				<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://login.idp.test/sso/saml"></SingleSignOnService>
			</IDPSSODescriptor>
		</EntityDescriptor>`
	const mfaEntityDescriptor = `
		<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://mfa.idp.test/metadata">
			<IDPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
				<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://mfa.idp.test/sso/saml"></SingleSignOnService>
			</IDPSSODescriptor>
		</EntityDescriptor>`

	connector, err := types.NewSAMLConnector("test-connector", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://idp.test/acs",
		EntityDescriptor:         loginEntityDescriptor,
		AttributesToRoles: []types.AttributeMapping{
			{Name: "groups", Value: "Everyone", Roles: []string{role}},
		},
		MFASettings: &types.SAMLConnectorMFASettings{
			Enabled:          true,
			EntityDescriptor: mfaEntityDescriptor,
		},
	})
	require.NoError(t, err)

	require.Empty(t, connector.GetIssuer())
	require.Empty(t, connector.GetSSO())
	require.Empty(t, connector.GetMFASettings().Issuer)
	require.Empty(t, connector.GetMFASettings().Sso)

	// Create a roleSet with <nil> role values as ValidateSAMLConnector only checks if the role
	// in the connector role mapping exists.
	err = ValidateSAMLConnector(connector, roleSet{role: nil})
	require.NoError(t, err)

	require.Equal(t, "https://login.idp.test/metadata", connector.GetIssuer())
	require.Equal(t, "https://login.idp.test/sso/saml", connector.GetSSO())
	require.Equal(t, "https://mfa.idp.test/metadata", connector.GetMFASettings().Issuer)
	require.Equal(t, "https://mfa.idp.test/sso/saml", connector.GetMFASettings().Sso)
}

func Test_ValidateSAMLConnector_error_sanitization(t *testing.T) {
	t.Parallel()

	const role = "existing-test-role"

	for _, tc := range []struct {
		name                   string
		spec                   types.SAMLConnectorSpecV2
		setEntityDescriptorURL func(types.SAMLConnector, string)
		getEntityDescriptor    func(types.SAMLConnector) string
	}{
		{
			name: "root entity descriptor URL",
			spec: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://teleport.example.com/v1/webapi/saml/acs",
				EntityDescriptorURL:      "https://to-be-changed.test",
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "admin", Roles: []string{role}},
				},
			},
			setEntityDescriptorURL: func(connector types.SAMLConnector, url string) {
				connector.SetEntityDescriptorURL(url)
			},
			getEntityDescriptor: func(connector types.SAMLConnector) string {
				return connector.GetEntityDescriptor()
			},
		},
		{
			name: "MFA entity descriptor URL",
			spec: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://teleport.example.com/v1/webapi/saml/acs",
				Issuer:                   "https://idp.example.com",
				SSO:                      "https://idp.example.com/sso",
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "admin", Roles: []string{role}},
				},
				MFASettings: &types.SAMLConnectorMFASettings{
					Enabled:             true,
					EntityDescriptorUrl: "https://to-be-changed.test",
				},
			},
			setEntityDescriptorURL: func(connector types.SAMLConnector, url string) {
				connector.GetMFASettings().EntityDescriptorUrl = url
			},
			getEntityDescriptor: func(connector types.SAMLConnector) string {
				return connector.GetMFASettings().EntityDescriptor
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				const happyPath = "/happy"
				const slowPath = "/slow"
				const entityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
					<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
						<IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
							<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"></SingleSignOnService>
						</IDPSSODescriptor>
					</EntityDescriptor>`

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == happyPath {
						w.Write([]byte(entityDescriptor))
						return
					}
					if r.URL.Path == slowPath {
						time.Sleep(1 * time.Hour)
					}
					http.Error(w, "upstream response should not be exposed", http.StatusForbidden)
				})

				listener := listener.NewInMemoryListener()
				server := httptest.NewUnstartedServer(handler)
				server.Listener = listener
				server.Start()
				t.Cleanup(server.Close)

				samlConnector, err := types.NewSAMLConnector("fetch-entity-descriptor-url-errors", tc.spec)
				require.NoError(t, err)

				testOpts := []types.SAMLConnectorValidationOption{
					types.SAMLConnectorValidationHTTPTransport(listener.MakeHTTPClient().Transport),
				}

				// Create a roleSet with <nil> role values as ValidateSAMLConnector only checks if the role
				// in the connector role mapping exists.
				var roleGetter = roleSet{role: nil}

				// There are quite a few things to leak in the error message:
				// 1. The HTTP response content.
				// 2. The HTTP response code.
				// 3. The descriptor URL and the path.
				tc.setEntityDescriptorURL(samlConnector, server.URL+"/test_metadata")

				err = ValidateSAMLConnector(samlConnector, roleGetter, testOpts...)
				require.ErrorIs(t, err, ErrFailedToFetchEntityDescriptor)
				// Make sure the error doesn't leak any extra info about the download failure.
				require.Equal(t, ErrFailedToFetchEntityDescriptor.Error(), err.Error())

				// Verify that it also doesn't leak any message specific to timeout.
				tc.setEntityDescriptorURL(samlConnector, server.URL+slowPath)

				errCh := make(chan error, 1)
				go func() {
					err := ValidateSAMLConnector(samlConnector, roleGetter, testOpts...)
					select {
					case errCh <- err:
					case <-ctx.Done():
					}
				}()

				time.Sleep(apidefaults.DefaultIOTimeout - 1*time.Nanosecond)
				synctest.Wait()
				select {
				case err := <-errCh:
					t.Fatalf("expected validation to be blocking on fetching the entity descriptor, but it returned error: %v", err)
				default:
				}

				time.Sleep(1 * time.Nanosecond)
				synctest.Wait()
				select {
				case err := <-errCh:
					require.ErrorIs(t, err, ErrFailedToFetchEntityDescriptor)
					// Make sure the error doesn't leak any extra info about the timeout.
					require.Equal(t, ErrFailedToFetchEntityDescriptor.Error(), err.Error())
				default:
					t.Fatal("expected ValidateSAMLConnector to already timeout")
				}

				// To unblock the slow entity server.
				time.Sleep(2 * time.Hour)
				synctest.Wait()

				// Verify the happy path.
				tc.setEntityDescriptorURL(samlConnector, server.URL+happyPath)
				err = ValidateSAMLConnector(samlConnector, roleGetter, testOpts...)
				require.NoError(t, err)
				require.Equal(t, entityDescriptor, tc.getEntityDescriptor(samlConnector))
			})
		})
	}
}

func Test_ValidateSAMLConnector_blocksHTTPSRedirectDowngrade(t *testing.T) {
	t.Parallel()

	const role = "existing-test-role"

	var downgradedRequestsCnt atomic.Int64
	downgradedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downgradedRequestsCnt.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(downgradedServer.Close)

	redirectingServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, downgradedServer.URL, http.StatusFound)
	}))
	t.Cleanup(redirectingServer.Close)

	for _, tc := range []struct {
		name string
		spec types.SAMLConnectorSpecV2
	}{
		{
			name: "root entity descriptor URL",
			spec: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://teleport.example.com/v1/webapi/saml/acs",
				EntityDescriptorURL:      redirectingServer.URL,
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "admin", Roles: []string{role}},
				},
			},
		},
		{
			name: "MFA entity descriptor URL",
			spec: types.SAMLConnectorSpecV2{
				AssertionConsumerService: "https://teleport.example.com/v1/webapi/saml/acs",
				Issuer:                   "https://idp.example.com",
				SSO:                      "https://idp.example.com/sso",
				AttributesToRoles: []types.AttributeMapping{
					{Name: "groups", Value: "admin", Roles: []string{role}},
				},
				MFASettings: &types.SAMLConnectorMFASettings{
					Enabled:             true,
					EntityDescriptorUrl: redirectingServer.URL,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			samlConnector, err := types.NewSAMLConnector("redirect-downgrade", tc.spec)
			require.NoError(t, err)

			err = ValidateSAMLConnector(samlConnector, roleSet{role: nil},
				types.SAMLConnectorValidationHTTPTransport(redirectingServer.Client().Transport),
			)
			require.ErrorIs(t, err, ErrFailedToFetchEntityDescriptor)
			// Make sure there is no any extra information in the error.
			require.Equal(t, ErrFailedToFetchEntityDescriptor.Error(), err.Error())
			require.Zero(t, downgradedRequestsCnt.Load())
		})
	}
}
