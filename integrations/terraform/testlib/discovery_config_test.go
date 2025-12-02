// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testlib

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
)

func (s *TerraformSuiteOSS) TestDiscoveryConfig() {
	t := s.T()
	name := "teleport_discovery_config.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("discovery_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.discovery_group", "azure_teleport"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.types.0", "vm"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.regions.0", "eastus"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.subscriptions.0", "123123-123123-123123-123123"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.resource_groups.0", "group"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.tags.%", "1"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.join_method", "azure"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.script_name", "default-installer"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.join_token", "azure-token"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.azure.client_id", "managed-identity-id"),
				),
			},
			{
				Config:   s.getFixture("discovery_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("discovery_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.discovery_group", "azure_teleport_updated"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.types.0", "vm"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.types.1", "aks"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.regions.0", "westus"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.regions.1", "eastus"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.subscriptions.0", "456456-456456-456456-456456"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.resource_groups.0", "group"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.resource_groups.1", "group2"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.tags.env.0", "prod"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.script_name", "updated-installer"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.join_token", "azure-token-updated"),
					resource.TestCheckResourceAttr(name, "spec.azure.0.install_params.azure.client_id", "managed-identity-id-updated"),
				),
			},
			{
				Config:   s.getFixture("discovery_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportDiscoveryConfig() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := "teleport_discovery_config"
	id := "test_import"
	name := r + "." + id

	discoveryCfg, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name:        id,
			Description: "Test discovery config for import",
			Labels: map[string]string{
				"purpose": "test",
			},
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "test_group",
		},
	)
	require.NoError(t, err)

	_, err = s.client.DiscoveryConfigClient().CreateDiscoveryConfig(ctx, discoveryCfg)
	require.NoError(t, err)

	existing, err := s.client.DiscoveryConfigClient().GetDiscoveryConfig(ctx, id)
	require.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(t, existing.Metadata.Name, state[0].Attributes["header.metadata.name"])
					require.Equal(t, existing.Metadata.Description, state[0].Attributes["header.metadata.description"])
					require.Equal(t, "test", state[0].Attributes["header.metadata.labels.purpose"])
					require.Equal(t, existing.Metadata.Revision, state[0].Attributes["header.metadata.revision"])
					require.Equal(t, "test_group", state[0].Attributes["spec.discovery_group"])

					return nil
				},
			},
		},
	})
}
