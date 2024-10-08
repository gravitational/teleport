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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSAMLSecretsStrip tests the WithoutSecrets method on SAMLConnectorV2.
func TestSAMLSecretsStrip(t *testing.T) {
	connector, err := NewSAMLConnector("test", SAMLConnectorSpecV2{
		AssertionConsumerService: "test",
		SSO:                      "test",
		EntityDescriptor:         "test",
		SigningKeyPair:           &AsymmetricKeyPair{PrivateKey: "test"},
		EncryptionKeyPair:        &AsymmetricKeyPair{PrivateKey: "test"},
	})
	require.NoError(t, err)
	require.Equal(t, "test", connector.GetSigningKeyPair().PrivateKey)
	require.Equal(t, "test", connector.GetEncryptionKeyPair().PrivateKey)

	withoutSecrets := connector.WithoutSecrets().(*SAMLConnectorV2)
	require.Empty(t, withoutSecrets.GetSigningKeyPair().PrivateKey)
	require.Empty(t, withoutSecrets.GetEncryptionKeyPair().PrivateKey)
}

// TestSAMLAcsUriHasConnector tests that the ACS URI has the connector ID as the last part if IdP-initiated login is enabled.
func TestSAMLACSURIHasConnectorName(t *testing.T) {
	connector, err := NewSAMLConnector("myBusinessConnector", SAMLConnectorSpecV2{
		AssertionConsumerService: "https://teleport.local/webapi/v1/saml/acs",
		SSO:                      "test",
		EntityDescriptor:         "test",
		AllowIDPInitiated:        true,
	})

	require.Nil(t, connector)
	require.Error(t, err)

	connector, err = NewSAMLConnector("myBusinessConnector", SAMLConnectorSpecV2{
		AssertionConsumerService: "https://teleport.local/webapi/v1/saml/acs/myBusinessConnector",
		SSO:                      "test",
		EntityDescriptor:         "test",
		AllowIDPInitiated:        true,
	})

	require.NotNil(t, connector)
	require.NoError(t, err)
}
