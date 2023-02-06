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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewSAMLIdPServiceProvider ensures a valid SAML IdP service provider.
func TestNewSAMLIdPServiceProvider(t *testing.T) {
	tests := []struct {
		name             string
		entityDescriptor string
		errAssertion     require.ErrorAssertionFunc
	}{
		{
			name:             "valid entity descriptor",
			entityDescriptor: testEntityDescriptor,
			errAssertion:     require.NoError,
		},
		{
			name:             "empty entity descriptor",
			entityDescriptor: "",
			errAssertion:     require.Error,
		},
		{
			name:             "entity descriptor only spaces",
			entityDescriptor: "    ",
			errAssertion:     require.Error,
		},
		{
			name:             "no XML",
			entityDescriptor: "this is not valid XML",
			errAssertion:     require.Error,
		},
		{
			name:             "invalid entity descriptor",
			entityDescriptor: "<invalid />",
			errAssertion:     require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSAMLIdPServiceProvider(Metadata{
				Name: "test",
			}, SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: test.entityDescriptor,
			})

			test.errAssertion(t, err)
		})
	}
}

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
