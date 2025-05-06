/*
Copyright 2025 Gravitational, Inc.

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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

func (s *TerraformSuiteOSS) TestAutoUpdateConfig() {
	name := "teleport_autoupdate_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("autoupdate_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "autoupdate_config"),
					resource.TestCheckResourceAttr(name, "spec.tools.mode", "enabled"),
					resource.TestCheckResourceAttr(name, "spec.agents.mode", "enabled"),
					resource.TestCheckResourceAttr(name, "spec.agents.strategy", "halt-on-error"),
					resource.TestCheckResourceAttr(name, "spec.agents.schedules.regular.0.name", "dev"),
				),
			},
			{
				Config:   s.getFixture("autoupdate_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("autoupdate_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "autoupdate_config"),
					resource.TestCheckResourceAttr(name, "spec.tools.mode", "enabled"),
					resource.TestCheckResourceAttr(name, "spec.agents.mode", "suspended"),
					resource.TestCheckResourceAttr(name, "spec.agents.strategy", "time-based"),
					resource.TestCheckResourceAttr(name, "spec.agents.maintenance_window_duration", "45m"),
				),
			},
			{
				Config:   s.getFixture("autoupdate_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportAutoUpdateConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_autoupdate_config"
	id := "test_import"
	name := r + "." + id

	testMode := "enabled"
	testStrategy := "halt-on-error"

	autoUpdateConfig, err := autoupdate.NewAutoUpdateConfig(
		&autoupdatev1pb.AutoUpdateConfigSpec{
			// We intentionally don't set tools to test a partial resource
			Tools: nil,
			Agents: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
				Mode:     testMode,
				Strategy: testStrategy,
				Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
					Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
						{
							Name:      "dev",
							Days:      []string{"Mon", "Tue", "Wed", "Thu"},
							StartHour: 4,
						},
						{
							Name:      "staging",
							Days:      []string{"Mon", "Tue", "Wed", "Thu"},
							StartHour: 14,
						},
						{
							Name:      "production",
							Days:      []string{"Mon", "Tue", "Wed", "Thu"},
							StartHour: 14,
							WaitHours: 24,
						},
					},
				},
			},
		},
	)
	require.NoError(s.T(), err)

	autoUpdateConfig, err = s.client.CreateAutoUpdateConfig(ctx, autoUpdateConfig)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		autoUpdateConfigCurrent, err := s.client.GetAutoUpdateConfig(ctx)
		require.NoError(s.T(), err)

		return autoUpdateConfig.GetMetadata().GetRevision() != autoUpdateConfigCurrent.GetMetadata().GetName()
	}, 5*time.Second, 200*time.Millisecond)

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
					require.Equal(s.T(), "autoupdate_config", state[0].Attributes["kind"])
					require.Equal(s.T(), testMode, state[0].Attributes["spec.agents.mode"])
					require.Equal(s.T(), testStrategy, state[0].Attributes["spec.agents.strategy"])
					require.Equal(s.T(), "dev", state[0].Attributes["spec.agents.schedules.regular.0.name"])

					return nil
				},
			},
		},
	})
}
