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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

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
