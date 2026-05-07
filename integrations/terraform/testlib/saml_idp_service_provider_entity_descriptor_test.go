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
 * using an entity_descriptor
 */

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityDescriptorCreateUpdateDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-descriptor-create-update"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/create/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/update/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityDescriptorMigrationToEntityIDSameEntityID() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-descriptor-migration-same"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_2_migration_same_from_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-migration-same"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/migration/same/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_2_migration_same_from_descriptor.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_3_migration_same_to_entity_id.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-migration-same"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/migration/same/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-descriptor/migration/same/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_3_migration_same_to_entity_id.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityDescriptorMigrationToEntityIDChangedEntityID() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-descriptor-migration-changed"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_4_migration_changed_from_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-migration-changed"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/migration/changed/from/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_4_migration_changed_from_descriptor.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_descriptor_5_migration_changed_to_entity_id.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-descriptor-migration-changed"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-descriptor/migration/changed/to/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-descriptor/migration/changed/to/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_descriptor_5_migration_changed_to_entity_id.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityDescriptorImportDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-entity-descriptor-import",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityDescriptor: `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/entity-descriptor/import/metadata"><SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/entity-descriptor/import/acs" index="0"/></SPSSODescriptor></EntityDescriptor>`,
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-descriptor-import"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_entity_descriptor_6_import.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-entity-descriptor-import",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-entity-descriptor-import", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/entity-descriptor/import/metadata", states[0].Attributes["spec.entity_id"])
					return nil
				},
			},
		},
	})
}
