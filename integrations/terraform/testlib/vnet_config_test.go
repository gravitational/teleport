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

package testlib

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	vnetv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
)

const vnetConfigSingletonName = "vnet-config"

// resetVnetConfig ensures singleton tests don't bleed state into each other.
func (s *TerraformSuiteOSS) resetVnetConfig(ctx context.Context) {
	err := s.client.DeleteVnetConfig(ctx)
	if trace.IsNotFound(err) {
		return
	}
	require.NoError(s.T(), err)
}

func (s *TerraformSuiteOSS) TestVnetConfigDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetVnetConfig(ctx)
	s.T().Cleanup(func() { s.resetVnetConfig(ctx) })

	name := "data.teleport_vnet_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("vnet_config_data_source_resource.tf"),
			},
			{
				Config: s.getFixture("vnet_config_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", vnetConfigSingletonName),
					resource.TestCheckResourceAttr(name, "spec.ipv4_cidr_range", "10.10.0.0/16"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestVnetConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetVnetConfig(ctx)
	s.T().Cleanup(func() { s.resetVnetConfig(ctx) })

	name := "teleport_vnet_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("vnet_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", vnetConfigSingletonName),
					resource.TestCheckResourceAttr(name, "id", vnetConfigSingletonName),
					resource.TestCheckResourceAttr(name, "spec.ipv4_cidr_range", "192.168.1.0/24"),
					resource.TestCheckResourceAttr(name, "spec.custom_dns_zones.0.suffix", "example.com"),
				),
			},
			{
				Config:   s.getFixture("vnet_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("vnet_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", vnetConfigSingletonName),
					resource.TestCheckResourceAttr(name, "id", vnetConfigSingletonName),
					resource.TestCheckResourceAttr(name, "spec.ipv4_cidr_range", "192.168.2.0/24"),
					resource.TestCheckResourceAttr(name, "spec.custom_dns_zones.0.suffix", "internal.example.com"),
				),
			},
			{
				Config:   s.getFixture("vnet_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportVnetConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	s.resetVnetConfig(ctx)
	s.T().Cleanup(func() { s.resetVnetConfig(ctx) })

	r := "teleport_vnet_config"
	id := "test_import"
	name := r + "." + id

	testIPv4CIDR := "172.30.0.0/16"
	testZone := "import.example.com"

	vnetConfig, err := typesvnet.NewVnetConfig(&vnetv1pb.VnetConfigSpec{
		Ipv4CidrRange: testIPv4CIDR,
		CustomDnsZones: []*vnetv1pb.CustomDNSZone{
			{Suffix: testZone},
		},
	})
	require.NoError(s.T(), err)

	_, err = s.client.UpsertVnetConfig(ctx, vnetConfig)
	require.NoError(s.T(), err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "vnet_config", state[0].Attributes["kind"])
					require.Equal(s.T(), "v1", state[0].Attributes["version"])
					require.Equal(s.T(), vnetConfigSingletonName, state[0].Attributes["metadata.name"])
					require.Equal(s.T(), vnetConfigSingletonName, state[0].Attributes["id"])
					require.Equal(s.T(), testIPv4CIDR, state[0].Attributes["spec.ipv4_cidr_range"])
					require.Equal(s.T(), testZone, state[0].Attributes["spec.custom_dns_zones.0.suffix"])

					return nil
				},
			},
		},
	})
}
