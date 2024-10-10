/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestSAMLSecretsStrip tests the WithoutSecrets method on SAMLConnectorV2.
func TestSAMLSecretsStrip(t *testing.T) {
	connector, err := types.NewSAMLConnector("test", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "test",
		SSO:                      "test",
		EntityDescriptor:         "test",
		SigningKeyPair:           &types.AsymmetricKeyPair{PrivateKey: "test"},
		EncryptionKeyPair:        &types.AsymmetricKeyPair{PrivateKey: "test"},
	})
	require.NoError(t, err)
	require.Equal(t, "test", connector.GetSigningKeyPair().PrivateKey)
	require.Equal(t, "test", connector.GetEncryptionKeyPair().PrivateKey)

	withoutSecrets := connector.WithoutSecrets().(*types.SAMLConnectorV2)
	require.Empty(t, withoutSecrets.GetSigningKeyPair().PrivateKey)
	require.Empty(t, withoutSecrets.GetEncryptionKeyPair().PrivateKey)
}

// TestSAMLAcsUriHasConnector tests that the ACS URI has the connector ID as the last part if IdP-initiated login is enabled.
func TestSAMLACSURIHasConnectorName(t *testing.T) {
	connector, err := types.NewSAMLConnector("myBusinessConnector", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://teleport.local/webapi/v1/saml/acs",
		SSO:                      "test",
		EntityDescriptor:         "test",
		AllowIDPInitiated:        true,
	})

	require.Nil(t, connector)
	require.Error(t, err)

	connector, err = types.NewSAMLConnector("myBusinessConnector", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://teleport.local/webapi/v1/saml/acs/myBusinessConnector",
		SSO:                      "test",
		EntityDescriptor:         "test",
		AllowIDPInitiated:        true,
	})

	require.NotNil(t, connector)
	require.NoError(t, err)
}

func TestSAMLForceAuthn(t *testing.T) {
	for _, tt := range []struct {
		name       string
		forceAuthn types.SAMLForceAuthn
		expectBase bool
		expectMFA  bool
	}{
		{
			name:       "force_authn unspecified",
			forceAuthn: types.SAMLForceAuthn_FORCE_AUTHN_UNSPECIFIED,
			expectBase: false,
			expectMFA:  true,
		}, {
			name:       "force_authn yes",
			forceAuthn: types.SAMLForceAuthn_FORCE_AUTHN_YES,
			expectBase: true,
			expectMFA:  true,
		}, {
			name:       "force_authn no",
			forceAuthn: types.SAMLForceAuthn_FORCE_AUTHN_NO,
			expectBase: false,
			expectMFA:  false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			samlConnector := types.SAMLConnectorV2{
				Spec: types.SAMLConnectorSpecV2{
					ForceAuthn: tt.forceAuthn,
					MFASettings: &types.SAMLConnectorMFASettings{
						Enabled:    true,
						ForceAuthn: tt.forceAuthn,
					},
				},
			}

			require.Equal(t, tt.expectBase, samlConnector.GetForceAuthn(), "expected force_authn to be %v but got %v", tt.expectBase, samlConnector.GetForceAuthn())

			require.NoError(t, samlConnector.WithMFASettings())
			require.Equal(t, tt.expectMFA, samlConnector.GetForceAuthn(), "expected force_authn to be %v for mfa but got %v", tt.expectMFA, samlConnector.GetForceAuthn())
		})
	}
}
