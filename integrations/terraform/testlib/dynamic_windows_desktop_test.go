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

func (s *TerraformSuiteOSS) TestDynamicWindowsDesktop() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_dynamic_windows_desktop.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("dynamic_windows_desktop_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "dynamic_windows_desktop"),
					resource.TestCheckResourceAttr(name, "spec.addr", "localhost:3000"),
					resource.TestCheckResourceAttr(name, "spec.non_ad", "true"),
					resource.TestCheckResourceAttr(name, "spec.domain", "my.domain"),
				),
			},
			{
				Config:   s.getFixture("dynamic_windows_desktop_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("dynamic_windows_desktop_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "dynamic_windows_desktop"),
					resource.TestCheckResourceAttr(name, "spec.addr", "localhost:3000"),
					resource.TestCheckResourceAttr(name, "spec.non_ad", "false"),
					resource.TestCheckResourceAttr(name, "spec.domain", "my.domain2"),
				),
			},
			{
				Config:   s.getFixture("dynamic_windows_desktop_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestImportDynamicWindowsDesktop() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	r := "teleport_dynamic_windows_desktop"
	id := "test_import"
	name := r + "." + id

	app, err := types.NewDynamicWindowsDesktopV1(id, nil, types.DynamicWindowsDesktopSpecV1{
		Addr: "localhost:3000",
	})
	require.NoError(s.T(), err)
	err = app.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.DynamicDesktopClient().CreateDynamicWindowsDesktop(ctx, app)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, app.GetName())
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
					require.Equal(s.T(), "dynamic_windows_desktop", state[0].Attributes["kind"])
					require.Equal(s.T(), "localhost:3000", state[0].Attributes["spec.addr"])

					return nil
				},
			},
		},
	})
}

func (s *TerraformSuiteOSSWithCache) TestDynamicWindowsDesktopWithCache() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_dynamic_windows_desktop.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("dynamic_windows_desktop_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "dynamic_windows_desktop"),
					resource.TestCheckResourceAttr(name, "spec.addr", "localhost:3000"),
					resource.TestCheckResourceAttr(name, "spec.non_ad", "true"),
					resource.TestCheckResourceAttr(name, "spec.domain", "my.domain"),
				),
			},
			{
				Config:   s.getFixture("dynamic_windows_desktop_0_create.tf"),
				PlanOnly: true,
			},
		},
	})
}
