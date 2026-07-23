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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteEnterprise) TestBeamsConfigDataSource() {
	name := "data.teleport_beams_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("beams_config_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "beams_config"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "id", types.MetaNameBeamsConfig),
				),
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestBeamsConfig() {
	name := "teleport_beams_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("beams_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.llm.anthropic.app_name", "my-anthropic"),
					resource.TestCheckResourceAttr(name, "spec.llm.openai.app_name", "my-openai"),
				),
			},
			{
				Config:   s.getFixture("beams_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("beams_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "spec.llm.anthropic.app_name", "updated-anthropic"),
					resource.TestCheckResourceAttr(name, "spec.llm.openai.app_name", "updated-openai"),
				),
			},
			{
				Config:   s.getFixture("beams_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
