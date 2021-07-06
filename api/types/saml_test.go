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
		EntityDescriptor:         "",
		SigningKeyPair:           &AsymmetricKeyPair{},
		EncryptionKeyPair:        &AsymmetricKeyPair{},
	})
	require.Nil(t, err)
	require.NotNil(t, connector.GetSigningKeyPair())
	require.NotNil(t, connector.GetEncryptionKeyPair())

	withoutSecrets := connector.WithoutSecrets().(*SAMLConnectorV2)
	require.Nil(t, withoutSecrets.GetSigningKeyPair())
	require.Nil(t, withoutSecrets.GetEncryptionKeyPair())
}
