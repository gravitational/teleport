/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Config 2.0 (the "License");
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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"
)

func (s *TerraformSuiteOSS) TestUIConfig() {
	name := "teleport_ui_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("ui_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "ui_config"),
					resource.TestCheckResourceAttr(name, "spec.scrollback_lines", "1000"),
					resource.TestCheckResourceAttr(name, "spec.show_resources", "requestable"),
				),
			},
			{
				Config:   s.getFixture("ui_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("ui_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "ui_config"),
					resource.TestCheckResourceAttr(name, "spec.scrollback_lines", "2000"),
					resource.TestCheckResourceAttr(name, "spec.show_resources", "requestable"),
				),
			},
			{
				Config:   s.getFixture("ui_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportUIConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_ui_config"
	id := "test_import"
	name := r + "." + id

	uiConfig := &types.UIConfigV1{
		Spec: types.UIConfigSpecV1{
			ScrollbackLines: 1000,
			ShowResources:   "requestable",
		},
	}

	err := uiConfig.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.SetUIConfig(ctx, uiConfig)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetUIConfig(ctx)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(s.T(), err)
		return true
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
					require.Equal(s.T(), "ui_config", state[0].Attributes["kind"])
					require.Equal(s.T(), "1000", state[0].Attributes["spec.scrollback_lines"])
					require.Equal(s.T(), "requestable", state[0].Attributes["spec.show_resources"])

					return nil
				},
			},
		},
	})
}
