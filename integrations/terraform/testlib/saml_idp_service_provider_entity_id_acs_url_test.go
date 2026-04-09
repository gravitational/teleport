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
 * using entity_id+acs_url.
 */

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityIDACSURLCreateUpdateDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-id-acs-url-create-update"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/create/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-id-acs-url/create/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-create-update"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/update/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-id-acs-url/update/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityIDACSURLMigrationToDescriptorSameEntityID() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-id-acs-url-migration-same"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_2_migration_same_from_entity_id.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-migration-same"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/migration/same/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-id-acs-url/migration/same/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_2_migration_same_from_entity_id.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_3_migration_same_to_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-migration-same"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/migration/same/metadata"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_3_migration_same_to_descriptor.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityIDACSURLMigrationToDescriptorChangedEntityID() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-id-acs-url-migration-changed"),
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_4_migration_changed_from_entity_id.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-migration-changed"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/migration/changed/from/metadata"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url", "https://sp.example.com/entity-id-acs-url/migration/changed/from/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_4_migration_changed_from_entity_id.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_entity_id_acs_url_5_migration_changed_to_descriptor.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "metadata.name", "test-entity-id-acs-url-migration-changed"),
					resource.TestCheckResourceAttr(samlIDPServiceProviderTFResourceName, "spec.entity_id", "https://sp.example.com/entity-id-acs-url/migration/changed/to/metadata"),
					resource.TestCheckNoResourceAttr(samlIDPServiceProviderTFResourceName, "spec.acs_url"), // Failure here? Make sure acs_url is NOT a computed field
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_entity_id_acs_url_5_migration_changed_to_descriptor.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestSAMLIdPServiceProviderEntityIDACSURLImportDelete() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadata := types.Metadata{
		Name: "test-entity-id-acs-url-import",
	}
	sp, err := types.NewSAMLIdPServiceProvider(metadata, types.SAMLIdPServiceProviderSpecV1{
		EntityID: "https://sp.example.com/entity-id-acs-url/import/metadata",
		ACSURL:   "https://sp.example.com/entity-id-acs-url/import/acs",
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.client.CreateSAMLIdPServiceProvider(ctx, sp))

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		CheckDestroy:              s.samlIDPServiceProviderCheckDestroyed(ctx, "test-entity-id-acs-url-import"),
		Steps: []resource.TestStep{
			{
				Config:        s.getFixture("saml_idp_service_provider_entity_id_acs_url_6_import.tf"),
				ResourceName:  samlIDPServiceProviderTFResourceName,
				ImportState:   true,
				ImportStateId: "test-entity-id-acs-url-import",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					require.Equal(s.T(), "test-entity-id-acs-url-import", states[0].Attributes["metadata.name"])
					require.Equal(s.T(), "https://sp.example.com/entity-id-acs-url/import/metadata", states[0].Attributes["spec.entity_id"])
					require.Equal(s.T(), "https://sp.example.com/entity-id-acs-url/import/acs", states[0].Attributes["spec.acs_url"])
					return nil
				},
			},
		},
	})
}
