/*
Copyright 2025 Gravitational, Inc.

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
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	vnet "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	vnettypes "github.com/gravitational/teleport/api/types/vnet"
)

func (s *TerraformSuiteOSS) TestVnetConfigDataSource() {
	name := "data.teleport_vnet_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("vnet_config_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", types.MetaNameVnetConfig),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestVnetConfig() {
	name := "teleport_vnet_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("vnet_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					//resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "spec.ipv4_cidr_range", "100.64.0.0/10"),
					resource.TestCheckResourceAttr(name, "spec.custom_dns_zones.0.suffix", "internal.example.com"),
				),
			},
			{
				Config:   s.getFixture("vnet_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("vnet_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					//resource.TestCheckResourceAttr(name, "kind", "vnet_config"),
					resource.TestCheckResourceAttr(name, "spec.ipv4_cidr_range", "100.64.0.0/11"),
					resource.TestCheckResourceAttr(name, "spec.custom_dns_zones.0.suffix", "updated.example.com"),
					resource.TestCheckResourceAttr(name, "spec.custom_dns_zones.1.suffix", "svc.example.com"),
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

	r := "teleport_vnet_config"
	id := "test_import"
	name := r + "." + id

	vnetConfig, err := vnettypes.NewVnetConfig(&vnet.VnetConfigSpec{
		Ipv4CidrRange: "100.64.0.0/12",
		CustomDnsZones: []*vnet.CustomDNSZone{{
			Suffix: "import.example.com",
		}},
	})
	require.NoError(s.T(), err)

	err = s.client.ResetVnetConfig(ctx)
	require.NoError(s.T(), err)

	vnetConfigBefore, err := s.client.GetVnetConfig(ctx)
	require.NoError(s.T(), err)

	_, err = s.client.UpsertVnetConfig(ctx, vnetConfig)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		vnetConfigCurrent, err := s.client.GetVnetConfig(ctx)
		require.NoError(s.T(), err)

		return vnetConfigBefore.GetMetadata().GetRevision() != vnetConfigCurrent.GetMetadata().GetRevision()
	}, 5*time.Second, time.Second)

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
					require.Equal(s.T(), "100.64.0.0/12", state[0].Attributes["spec.ipv4_cidr_range"])
					require.Equal(s.T(), "import.example.com", state[0].Attributes["spec.custom_dns_zones.0.suffix"])

					return nil
				},
			},
		},
	})
}
