/*
Copyright 2026 Gravitational, Inc.

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

/* This file tests explicitly setting all fields of saml_idp_service_provider
 * except entity_descriptor in terraform
 */

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderCreateUpdateDeleteAllFields() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-all-fields-create-update"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_all_fields_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-all-fields-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/all-fields/create/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/all-fields/create/acs"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name", "username"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.value", "external.username"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.name", "foobar"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.value", "external.foobar"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.2.name", "website"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.2.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.2.value", "external.website"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.preset", "unspecified"), // See api/types/samlsp/samlsp.go for possible values
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.relay_state", "https://example.com/relay-state"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.launch_urls.0", "https://example.com/launch/1"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.launch_urls.1", "https://example.com/launch/2"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_all_fields_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_all_fields_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-all-fields-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/all-fields/update/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/all-fields/update/acs"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name", "department"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.value", "external.department"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.name", "email"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.1.value", "external.email"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.preset", "gcp-workforce"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.relay_state", "https://example.com/updated-relay-state"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.launch_urls.0", "https://example.com/updated-launch/1"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.launch_urls.1", "https://example.com/updated-launch/2"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_all_fields_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderImportAllFields() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-all-fields-import",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityID: "https://sp.example.com/all-fields/import/metadata",
		ACSURL:   "https://sp.example.com/all-fields/import/acs",
		AttributeMapping: []*types.SAMLAttributeMapping{
			{
				Name:       "username",
				NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
				Value:      "external.username",
			},
			{
				Name:       "groups",
				NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
				Value:      "external.groups",
			},
		},
		Preset:     "unspecified",
		RelayState: "https://example.com/relay-state",
		LaunchURLs: []string{
			"https://example.com/launch/1",
			"https://example.com/launch/2",
		},
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-all-fields-import"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_all_fields_2_import.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-all-fields-import",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-all-fields-import", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/all-fields/import/metadata", states[0].Attributes["spec.entity_id"])
					require.Equal(s.T(), "https://sp.example.com/all-fields/import/acs", states[0].Attributes["spec.acs_url"])
					require.Equal(s.T(), "username", states[0].Attributes["spec.attribute_mapping.0.name"])
					require.Equal(s.T(), "urn:oasis:names:tc:SAML:2.0:attrname-format:basic", states[0].Attributes["spec.attribute_mapping.0.name_format"])
					require.Equal(s.T(), "external.username", states[0].Attributes["spec.attribute_mapping.0.value"])
					require.Equal(s.T(), "groups", states[0].Attributes["spec.attribute_mapping.1.name"])
					require.Equal(s.T(), "urn:oasis:names:tc:SAML:2.0:attrname-format:basic", states[0].Attributes["spec.attribute_mapping.1.name_format"])
					require.Equal(s.T(), "external.groups", states[0].Attributes["spec.attribute_mapping.1.value"])
					require.Equal(s.T(), "unspecified", states[0].Attributes["spec.preset"])
					require.Equal(s.T(), "https://example.com/relay-state", states[0].Attributes["spec.relay_state"])
					require.Equal(s.T(), "https://example.com/launch/1", states[0].Attributes["spec.launch_urls.0"])
					require.Equal(s.T(), "https://example.com/launch/2", states[0].Attributes["spec.launch_urls.1"])
					return nil
				},
			},
		},
	})
}
