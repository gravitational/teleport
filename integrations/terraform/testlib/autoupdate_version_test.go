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
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

func (s *TerraformSuiteOSS) TestAutoUpdateVersion() {
	name := "teleport_autoupdate_version.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("autoupdate_version_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "autoupdate_version"),
					resource.TestCheckResourceAttr(name, "spec.tools.target_version", "1.2.3"),
					resource.TestCheckResourceAttr(name, "spec.agents.schedule", "regular"),
					resource.TestCheckResourceAttr(name, "spec.agents.mode", "enabled"),
					resource.TestCheckResourceAttr(name, "spec.agents.start_version", "1.2.3"),
					resource.TestCheckResourceAttr(name, "spec.agents.target_version", "1.2.4"),
				),
			},
			{
				Config:   s.getFixture("autoupdate_version_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("autoupdate_version_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "autoupdate_version"),
					resource.TestCheckResourceAttr(name, "spec.tools.target_version", "1.2.4"),
					resource.TestCheckResourceAttr(name, "spec.agents.schedule", "regular"),
					resource.TestCheckResourceAttr(name, "spec.agents.mode", "enabled"),
					resource.TestCheckResourceAttr(name, "spec.agents.start_version", "1.2.3"),
					resource.TestCheckResourceAttr(name, "spec.agents.target_version", "1.2.5"),
				),
			},
			{
				Config:   s.getFixture("autoupdate_version_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportAutoUpdateVersion() {
	r := "teleport_autoupdate_version"
	name := r + ".test"

	testStartVersion := "1.2.3"
	testTargetVersion := "1.2.4"
	testSchedule := "regular"
	testMode := "enabled"

	autoUpdateVersion, err := autoupdate.NewAutoUpdateVersion(
		&autoupdatev1pb.AutoUpdateVersionSpec{
			// We intentionally don't set tools to test a partial resource
			Tools: nil,
			Agents: &autoupdatev1pb.AutoUpdateVersionSpecAgents{
				StartVersion:  testStartVersion,
				TargetVersion: testTargetVersion,
				Schedule:      testSchedule,
				Mode:          testMode,
			},
		},
	)
	require.NoError(s.T(), err)

	autoUpdateVersion, err = s.client.CreateAutoUpdateVersion(s.T().Context(), autoUpdateVersion)
	require.NoError(s.T(), err)

	require.EventuallyWithT(s.T(), func(t *assert.CollectT) {
		autoUpdateVersionCurrent, err := s.client.GetAutoUpdateVersion(s.T().Context())
		require.NoError(t, err)
		require.Equal(t, autoUpdateVersion.GetMetadata().GetRevision(), autoUpdateVersionCurrent.GetMetadata().GetRevision())
	}, 15*time.Second, 100*time.Millisecond)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "test" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: "autoupdate-version",
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "autoupdate_version", state[0].Attributes["kind"])
					require.Equal(s.T(), testStartVersion, state[0].Attributes["spec.agents.start_version"])
					require.Equal(s.T(), testTargetVersion, state[0].Attributes["spec.agents.target_version"])
					require.Equal(s.T(), testSchedule, state[0].Attributes["spec.agents.schedule"])
					require.Equal(s.T(), testMode, state[0].Attributes["spec.agents.mode"])

					return nil
				},
			},
		},
	})
}
