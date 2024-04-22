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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteEnterprise) TestLoginRule() {
	require.True(s.T(),
		s.teleportFeatures.GetOIDC() || s.teleportFeatures.GetSAML(),
		"Test requires enterprise version of teleport",
	)

	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	expressionRuleName := "teleport_login_rule.expression_rule"
	mapRuleName := "teleport_login_rule.map_rule"
	allRules := []string{expressionRuleName, mapRuleName}

	checkDestroyed := func(state *terraform.State) error {
		for _, ruleName := range allRules {
			_, err := s.client.GetLoginRule(ctx, ruleName)
			switch {
			case err == nil:
				return fmt.Errorf("login rule %s was not deleted", ruleName)
			case trace.IsNotFound(err):
				continue
			default:
				return err
			}
		}
		return nil
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("login_rule_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(expressionRuleName, "metadata.name", "expression_rule"),
					resource.TestCheckResourceAttr(expressionRuleName, "metadata.labels.env", "test"),
					resource.TestCheckResourceAttr(expressionRuleName, "version", "v1"),
					resource.TestCheckResourceAttr(expressionRuleName, "priority", "1"),
					resource.TestCheckResourceAttr(expressionRuleName, "traits_expression", "external"),
					resource.TestCheckNoResourceAttr(expressionRuleName, "traits_map"),
					resource.TestCheckResourceAttr(mapRuleName, "metadata.name", "map_rule"),
					resource.TestCheckResourceAttr(mapRuleName, "metadata.labels.env", "test"),
					resource.TestCheckResourceAttr(mapRuleName, "version", "v1"),
					resource.TestCheckResourceAttr(mapRuleName, "priority", "2"),
					resource.TestCheckResourceAttr(mapRuleName, "traits_map.logins.values.0", "external.logins"),
					resource.TestCheckResourceAttr(mapRuleName, "traits_map.logins.values.1", "external.username"),
					resource.TestCheckNoResourceAttr(mapRuleName, "traits_expression"),
				),
			},
			{
				Config:   s.getFixture("login_rule_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("login_rule_0_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(expressionRuleName, "metadata.name", "expression_rule"),
					resource.TestCheckResourceAttr(expressionRuleName, "metadata.labels.env", "test"),
					resource.TestCheckResourceAttr(expressionRuleName, "version", "v1"),
					resource.TestCheckResourceAttr(expressionRuleName, "priority", "1"),
					resource.TestCheckResourceAttr(expressionRuleName, "traits_expression", `external.put("logins", external.logins.add("external.username"))`),
					resource.TestCheckNoResourceAttr(expressionRuleName, "traits_map"),
					resource.TestCheckResourceAttr(mapRuleName, "metadata.name", "map_rule"),
					resource.TestCheckResourceAttr(mapRuleName, "metadata.labels.env", "test"),
					resource.TestCheckResourceAttr(mapRuleName, "version", "v1"),
					resource.TestCheckResourceAttr(mapRuleName, "priority", "2"),
					resource.TestCheckResourceAttr(mapRuleName, "traits_map.kube_groups.values.0", `"system:masters"`),
					resource.TestCheckNoResourceAttr(mapRuleName, "traits_map.logins"),
					resource.TestCheckNoResourceAttr(mapRuleName, "traits_expression"),
				),
			},
			{
				Config:   s.getFixture("login_rule_0_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportLoginRule() {
	require.True(s.T(),
		s.teleportFeatures.GetOIDC() || s.teleportFeatures.GetSAML(),
		"Test requires OIDC or SAML",
	)

	ctx := context.Background()

	r := "teleport_login_rule"
	id := "test_import"
	name := r + "." + id

	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: id,
		},
		Version:          types.V1,
		Priority:         1,
		TraitsExpression: "external",
	}

	_, err := s.client.CreateLoginRule(ctx, rule)
	s.Require().NoError(err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					s.Require().Equal(id, state[0].Attributes["metadata.name"])
					s.Require().Equal("1", state[0].Attributes["priority"])
					s.Require().Equal(rule.TraitsExpression, state[0].Attributes["traits_expression"])
					return nil
				},
			},
		},
	})
}
