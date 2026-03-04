/*
Copyright 2015-2021 Gravitational, Inc.

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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestClusterMaintenanceConfigDataSource() {
	ctx, cancel := context.WithCancel(s.T().Context())
	s.T().Cleanup(cancel)

	name := "data.teleport_cluster_maintenance_config.test"

	config := &types.ClusterMaintenanceConfigV1{
		Spec: types.ClusterMaintenanceConfigSpecV1{
			AgentUpgrades: &types.AgentUpgradeWindow{
				UTCStartHour: 8,
				Weekdays:     []string{"Mon"},
			},
		},
	}

	err := config.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.UpdateClusterMaintenanceConfig(ctx, config)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetClusterMaintenanceConfig(ctx)
		return err == nil
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("maintenance_config_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_maintenance_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", "cluster-maintenance-config"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.utc_start_hour", "8"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.weekdays.0", "Mon"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestClusterMaintenanceConfig() {
	name := "teleport_cluster_maintenance_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("maintenance_config_0_set.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_maintenance_config"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.utc_start_hour", "1"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.weekdays.0", "monday"),
				),
			},
			{
				Config:   s.getFixture("maintenance_config_0_set.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("maintenance_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_maintenance_config"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.utc_start_hour", "12"),
					resource.TestCheckResourceAttr(name, "spec.agent_upgrades.weekdays.0", "tuesday"),
				),
			},
			{
				Config:   s.getFixture("maintenance_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
