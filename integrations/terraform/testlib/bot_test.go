/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func (s *TerraformSuiteOSS) TestBot() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkResourcesDestroyed := func(state *terraform.State) error {
		var errs []error
		if _, err := s.client.GetToken(ctx, "bot-test"); err != nil {
			if !trace.IsNotFound(err) {
				errs = append(errs, err)
			}
		}

		if _, err := s.client.GetUser(ctx, "bot-test", false); err != nil {
			if !trace.IsNotFound(err) {
				errs = append(errs, err)
			}
		}

		return trace.NewAggregate(errs...)
	}

	tokenName := "teleport_provision_token.bot_test"
	botName := "teleport_bot.test"
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkResourcesDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("bot_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "name", "test"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "token_id", "bot-test"),
					resource.TestCheckResourceAttr(botName, "roles.0", "terraform"),
					resource.TestCheckNoResourceAttr(botName, "spec.traits.logins1"),
				),
			},
			{
				Config:   s.getFixture("bot_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("bot_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "name", "test"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "token_id", "bot-test"),
					resource.TestCheckResourceAttr(botName, "roles.0", "terraform"),

					// Note: traits are immutable and the plan will not converge
					// if the resource is not recreated when traits are
					// modified.
					resource.TestCheckResourceAttr(botName, "traits.logins1.0", "example"),
				),
			},
			{
				Config:   s.getFixture("bot_1_update.tf"),
				PlanOnly: true,
			},
		},
	})

}
