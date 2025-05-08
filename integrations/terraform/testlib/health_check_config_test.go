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
	"google.golang.org/protobuf/types/known/durationpb"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/services"
)

func (s *TerraformSuiteOSS) TestHealthCheckConfig() {
	t := s.T()
	name := "teleport_health_check_config.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories:  s.terraformProviders,
		PreventPostDestroyRefresh: true,
		IsUnitTest:                true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("health_check_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "health_check_config"),
					resource.TestCheckResourceAttr(name, "spec.interval", "60s"),
					resource.TestCheckResourceAttr(name, "spec.timeout", "5s"),
					resource.TestCheckResourceAttr(name, "spec.healthy_threshold", "3"),
					resource.TestCheckResourceAttr(name, "spec.unhealthy_threshold", "2"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels.0.name", "inEnv"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels.0.values.0", "foo"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels.0.values.1", "bar"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels_expression", "labels.foo == `bar`"),
				),
			},
			{
				Config:   s.getFixture("health_check_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("health_check_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "health_check_config"),
					resource.TestCheckResourceAttr(name, "spec.interval", "45s"),
					resource.TestCheckResourceAttr(name, "spec.timeout", "7s"),
					resource.TestCheckResourceAttr(name, "spec.healthy_threshold", "2"),
					resource.TestCheckResourceAttr(name, "spec.unhealthy_threshold", "1"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels.0.name", "env"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels.0.values.0", "prod"),
					resource.TestCheckResourceAttr(name, "spec.match.db_labels_expression", "labels.foo == `baz`"),
				),
			},
			{
				Config:   s.getFixture("health_check_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportHealthCheckConfig() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := "teleport_health_check_config"
	id := "test_import"
	name := r + "." + id

	healthCfg, err := healthcheckconfig.NewHealthCheckConfig(id,
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Interval:         durationpb.New(42 * time.Second),
			HealthyThreshold: 3,
			// We intentionally don't set Timeout and UnhealthyThreshold to test a partial resource
			Match: &healthcheckconfigv1.Matcher{
				DbLabels: []*labelv1.Label{{
					Name:   "env",
					Values: []string{"one", "two"},
				}},
				DbLabelsExpression: "labels.env == `one`",
			},
		},
	)
	require.NoError(t, err)
	healthCfg.Metadata.Labels = map[string]string{
		"purpose": "test",
	}

	w, err := services.NewHealthCheckConfigWatcher(ctx, services.HealthCheckConfigWatcherConfig{
		Reader:     s.client,
		ResourcesC: make(chan []*healthcheckconfigv1.HealthCheckConfig, 10),
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 100 * time.Millisecond,
			Client:         s.client,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	require.NoError(t, w.WaitInitialization())

	waitForResources := func() []*healthcheckconfigv1.HealthCheckConfig {
		select {
		case resources := <-w.ResourcesC:
			return resources
		case <-w.Done():
			require.FailNow(t, "Watcher has unexpectedly exited.")
		case <-time.After(2 * time.Second):
			require.FailNow(t, "Timeout waiting for the first event.")
		}
		return nil
	}
	got := waitForResources()
	require.Empty(t, got)

	_, err = s.client.CreateHealthCheckConfig(ctx, healthCfg)
	require.NoError(t, err)

	got = waitForResources()
	require.Len(t, got, 1)
	existing := got[0]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(t, "health_check_config", state[0].Attributes["kind"])
					require.Equal(t, existing.Metadata.Name, state[0].Attributes["metadata.name"])
					require.Equal(t, existing.Metadata.Namespace, state[0].Attributes["metadata.namespace"])
					require.Equal(t, existing.Metadata.Description, state[0].Attributes["metadata.description"])
					require.Equal(t, "test", state[0].Attributes["metadata.labels.purpose"])
					require.Equal(t, existing.Metadata.Revision, state[0].Attributes["metadata.revision"])
					require.Equal(t, "42s", state[0].Attributes["spec.interval"])
					require.Empty(t, state[0].Attributes["spec.timeout"])
					require.Equal(t, "3", state[0].Attributes["spec.healthy_threshold"])
					require.Empty(t, state[0].Attributes["spec.unhealthy_threshold"])
					require.Equal(t, "env", state[0].Attributes["spec.match.db_labels.0.name"])
					require.Equal(t, "one", state[0].Attributes["spec.match.db_labels.0.values.0"])
					require.Equal(t, "two", state[0].Attributes["spec.match.db_labels.0.values.1"])
					require.Equal(t, "labels.env == `one`", state[0].Attributes["spec.match.db_labels_expression"])

					return nil
				},
			},
		},
	})
}
