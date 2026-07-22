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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func (s *TerraformSuiteOSS) TestUser() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkUserDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetUser(ctx, "test", false)
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_user.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkUserDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("user_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "user"),
					resource.TestCheckResourceAttr(name, "version", "v2"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2035-10-12T07:20:50Z"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "terraform-provider"),
					resource.TestCheckResourceAttr(name, "spec.traits.logins1.0", "example"),
					resource.TestCheckResourceAttr(name, "spec.traits.logins2.0", "example"),
					resource.TestCheckResourceAttr(name, "spec.oidc_identities.0.connector_id", "oidc"),
					resource.TestCheckResourceAttr(name, "spec.oidc_identities.0.username", "example"),
					resource.TestCheckResourceAttr(name, "spec.github_identities.0.connector_id", "github"),
					resource.TestCheckResourceAttr(name, "spec.github_identities.0.username", "example"),
					resource.TestCheckResourceAttr(name, "spec.saml_identities.0.connector_id", "saml"),
					resource.TestCheckResourceAttr(name, "spec.saml_identities.0.username", "example"),
				),
			},
			{
				Config:   s.getFixture("user_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("user_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "user"),
					resource.TestCheckResourceAttr(name, "metadata.expires", "2035-10-12T07:20:52Z"),
					resource.TestCheckResourceAttr(name, "spec.roles.0", "terraform-provider"),
					resource.TestCheckResourceAttr(name, "spec.traits.logins2.0", "example"),
					resource.TestCheckNoResourceAttr(name, "spec.traits.logins1"),
					resource.TestCheckResourceAttr(name, "spec.oidc_identities.0.connector_id", "oidc-2"),
					resource.TestCheckResourceAttr(name, "spec.oidc_identities.0.username", "example"),
					resource.TestCheckNoResourceAttr(name, "spec.github_identities"),
					resource.TestCheckNoResourceAttr(name, "spec.saml_identities"),
				),
			},
			{
				Config:   s.getFixture("user_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
