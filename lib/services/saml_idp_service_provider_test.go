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
	expected, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "test-sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testEntityDescriptor,
			EntityID:         "IAMShowcase",
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
	expected, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "test-sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testEntityDescriptor,
			EntityID:         "IAMShowcase",
		})
	require.NoError(t, err)
	data, err := MarshalSAMLIdPServiceProvider(expected)
	require.NoError(t, err)
	actual, err := UnmarshalSAMLIdPServiceProvider(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestValidateAssertionConsumerServicesEndpoint(t *testing.T) {
	cases := []struct {
		location  string
		assertion require.ErrorAssertionFunc
	}{
		{
			location:  "https://sptest.iamshowcase.com/acs",
			assertion: require.NoError,
		},
		{
			location:  "http://sptest.iamshowcase.com/acs",
			assertion: require.Error,
		},
		{
			location:  "javascript://sptest.iamshowcase.com/acs",
			assertion: require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.location, func(t *testing.T) {
			test.assertion(t, ValidateAssertionConsumerServicesEndpoint(test.location))
		})
	}
}

var samlIDPServiceProviderYAML = `---
kind: saml_idp_service_provider
version: v1
metadata:
  name: test-sp
spec:
  version: v1
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
       <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
          <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
       </md:SPSSODescriptor>
    </md:EntityDescriptor>
  entity_id: IAMShowcase
`

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
