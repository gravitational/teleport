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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	accessmonitoringrules "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestAccessMonitoringRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetApp(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_access_monitoring_rule.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_monitoring_rule_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "access_monitoring_rule"),
					resource.TestCheckResourceAttr(name, "spec.subjects.0", "access_request"),
					resource.TestCheckResourceAttr(name, "spec.condition", "access_request.spec.roles.contains(\"your_role_name\")"),
					resource.TestCheckResourceAttr(name, "spec.notification.name", "slack"),
					resource.TestCheckResourceAttr(name, "spec.notification.recipients.0", "your-slack-channel"),
				),
			},
			{
				Config:   s.getFixture("access_monitoring_rule_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("access_monitoring_rule_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "access_monitoring_rule"),
					resource.TestCheckResourceAttr(name, "spec.subjects.0", "access_request"),
					resource.TestCheckResourceAttr(name, "spec.condition", "access_request.spec.roles.contains(\"your_other_role_name\")"),
					resource.TestCheckResourceAttr(name, "spec.notification.name", "slack"),
					resource.TestCheckResourceAttr(name, "spec.notification.recipients.0", "your-slack-channel"),
					resource.TestCheckResourceAttr(name, "spec.notification.recipients.1", "your-second-slack-channel"),
				),
			},
			{
				Config:   s.getFixture("access_monitoring_rule_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportAccessMonitoringRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_access_monitoring_rule"
	id := "test_import"
	name := r + "." + id

	condition := "access_request.spec.roles.contains(\"your_other_role_name\")"
	recipient := "your-slack-channel"
	notification := "slack"

	amr := &accessmonitoringrules.AccessMonitoringRule{
		Metadata: &v1.Metadata{
			Name: id,
		},
		Kind:    types.KindAccessMonitoringRule,
		SubKind: "",
		Version: types.V1,
		Spec: &accessmonitoringrules.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			Condition: condition,
			Notification: &accessmonitoringrules.Notification{
				Name:       "slack",
				Recipients: []string{recipient},
			},
		},
	}
	amr, err := s.client.AccessMonitoringRulesClient().CreateAccessMonitoringRule(ctx, amr)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.AccessMonitoringRulesClient().GetAccessMonitoringRule(ctx, amr.GetMetadata().GetName())
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
					require.Equal(s.T(), "access_monitoring_rule", state[0].Attributes["kind"])
					require.Equal(s.T(), types.KindAccessRequest, state[0].Attributes["spec.subjects.0"])
					require.Equal(s.T(), condition, state[0].Attributes["spec.condition"])
					require.Equal(s.T(), recipient, state[0].Attributes["spec.notification.recipients.0"])
					require.Equal(s.T(), notification, state[0].Attributes["spec.notification.name"])

					return nil
				},
			},
		},
	})
}
