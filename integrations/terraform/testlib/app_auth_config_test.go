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

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types/appauthconfig"
)

func (s *TerraformSuiteOSS) TestAppAuthConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetAppAuthConfig(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_app_auth_config.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("app_auth_config_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app_auth_config"),
					resource.TestCheckResourceAttr(name, "spec.jwt.issuer", "https://issuer"),
					resource.TestCheckResourceAttr(name, "spec.jwt.audience", "teleport"),
					resource.TestCheckResourceAttr(name, "spec.jwt.jwks_url", "https://issuer/.well-known/jwks.json"),
				),
			},
			{
				Config:   s.getFixture("app_auth_config_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("app_auth_config_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app_auth_config"),
					resource.TestCheckResourceAttr(name, "spec.jwt.issuer", "https://new-issuer"),
					resource.TestCheckResourceAttr(name, "spec.jwt.audience", "new-teleport"),
					resource.TestCheckResourceAttr(name, "spec.jwt.jwks_url", "https://new-issuer/.well-known/jwks.json"),
				),
			},
			{
				Config:   s.getFixture("app_auth_config_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportAppAuthConfig() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_app_auth_config"
	id := "test_import"
	name := r + "." + id

	config := appauthconfig.NewAppAuthConfigJWT(
		id,
		[]*labelv1.Label{{Name: "*", Values: []string{"*"}}},
		&appauthconfigv1.AppAuthConfigJWTSpec{
			Audience: "teleport",
			Issuer:   "https://issuer-url",
			KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
				JwksUrl: "https://issuer-url/.well-known/jwks.json",
			},
		},
	)

	_, err := s.client.CreateAppAuthConfig(ctx, config)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetAppAuthConfig(ctx, config.Metadata.Name)
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
					require.Equal(s.T(), "app_auth_config", state[0].Attributes["kind"])
					require.Equal(s.T(), "https://issuer-url", state[0].Attributes["spec.jwt.issuer"])
					require.Equal(s.T(), "teleport", state[0].Attributes["spec.jwt.audience"])
					require.Equal(s.T(), "https://issuer-url/.well-known/jwks.json", state[0].Attributes["spec.jwt.jwks_url"])

					return nil
				},
			},
		},
	})
}
