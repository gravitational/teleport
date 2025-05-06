// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testlib

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteEnterprise) TestOktaImportRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.OktaClient().GetOktaImportRule(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_okta_import_rule.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("okta_import_rule_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "okta_import_rule"),
					resource.TestCheckResourceAttr(name, "spec.priority", "100"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.add_labels.label1", "value1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.0", "1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.1", "2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.2", "3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.add_labels.label2", "value2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.0", "1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.1", "2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.2", "3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.add_labels.label3", "value3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.match.0.group_name_regexes.0", "^.*$"),
					resource.TestCheckResourceAttr(name, "spec.mappings.3.add_labels.label4", "value4"),
					resource.TestCheckResourceAttr(name, "spec.mappings.3.match.0.app_name_regexes.0", "^.*$"),
				),
			},
			{
				Config:   s.getFixture("okta_import_rule_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("okta_import_rule_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "okta_import_rule"),
					resource.TestCheckResourceAttr(name, "spec.priority", "100"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.add_labels.label1", "value1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.0", "1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.1", "2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.0.match.0.app_ids.2", "3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.add_labels.label2", "value2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.0", "1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.1", "2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.1.match.0.group_ids.2", "3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.add_labels.label3", "value3"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.match.0.group_ids.0", "1"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.match.0.group_ids.1", "2"),
					resource.TestCheckResourceAttr(name, "spec.mappings.2.match.0.group_ids.2", "3"),
				),
			},
			{
				Config:   s.getFixture("okta_import_rule_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportOktaImportRule() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_okta_import_rule"
	id := "test_import"
	name := r + "." + id

	oir := &types.OktaImportRuleV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: id,
			},
		},
		Spec: types.OktaImportRuleSpecV1{
			Priority: 100,
			Mappings: []*types.OktaImportRuleMappingV1{
				{
					AddLabels: map[string]string{
						"label1": "value1",
					},
					Match: []*types.OktaImportRuleMatchV1{
						{
							AppIDs: []string{"1", "2", "3"},
						},
					},
				},
				{
					AddLabels: map[string]string{
						"label2": "value2",
					},
					Match: []*types.OktaImportRuleMatchV1{
						{
							GroupIDs: []string{"1", "2", "3"},
						},
					},
				},
			},
		},
	}
	err := oir.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.OktaClient().CreateOktaImportRule(ctx, oir)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.OktaClient().GetOktaImportRule(ctx, oir.GetName())
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(s.T(), err)
		return true
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
					require.Equal(s.T(), "okta_import_rule", state[0].Attributes["kind"])

					return nil
				},
			},
		},
	})
}
