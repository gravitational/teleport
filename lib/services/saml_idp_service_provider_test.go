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
	"fmt"
	"strings"
	"testing"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
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

func TestFilterSAMLEntityDescriptor(t *testing.T) {
	tts := []struct {
		eds           string
		ok            bool
		before, after int
		name          string
	}{
		{
			eds: edBuilder().
				ACS(saml.HTTPPostBinding, "https://one.example.com/acs").
				ACS(saml.HTTPPostBinding, "https://two.example.com/acs").
				Done(),
			ok:     true,
			before: 2,
			after:  2,
			name:   "no filtering",
		},
		{
			eds: edBuilder().
				ACS(saml.HTTPPostBinding, "https://example.com/acs").
				ACS(saml.HTTPPostBinding, "http://example.com/acs").
				Done(),
			ok:     false,
			before: 2,
			after:  1,
			name:   "scheme filtering",
		},
		{
			eds: edBuilder().
				ACS(saml.HTTPArtifactBinding, "https://example.com/acs").
				ACS("urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST-SimpleSign", "https://example.com/POST-SimpleSign").
				ACS(saml.HTTPPostBinding, "https://example.com/acs").
				Done(),
			ok:     false,
			before: 3,
			after:  1,
			name:   "binding filtering",
		},
		{
			eds: edBuilder().
				ACS("urn:oasis:names:tc:SAML:2.0:bindings:PAOS", "https://example.com/ECP").
				ACS(saml.HTTPPostBinding, "http://example.com/acs").
				Done(),
			ok:     false,
			before: 2,
			after:  0,
			name:   "all invalid",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			ed, err := samlsp.ParseMetadata([]byte(tt.eds))
			require.NoError(t, err)

			require.Equal(t, tt.before, getACSCount(ed))

			err = FilterSAMLEntityDescriptor(ed, false /* quiet */)
			if !tt.ok {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.after, getACSCount(ed))
		})
	}
}

func getACSCount(ed *saml.EntityDescriptor) int {
	var count int
	for _, desc := range ed.SPSSODescriptors {
		count += len(desc.AssertionConsumerServices)
	}
	return count
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
		{
			location:  `https://sptest.iamshowcase.com/acs"`,
			assertion: require.Error,
		},
		{
			location:  `https://sptest.iamshowcase.com/acs<`,
			assertion: require.Error,
		},
		{
			location:  `https://sptest.iamshowcase.com/acs>`,
			assertion: require.Error,
		},
		{
			location:  `https://sptest.iamshowcase.com/acs!`,
			assertion: require.Error,
		},
		{
			location:  `https://sptest.iamshowcase.com/acs;`,
			assertion: require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.location, func(t *testing.T) {
			test.assertion(t, validateAssertionConsumerServicesEndpoint(test.location))
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

const edBuilderTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
	  %s
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

const edBuilderACSTemplate = `<md:AssertionConsumerService Binding="%s" Location="%s" index="%d" isDefault="true"/>`

type entityDescriptorBuilder struct {
	acs []struct {
		binding, location string
	}
}

func edBuilder() *entityDescriptorBuilder {
	return &entityDescriptorBuilder{}
}

func (b *entityDescriptorBuilder) ACS(binding, location string) *entityDescriptorBuilder {
	b.acs = append(b.acs, struct {
		binding, location string
	}{binding, location})
	return b
}

func (b *entityDescriptorBuilder) Done() string {
	var acss []string
	for i, acs := range b.acs {
		acss = append(acss, fmt.Sprintf(edBuilderACSTemplate, acs.binding, acs.location, i))
	}

	return fmt.Sprintf(edBuilderTemplate, strings.Join(acss, "\n      "))
}

func TestValidateSAMLIdPACSURLAndRelayStateInputs(t *testing.T) {
	cases := []struct {
		acsURL     string
		entityID   string
		relayState string
		assertion  require.ErrorAssertionFunc
	}{
		{
			acsURL:     "https://sp.com/",
			entityID:   "https://sp.com/",
			relayState: "https://sp.com/",
			assertion:  require.NoError,
		},
		{
			acsURL:    "javascript://sp.com/acs",
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:    `https://sp.com/acs"`,
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:    "https://sp.com/acs<",
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:    "https://sp.com/acs>",
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:    "https://sp.com/acs!",
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:    "https://sp.com/acs;",
			entityID:  "https://sp.com/",
			assertion: require.Error,
		},
		{
			acsURL:     "https://sp.com/acs",
			entityID:   "https://sp.com/",
			relayState: `https://sp.com/acs>`,
			assertion:  require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.acsURL, func(t *testing.T) {
			sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
				Name: "sp",
			}, types.SAMLIdPServiceProviderSpecV1{
				EntityID:   test.entityID,
				ACSURL:     test.acsURL,
				RelayState: test.relayState,
			})
			require.NoError(t, err)
			test.assertion(t, ValidateSAMLIdPACSURLAndRelayStateInputs(sp))
		})
	}
}
