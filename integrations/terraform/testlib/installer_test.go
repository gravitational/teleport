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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

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
