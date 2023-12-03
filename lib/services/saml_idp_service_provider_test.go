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
