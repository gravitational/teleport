/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

func (s *TerraformSuiteOSS) TestInstallerDataSource() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	installer, err := types.NewInstallerV1("test", "example-script")
	require.NoError(s.T(), err)

	err = s.client.SetInstaller(ctx, installer)
	require.NoError(s.T(), err)

	require.Eventually(s.T(), func() bool {
		_, err := s.client.GetInstaller(ctx, installer.GetName())
		return err == nil
	}, 5*time.Second, time.Second)

	s.T().Cleanup(func() {
		require.NoError(s.T(), s.client.DeleteInstaller(ctx, installer.GetName()))
	})

	name := "data.teleport_installer.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("installer_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "installer"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", "test"),
					resource.TestCheckResourceAttr(name, "spec.script", "example-script"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestInstaller() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkInstallerDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetInstaller(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_installer.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkInstallerDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("installer_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "installer"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "spec.script", "[Install Teleport Script]\n"),
				),
			},
			{
				Config:   s.getFixture("installer_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("installer_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "installer"),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "spec.script", "[Updated Install Teleport Script]\n"),
				),
			},
			{
				Config:   s.getFixture("installer_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
