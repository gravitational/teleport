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

	"github.com/gravitational/teleport/api/types/samlsp"
)

// TestNewSAMLIdPServiceProvider ensures a valid SAML IdP service provider.
func TestNewSAMLIdPServiceProvider(t *testing.T) {
	const acsURL = "https://example.com/acs"
	tests := []struct {
		name               string
		entityDescriptor   string
		entityID           string
		acsURL             string
		errAssertion       require.ErrorAssertionFunc
		expectedEntityID   string
		attributeMapping   []*SAMLAttributeMapping
		preset             string
		relayState         string
		expectedRelayState string
		launchURLs         []string
	}{
		{
			name:             "valid entity descriptor",
			entityDescriptor: testEntityDescriptor,
			entityID:         "IAMShowcase",
			errAssertion:     require.NoError,
			expectedEntityID: "IAMShowcase",
		},
		{
			// This validates that parse is not called when the entity ID is set.
			name:             "invalid entity descriptor with valid entity ID",
			entityDescriptor: "invalid XML",
			entityID:         "IAMShowcase",
			errAssertion:     require.NoError,
			expectedEntityID: "IAMShowcase",
		},
		{
			name:             "empty entity descriptor, entity ID and ACS URL",
			entityDescriptor: "",
			errAssertion:     require.Error,
		},
		{
			name:             "empty entity ID",
			entityDescriptor: testEntityDescriptor,
			errAssertion:     require.NoError,
			expectedEntityID: "IAMShowcase",
		},
		{
			name:             "empty entity descriptor and entity ID",
			entityDescriptor: "",
			acsURL:           acsURL,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, ErrEmptyEntityDescriptorAndEntityID)
			},
		},
		{
			name:             "empty entity descriptor and ACS URL",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, ErrEmptyEntityDescriptorAndACSURL)
			},
		},
		{
			name:             "empty entity descriptor with entity ID and ACS URL",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			acsURL:           acsURL,
			errAssertion:     require.NoError,
			expectedEntityID: "IAMShowcase",
		},
		{
			name:             "duplicate attribute mapping",
			entityDescriptor: testEntityDescriptor,
			attributeMapping: []*SAMLAttributeMapping{
				{
					Name:  "username",
					Value: "user.traits.name",
				},
				{
					Name:  "user1",
					Value: "user.traits.firstname",
				},
				{
					Name:  "username",
					Value: "user.traits.givenname",
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, ErrDuplicateAttributeName)
			},
		},
		{
			name:             "missing attribute name",
			entityDescriptor: testEntityDescriptor,
			entityID:         "IAMShowcase",
			expectedEntityID: "IAMShowcase",
			attributeMapping: []*SAMLAttributeMapping{
				{
					Name:  "",
					Value: "user.traits.name",
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "attribute name is required")
			},
		},
		{
			name:             "missing attribute value",
			entityDescriptor: testEntityDescriptor,
			entityID:         "IAMShowcase",
			expectedEntityID: "IAMShowcase",
			attributeMapping: []*SAMLAttributeMapping{
				{
					Name:  "name",
					Value: "",
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "attribute value is required")
			},
		},
		{
			name:             "valid attribute mapping",
			entityDescriptor: testEntityDescriptor,
			entityID:         "IAMShowcase",
			expectedEntityID: "IAMShowcase",
			attributeMapping: []*SAMLAttributeMapping{
				{
					Name:  "username",
					Value: "user.traits.name",
				},
				{
					Name:  "user1",
					Value: "user.traits.givenname",
				},
			},
			errAssertion: require.NoError,
		},
		{
			name:             "invalid attribute mapping name format",
			entityDescriptor: testEntityDescriptor,
			entityID:         "IAMShowcase",
			expectedEntityID: "IAMShowcase",
			attributeMapping: []*SAMLAttributeMapping{
				{
					Name:       "username",
					Value:      "user.traits.name",
					NameFormat: "emailAddress",
				},
				{
					Name:  "user1",
					Value: "user.traits.givenname",
				},
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid name format")
			},
		},
		{
			name:             "supported empty preset value",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			acsURL:           acsURL,
			expectedEntityID: "IAMShowcase",
			errAssertion:     require.NoError,
			preset:           samlsp.GCPWorkforce,
		},
		{
			name:             "supported unspecified preset value",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			acsURL:           acsURL,
			expectedEntityID: "IAMShowcase",
			errAssertion:     require.NoError,
			preset:           samlsp.Unspecified,
		},
		{
			name:             "aws-identity-center preset",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			acsURL:           acsURL,
			expectedEntityID: "IAMShowcase",
			errAssertion:     require.NoError,
			preset:           samlsp.AWSIdentityCenter,
		},
		{
			name:             "unsupported preset value",
			entityDescriptor: "",
			entityID:         "IAMShowcase",
			acsURL:           acsURL,
			expectedEntityID: "IAMShowcase",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported preset")
			},
			preset: "notsupported",
		},
		{
			name:               "GCP Workforce user provided relay state",
			entityID:           "IAMShowcase",
			acsURL:             acsURL,
			errAssertion:       require.NoError,
			preset:             samlsp.GCPWorkforce,
			relayState:         "user_provided_relay_state",
			expectedRelayState: "user_provided_relay_state",
		},
		{
			name:               "GCP Workforce default relay state",
			entityID:           "IAMShowcase",
			acsURL:             acsURL,
			errAssertion:       require.NoError,
			preset:             samlsp.GCPWorkforce,
			expectedRelayState: samlsp.DefaultRelayStateGCPWorkforce,
		},
		{
			name:               "default relay state should not be set for empty preset value",
			entityID:           "IAMShowcase",
			acsURL:             acsURL,
			errAssertion:       require.NoError,
			preset:             "",
			expectedRelayState: "",
		},
		{
			name:       "http launch url",
			entityID:   "IAMShowcase",
			acsURL:     acsURL,
			launchURLs: []string{"http://test.com/myapp"},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid scheme")
			},
		},
		{
			name:       "empty launch URLs",
			entityID:   "IAMShowcase",
			acsURL:     acsURL,
			launchURLs: []string{""},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid scheme")
			},
		},
		{
			name:         "valid launch URLs",
			entityID:     "IAMShowcase",
			acsURL:       acsURL,
			launchURLs:   []string{"https://test.com/myapp", "https://anysubdomain.test.com/myapp"},
			errAssertion: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sp, err := NewSAMLIdPServiceProvider(Metadata{
				Name: "test",
			}, SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: test.entityDescriptor,
				EntityID:         test.entityID,
				ACSURL:           test.acsURL,
				AttributeMapping: test.attributeMapping,
				Preset:           test.preset,
				RelayState:       test.relayState,
				LaunchURLs:       test.launchURLs,
			})

			test.errAssertion(t, err)
			if sp != nil {
				if test.expectedEntityID != "" {
					require.Equal(t, test.expectedEntityID, sp.GetEntityID())
				}
				if len(sp.GetAttributeMapping()) > 0 {
					require.Equal(t, test.attributeMapping, sp.GetAttributeMapping())
				}
				if test.preset == "" && test.relayState == "" {
					require.Empty(t, sp.GetRelayState())
				}
				if test.expectedRelayState != "" {
					require.Equal(t, test.expectedRelayState, sp.GetRelayState())
				}
			}
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
