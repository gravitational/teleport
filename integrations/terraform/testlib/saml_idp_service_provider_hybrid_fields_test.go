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

/* This file tests cases that start by defining a saml_idp_service_provider
 * using both entity_id+acs_url+entity_descriptor.
 */

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderHybridFieldsCreateUpdateDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-hybrid-fields-create-update"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/create/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/hybrid-fields/create/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/update/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/hybrid-fields/update/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderHybridFieldsMigrationToDescriptor() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-hybrid-fields-migration-to-descriptor"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_2_migration_to_descriptor_from_hybrid.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-migration-to-descriptor"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/migration/to-descriptor/from/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_2_migration_to_descriptor_from_hybrid.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_3_migration_to_descriptor_to_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-migration-to-descriptor"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/migration/to-descriptor/to/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_3_migration_to_descriptor_to_descriptor.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderHybridFieldsMigrationToEntityID() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-hybrid-fields-migration-to-entity-id"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_4_migration_to_entity_id_from_hybrid.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-migration-to-entity-id"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/migration/to-entity-id/from/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_4_migration_to_entity_id_from_hybrid.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_hybrid_fields_5_migration_to_entity_id_to_entity_id.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-hybrid-fields-migration-to-entity-id"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/hybrid-fields/migration/to-entity-id/to/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/hybrid-fields/migration/to-entity-id/to/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_hybrid_fields_5_migration_to_entity_id_to_entity_id.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderHybridFieldsImportDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-hybrid-fields-import",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityID:         "https://sp.example.com/hybrid-fields/import/metadata",
		ACSURL:           "https://sp.example.com/hybrid-fields/import/acs",
		EntityDescriptor: `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/hybrid-fields/import/metadata"><md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/hybrid-fields/import/acs" index="0"/></md:SPSSODescriptor></md:EntityDescriptor>`,
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-hybrid-fields-import"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_hybrid_fields_6_import.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-hybrid-fields-import",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-hybrid-fields-import", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/hybrid-fields/import/metadata", states[0].Attributes["spec.entity_id"])
					require.Equal(s.T(), "https://sp.example.com/hybrid-fields/import/acs", states[0].Attributes["spec.acs_url"])
					return nil
				},
			},
		},
	})
}
