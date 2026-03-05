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

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteOSS) TestAppDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	app := &types.AppV3{
		Metadata: types.Metadata{
			Name: "test",
		},
		Spec: types.AppSpecV3{
			URI:        "localhost:3000",
			PublicAddr: "example.teleport.sh:443",
			Rewrite: &types.Rewrite{
				Redirect: []string{"example.teleport.sh"},
				Headers: []*types.Header{
					{
						Name:  "X-Custom-Header",
						Value: "value",
					},
				},
			},
		},
	}
	err := app.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.CreateApp(ctx, app)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetApp(ctx, app.GetName())
		return err == nil
	}, 5*time.Second, time.Second)

	s.T().Cleanup(func() {
		require.NoError(s.T(), s.client.DeleteApp(ctx, app.GetName()))
	})

	name := "data.teleport_app.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("app_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "version", "v3"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
					resource.TestCheckResourceAttr(name, "spec.public_addr", "example.teleport.sh:443"),
					resource.TestCheckResourceAttr(name, "spec.rewrite.redirect.0", "example.teleport.sh"),
					resource.TestCheckResourceAttr(name, "spec.rewrite.headers.0.name", "X-Custom-Header"),
					resource.TestCheckResourceAttr(name, "spec.rewrite.headers.0.value", "value"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestApp() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetApp(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_app.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("app_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
			{
				Config:   s.getFixture("app_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("app_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
			{
				Config:   s.getFixture("app_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportApp() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_app"
	id := "test_import"
	name := r + "." + id

	app := &types.AppV3{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.AppSpecV3{
			URI: "localhost:3000/test",
		},
	}
	err := app.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	err = s.client.CreateApp(ctx, app)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetApp(ctx, app.GetName())
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
					require.Equal(s.T(), "app", state[0].Attributes["kind"])
					require.Equal(s.T(), "localhost:3000/test", state[0].Attributes["spec.uri"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSWithCache) TestAppWithCache() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)
	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetApp(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_app.test_with_cache"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("app_0_create_with_cache.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
			{
				Config:   s.getFixture("app_0_create_with_cache.tf"),
				PlanOnly: true,
			},
		},
	})
}
