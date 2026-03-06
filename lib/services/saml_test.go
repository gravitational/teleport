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
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
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
			connector, err := types.NewSAMLConnector("test_connector", types.SAMLConnectorSpecV2{
				AssertionConsumerService: "http://localhost:65535/acs", // not called
				Issuer:                   "test",
				SSO:                      "https://localhost:65535/sso", // not called
				AttributesToRoles: []types.AttributeMapping{
					// not used. can be any name, value but role must exist
					{Name: "groups", Value: "admin", Roles: tc.roles},
				},
			})
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

			err := FillSAMLSigningKeyFromExisting(ctx, connector, existingConnectors)
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

func TestCheckSAMLSigningKeyExpiry(t *testing.T) {
	testCases := []struct {
		name         string
		timeframe    time.Duration
		ttls         []time.Duration
		assertResult bool
		assertErr    require.ErrorAssertionFunc
	}{
		{
			name:         "cert not expiring within timeframe",
			timeframe:    30 * 24 * time.Hour,
			ttls:         []time.Duration{45 * 24 * time.Hour},
			assertResult: false,
			assertErr:    require.NoError,
		},
		{
			name:         "cert expiring within timeframe",
			timeframe:    30 * 24 * time.Hour,
			ttls:         []time.Duration{15 * 24 * time.Hour},
			assertResult: true,
			assertErr:    require.NoError,
		},
		{
			name:         "cert already expired",
			timeframe:    30 * 24 * time.Hour,
			ttls:         []time.Duration{-24 * time.Hour},
			assertResult: true,
			assertErr:    require.NoError,
		},
		{
			name:      "multiple certs with one expiring",
			timeframe: 30 * 24 * time.Hour,
			ttls: []time.Duration{
				45 * 24 * time.Hour,
				15 * 24 * time.Hour,
				60 * 24 * time.Hour,
			},
			assertResult: true,
			assertErr:    require.NoError,
		},
		{
			name:         "empty entity descriptor",
			timeframe:    30 * 24 * time.Hour,
			ttls:         []time.Duration{},
			assertResult: false,
			assertErr:    require.NoError,
		},
		{
			name:         "invalid entity descriptor",
			timeframe:    30 * 24 * time.Hour,
			ttls:         nil, // Absence of ttls is used to generate an invalid entity descriptor.
			assertResult: false,
			assertErr:    require.Error,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var entityDescriptor string
			// Absence of ttls signals to generate an invalid entity descriptor.
			if tc.ttls != nil {
				entityDescriptor = createTestEntityDescriptor(t, tc.ttls)
			} else {
				entityDescriptor = "invalid entity descriptor"
			}

			connector, err := types.NewSAMLConnector("test-connector", types.SAMLConnectorSpecV2{
				AssertionConsumerService: "http://localhost:65535/acs", // Not called.
				EntityDescriptor:         entityDescriptor,
			})
			require.NoError(t, err)

			result, err := CheckSAMLSigningKeyExpiry(connector, tc.timeframe)
			assert.Equal(t, tc.assertResult, result)
			tc.assertErr(t, err)
		})
	}
}

func createTestEntityDescriptor(t *testing.T, ttls []time.Duration) string {
	t.Helper()

	var certs []string

	for _, ttl := range ttls {
		_, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{}, nil, ttl)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		certs = append(certs, fmt.Sprintf(
			`<md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>`,
			base64.StdEncoding.EncodeToString(block.Bytes),
		))
	}

	return fmt.Sprintf(
		`<?xml version="1.0"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test"><md:IDPSSODescriptor>%s</md:IDPSSODescriptor></md:EntityDescriptor>`,
		strings.Join(certs, ""),
	)
}
