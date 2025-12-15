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
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
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

	// Test starting with old schema.
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
					resource.TestCheckResourceAttr(botName, "traits.logins1.0", "example"),
				),
			},
			{
				Config:   s.getFixture("bot_1_update.tf"),
				PlanOnly: true,
			},

			// Switch to new schema (same resource attributes).
			{
				Config: s.getFixture("bot_2_new_schema.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "metadata.name", "test"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "spec.roles.0", "terraform"),
					resource.TestCheckResourceAttr(botName, "spec.traits.logins1.0", "example"),
				),
			},
			{
				Config:   s.getFixture("bot_2_new_schema.tf"),
				PlanOnly: true,
			},

			// Update with the new schema.
			{
				Config: s.getFixture("bot_3_new_schema_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "metadata.name", "test"),
					resource.TestCheckResourceAttr(botName, "metadata.labels.team", "engineering"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "spec.roles.1", "deployer"),
					resource.TestCheckResourceAttr(botName, "spec.max_session_ttl", "1h"),
				),
			},
			{
				Config:   s.getFixture("bot_3_new_schema_update.tf"),
				PlanOnly: true,
			},

			// Test reverting back to the old schema.
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
					resource.TestCheckResourceAttr(botName, "traits.logins1.0", "example"),
				),
			},
			{
				Config:   s.getFixture("bot_1_update.tf"),
				PlanOnly: true,
			},
		},
	})

	// Test with only new schema.
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkResourcesDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("bot_2_new_schema.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "metadata.name", "test"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "spec.roles.0", "terraform"),
					resource.TestCheckResourceAttr(botName, "spec.traits.logins1.0", "example"),
				),
			},
			{
				Config:   s.getFixture("bot_2_new_schema.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("bot_3_new_schema_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tokenName, "kind", "token"),
					resource.TestCheckResourceAttr(tokenName, "metadata.name", "bot-test"),
					resource.TestCheckResourceAttr(tokenName, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(botName, "metadata.name", "test"),
					resource.TestCheckResourceAttr(botName, "metadata.labels.team", "engineering"),
					resource.TestCheckResourceAttr(botName, "user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.user_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "status.role_name", "bot-test"),
					resource.TestCheckResourceAttr(botName, "spec.roles.1", "deployer"),
					resource.TestCheckResourceAttr(botName, "spec.max_session_ttl", "1h"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportBot() {
	t := s.T()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := "teleport_bot"
	id := "test_import"
	name := r + "." + id

	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{"deployer"},
			Traits: []*machineidv1.Trait{
				{
					Name:   "logins",
					Values: []string{"root", "ubuntu"},
				},
			},
			MaxSessionTtl: durationpb.New(5 * time.Minute),
		},
	}
	bot, err := s.client.BotServiceClient().
		CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: bot})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := s.client.BotServiceClient().
			GetBot(ctx, &machineidv1.GetBotRequest{BotName: bot.Metadata.Name})
		return err == nil
	}, 5*time.Second, time.Second)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config:        fmt.Sprintf("%s\nresource %q %q { }", s.terraformConfig, r, id),
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					assert.Equal(t, types.KindBot, state[0].Attributes["kind"])
					assert.Equal(t, "5m0s", state[0].Attributes["spec.max_session_ttl"])
					assert.Equal(t, "deployer", state[0].Attributes["spec.roles.0"])
					assert.Equal(t, "root", state[0].Attributes["spec.traits.logins.0"])

					return nil
				},
			},
		},
	})
}
