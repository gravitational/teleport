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
	"encoding/base64"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/stretchr/testify/require"
)

func (s *TerraformSuiteOSS) TestConfigureAuthBase64() {
	name := "teleport_app.test_auth_b64"

	key, err := os.ReadFile(s.teleportConfig.ClientKey)
	require.NoError(s.T(), err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	cert, err := os.ReadFile(s.teleportConfig.ClientCrt)
	require.NoError(s.T(), err)
	certBase64 := base64.StdEncoding.EncodeToString(cert)

	rootCA, err := os.ReadFile(s.teleportConfig.RootCAs)
	require.NoError(s.T(), err)
	rootCABase64 := base64.StdEncoding.EncodeToString(rootCA)

	providerConfigUsingB64Auth := `
provider "teleport" {
	addr = "` + s.teleportConfig.Addr + `"
	key_base64 = "` + keyBase64 + `"
	cert_base64 = "` + certBase64 + `"
	root_ca_base64 = "` + rootCABase64 + `"
}
	`

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixtureWithCustomConfig("app_0_create_auth_b64.tf", providerConfigUsingB64Auth),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestConfigureAuthFiles() {
	name := "teleport_app.test_auth_files"

	providerConfigUsingAuthFiles := `
provider "teleport" {
	addr = "` + s.teleportConfig.Addr + `"
	key_path = "` + s.teleportConfig.ClientKey + `"
	cert_path = "` + s.teleportConfig.ClientCrt + `"
	root_ca_path = "` + s.teleportConfig.RootCAs + `"
}
	`

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixtureWithCustomConfig("app_0_create_auth_files.tf", providerConfigUsingAuthFiles),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestConfigureIdentityFilePath() {
	name := "teleport_app.test"

	providerConfigUsingAuthFiles := `
provider "teleport" {
	addr = "` + s.teleportConfig.Addr + `"
	identity_file_path = "` + s.teleportConfig.Identity + `"
}
	`

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixtureWithCustomConfig("app_0_create.tf", providerConfigUsingAuthFiles),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestConfigureIdentityFileBase64() {
	name := "teleport_app.test"

	identity, err := os.ReadFile(s.teleportConfig.Identity)
	require.NoError(s.T(), err)
	identityAsB64 := base64.StdEncoding.EncodeToString(identity)

	providerConfigUsingAuthFiles := `
provider "teleport" {
	addr = "` + s.teleportConfig.Addr + `"
	identity_file_base64 = "` + identityAsB64 + `"
}
	`

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixtureWithCustomConfig("app_0_create.tf", providerConfigUsingAuthFiles),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func (s *TerraformSuiteOSS) TestConfigureIdentityFileBase64_InvalidBase64() {
	identityAsB64 := base64.StdEncoding.EncodeToString([]byte("invalid"))

	providerConfigUsingAuthFiles := `
provider "teleport" {
	addr = "` + s.teleportConfig.Addr + `"
	identity_file_base64 = "` + identityAsB64 + `"
	dial_timeout_duration = "1s"
}
	`

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:      s.getFixtureWithCustomConfig("app_0_create.tf", providerConfigUsingAuthFiles),
				ExpectError: regexp.MustCompile("identity file could not be decoded"),
			},
		},
	})
}
