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
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
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

	for name, input := range map[string]string{
		"without certificate padding": fixtures.SAMLOktaConnectorV2,
		"with certificate padding":    fixtures.SAMLOktaConnectorV2WithPadding,
	} {
		t.Run(name, func(t *testing.T) {
			decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
			var raw UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)

			oc, err := UnmarshalSAMLConnector(raw.Raw)
			require.NoError(t, err)

			ed := oc.GetEntityDescriptor()
			certs, err := CheckSAMLEntityDescriptor(ed)
			require.NoError(t, err)
			require.Len(t, certs, 1)
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
			assertResult: func(t require.TestingT, value interface{}, args ...interface{}) {
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
