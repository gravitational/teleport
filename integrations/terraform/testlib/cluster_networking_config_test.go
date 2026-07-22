/*
Copyright 2023 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestClusterNetworkingConfigDataSource() {
	name := "data.teleport_cluster_networking_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("networking_config_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_networking_config"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "id", "cluster-networking-config"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestClusterNetworkingConfig() {
	name := "teleport_cluster_networking_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("networking_config_0_set.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_networking_config"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.client_idle_timeout", "30m"),
				),
			},
			{
				Config:   s.getFixture("networking_config_0_set.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("networking_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "cluster_networking_config"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "no"),
					resource.TestCheckResourceAttr(name, "spec.client_idle_timeout", "1h"),
				),
			},
			{
				Config:   s.getFixture("networking_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportClusterNetworkingConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_cluster_networking_config"
	id := "test_import"
	name := r + "." + id

	clusterNetworkingConfig := &types.ClusterNetworkingConfigV2{
		Metadata: types.Metadata{},
		Spec: types.ClusterNetworkingConfigSpecV2{
			ClientIdleTimeout: types.Duration(30 * time.Second),
		},
	}
	err := clusterNetworkingConfig.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	clusterNetworkConfigBefore, err := s.client.GetClusterNetworkingConfig(ctx)
	require.NoError(s.T(), err)

	_, err = s.client.UpsertClusterNetworkingConfig(ctx, clusterNetworkingConfig)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		clusterNetworkConfigCurrent, err := s.client.GetClusterNetworkingConfig(ctx)
		require.NoError(s.T(), err)

		return clusterNetworkConfigBefore.GetMetadata().Revision != clusterNetworkConfigCurrent.GetMetadata().Revision
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
					require.Equal(s.T(), "cluster_networking_config", state[0].Attributes["kind"])
					require.Equal(s.T(), "30s", state[0].Attributes["spec.client_idle_timeout"])

					return nil
				},
			},
		},
	})
}
