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
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestSessionRecordingConfig() {
	name := "teleport_session_recording_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("session_recording_config_0_set.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "session_recording_config"),
					resource.TestCheckResourceAttr(name, "spec.mode", "node"),
				),
			},
			{
				Config:   s.getFixture("session_recording_config_0_set.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("session_recording_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "session_recording_config"),
					resource.TestCheckResourceAttr(name, "spec.mode", "off"),
				),
			},
			{
				Config:   s.getFixture("session_recording_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportSessionRecordingConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_session_recording_config"
	id := "test_import"
	name := r + "." + id

	sessionrRecordingConfig := &types.SessionRecordingConfigV2{
		Metadata: types.Metadata{},
		Spec: types.SessionRecordingConfigSpecV2{
			Mode: "off",
		},
	}
	err := sessionrRecordingConfig.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	recordingConfigBefore, err := s.client.GetSessionRecordingConfig(ctx)
	require.NoError(s.T(), err)

	_, err = s.client.ClusterConfigClient().UpsertSessionRecordingConfig(ctx, &clusterconfigv1.UpsertSessionRecordingConfigRequest{
		SessionRecordingConfig: sessionrRecordingConfig,
	})
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		recordingConfigCurrent, err := s.client.GetSessionRecordingConfig(ctx)
		require.NoError(s.T(), err)

		return recordingConfigBefore.GetMetadata().Revision != recordingConfigCurrent.GetMetadata().Revision
	}, 5*time.Second, time.Second)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "session_recording_config", state[0].Attributes["kind"])
					require.Equal(s.T(), "off", state[0].Attributes["spec.mode"])

					return nil
				},
			},
		},
	})
}
