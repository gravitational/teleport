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

// These tests demonstrate that the API's modification of `entity_descriptor`
// based on the value of `attribute_mapping` is handled by the provider.

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderAttributeMappingCreateUpdateDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-attribute-mapping-create-update"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/create/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/attribute-mapping/create/acs"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name", "username"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.value", "external.username"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/update/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/attribute-mapping/update/acs"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name", "username"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.value", "external.email"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderAttributeMappingMigrationToDescriptor() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-attribute-mapping-migration-to-descriptor"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_2_migration_to_descriptor_from_mapping.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-migration-to-descriptor"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/migration/to-descriptor/from/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/attribute-mapping/migration/to-descriptor/from/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_2_migration_to_descriptor_from_mapping.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_3_migration_to_descriptor_to_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-migration-to-descriptor"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/migration/to-descriptor/to/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_3_migration_to_descriptor_to_descriptor.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderAttributeMappingMigrationToAttributeMapping() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-attribute-mapping-migration-to-mapping"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_4_migration_to_mapping_from_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-migration-to-mapping"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/migration/to-mapping/from/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_4_migration_to_mapping_from_descriptor.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_attribute_mapping_5_migration_to_mapping_to_mapping.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-attribute-mapping-migration-to-mapping"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/attribute-mapping/migration/to-mapping/to/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/attribute-mapping/migration/to-mapping/to/acs"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name", "groups"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.name_format", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.attribute_mapping.0.value", "external.groups"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_attribute_mapping_5_migration_to_mapping_to_mapping.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderAttributeMappingImportDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-attribute-mapping-import",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityID: "https://sp.example.com/attribute-mapping/import/metadata",
		ACSURL:   "https://sp.example.com/attribute-mapping/import/acs",
		AttributeMapping: []*types.SAMLAttributeMapping{
			{
				Name:       "username",
				NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
				Value:      "external.username",
			},
		},
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-attribute-mapping-import"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_attribute_mapping_6_import.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-attribute-mapping-import",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-attribute-mapping-import", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/attribute-mapping/import/metadata", states[0].Attributes["spec.entity_id"])
					require.Equal(s.T(), "https://sp.example.com/attribute-mapping/import/acs", states[0].Attributes["spec.acs_url"])
					require.Equal(s.T(), "username", states[0].Attributes["spec.attribute_mapping.0.name"])
					require.Equal(s.T(), "urn:oasis:names:tc:SAML:2.0:attrname-format:basic", states[0].Attributes["spec.attribute_mapping.0.name_format"])
					require.Equal(s.T(), "external.username", states[0].Attributes["spec.attribute_mapping.0.value"])
					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderAttributeMappingImportExternalDescriptor() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-attribute-mapping-import-external-hybrid",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityDescriptor: `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/attribute-mapping/import-external/metadata">
  <md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/attribute-mapping/import-external/acs" index="0"/>
  </md:SPSSODescriptor>
  <md:SPSSODescriptor protocolSupportEnumeration="">
    <md:AttributeConsumingService index="0">
      <md:ServiceName xml:lang="en">teleport_saml_idp_service</md:ServiceName>
      <md:RequestedAttribute Name="username" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:basic"/>
    </md:AttributeConsumingService>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`,
		AttributeMapping: []*types.SAMLAttributeMapping{},
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-attribute-mapping-import-external-hybrid"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_attribute_mapping_7_import_external_hybrid.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-attribute-mapping-import-external-hybrid",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-attribute-mapping-import-external-hybrid", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/attribute-mapping/import-external/metadata", states[0].Attributes["spec.entity_id"])
					require.Empty(s.T(), states[0].Attributes["spec.acs_url"])
					require.Empty(s.T(), states[0].Attributes["spec.attribute_mapping.0.name"])
					require.NotContains(s.T(), states[0].Attributes["spec.entity_descriptor"], "RequestedAttribute")
					require.NotContains(s.T(), states[0].Attributes["spec.entity_descriptor"], "AttributeConsumingService")
					return nil
				},
			},
		},
	})
}
