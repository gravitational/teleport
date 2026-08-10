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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestProvisionToken() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2038-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.roles.1", "Auth"),

					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("provision_token_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2038-01-01T00:00:00Z"),
					resource.TestCheckNoResourceAttr(name, "metadata.labels.example"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckNoResourceAttr(name, "spec.roles.1"),

					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestProvisionTokenV2() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.test2"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_v2_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test2"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2038-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.roles.1", "Auth"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "iam"),
					resource.TestCheckResourceAttr(name, "spec.allow.0.aws_account", "1234567890"),

					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_v2_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("provision_token_v2_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test2"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2038-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.roles.1", "Auth"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "iam"),
					resource.TestCheckResourceAttr(name, "spec.allow.0.aws_account", "1234567890"),
					resource.TestCheckResourceAttr(name, "spec.allow.1.aws_account", "1111111111"),

					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_v2_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportProvisionToken() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_provision_token"
	id := "test_import"
	name := r + "." + id

	token := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{"Node", "Auth"},
		},
	}
	err := token.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.UpsertToken(ctx, token)
	require.NoError(s.T(), err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "token", state[0].Attributes["kind"])
					require.Equal(s.T(), "test_import", state[0].Attributes["metadata.name"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestProvisionTokenDoesNotLeakSensitiveData() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_secret_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "thisisasecretandmustnotbelogged"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2038-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.roles.1", "Auth"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					func(s *terraform.State) error {
						tokenResource := s.RootModule().Resources[name]
						tokenID := tokenResource.Primary.Attributes["id"]
						tokenName := tokenResource.Primary.Attributes["metadata.name"]
						if tokenID == tokenName {
							return fmt.Errorf("token id must not include the name because the name is the actual token secret")
						}

						return nil
					},
				),
			},
			{
				Config:   s.getFixture("provision_token_secret_0_create.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestProvisionTokenWithoutExpiration() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_no_expiry_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "metadata.labels.example", "yes"),
					resource.TestCheckNoResourceAttr(name, "metadata.expiry"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Node"),
					resource.TestCheckResourceAttr(name, "spec.roles.1", "Auth"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_no_expiry_0_create.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestProvisionTokenIAMToken() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "iam-token")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.iam-token"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_iam_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "iam-token"),
					resource.TestCheckNoResourceAttr(name, "metadata.expiry"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "iam"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
				),
			},
			{
				Config:   s.getFixture("provision_token_iam_create.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestProvisionTokenV2Gitlab() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkRoleDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetToken(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_provision_token.token"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkRoleDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("provision_token_v2_gitlab_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "gitlab-test-terraform"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "gitlab"),
					resource.TestCheckNoResourceAttr(name, "spec.gitlab.allow.0.environment_protected"),
					resource.TestCheckNoResourceAttr(name, "spec.gitlab.allow.0.ref_protected"),
				),
			},
			{
				Config:   s.getFixture("provision_token_v2_gitlab_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("provision_token_v2_gitlab_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "gitlab-test-terraform"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "gitlab"),
					resource.TestCheckResourceAttr(name, "spec.gitlab.allow.0.environment_protected", "true"),
					resource.TestCheckNoResourceAttr(name, "spec.gitlab.allow.0.ref_protected"),
				),
			},
			{
				Config:   s.getFixture("provision_token_v2_gitlab_1_update.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("provision_token_v2_gitlab_2_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "token"),
					resource.TestCheckResourceAttr(name, "metadata.name", "gitlab-test-terraform"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "Bot"),
					resource.TestCheckResourceAttr(name, "spec.join_method", "gitlab"),
					resource.TestCheckNoResourceAttr(name, "spec.gitlab.allow.0.environment_protected"),
					resource.TestCheckNoResourceAttr(name, "spec.gitlab.allow.0.ref_protected"),
				),
			},
			{
				Config:   s.getFixture("provision_token_v2_gitlab_2_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
