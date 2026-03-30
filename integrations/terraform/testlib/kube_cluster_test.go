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
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestKubernetesCluster() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetKubernetesCluster(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_kube_cluster.test"

	kubeconfig0, err := os.ReadFile("./fixtures/kubeconfig-0.yaml")
	require.NoError(s.T(), err)
	kubeconfig1, err := os.ReadFile("./fixtures/kubeconfig-1.yaml")
	require.NoError(s.T(), err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("kube_cluster_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "kube_cluster"),
					resource.TestCheckResourceAttr(name, "spec.kubeconfig", string(kubeconfig0)),
				),
			},
			{
				Config:   s.getFixture("kube_cluster_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("kube_cluster_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "kube_cluster"),
					resource.TestCheckResourceAttr(name, "spec.kubeconfig", string(kubeconfig1)),
				),
			},
			{
				Config:   s.getFixture("kube_cluster_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportKubernetesCluster() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_kube_cluster"
	id := "test_import"
	name := r + "." + id

	kubeconfig0, err := os.ReadFile("./fixtures/kubeconfig-0.yaml")
	require.NoError(s.T(), err)

	cluster := &types.KubernetesClusterV3{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.KubernetesClusterSpecV3{
			Kubeconfig: kubeconfig0,
		},
	}
	err = cluster.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.CreateKubernetesCluster(ctx, cluster)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetKubernetesCluster(ctx, cluster.GetName())
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
					require.Equal(s.T(), "kube_cluster", state[0].Attributes["kind"])
					require.Equal(s.T(), string(kubeconfig0), state[0].Attributes["spec.kubeconfig"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSWithCache) TestKubernetesClusterWithCache() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetKubernetesCluster(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}
	kubeconfig0, err := os.ReadFile("./fixtures/kubeconfig-0.yaml")
	require.NoError(s.T(), err)

	name := "teleport_kube_cluster.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("kube_cluster_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "kube_cluster"),
					resource.TestCheckResourceAttr(name, "spec.kubeconfig", string(kubeconfig0)),
				),
			},
			{
				Config:   s.getFixture("kube_cluster_0_create.tf"),
				PlanOnly: true,
			},
		},
	})
}
