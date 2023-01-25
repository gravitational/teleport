/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestSAMLIdPServiceProviderUnmarshal verifies a SAML IdP service provider resource can be unmarshaled.
func TestSAMLIdPServiceProviderUnmarshal(t *testing.T) {
	expected, err := types.NewSAMLIdPServiceProvider("test-sp",
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "<valid />",
		})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(samlIDPServiceProviderYAML))
	require.NoError(t, err)
	actual, err := UnmarshalSAMLIdPServiceProvider(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestSAMLIdPServiceProviderMarshal verifies a marshaled SAML IdP service provider resources can be unmarshaled back.
func TestSAMLIdPServiceProviderMarshal(t *testing.T) {
	expected, err := types.NewSAMLIdPServiceProvider("test-sp",
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "<valid />",
		})
	require.NoError(t, err)
	data, err := MarshalSAMLIdPServiceProvider(expected)
	require.NoError(t, err)
	actual, err := UnmarshalSAMLIdPServiceProvider(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var samlIDPServiceProviderYAML = `---
kind: saml_idp_service_provider
version: v1
metadata:
  name: test-sp
spec:
  version: v1
  entity_descriptor: <valid />
`
