/*
Copyright 2015-2025 Gravitational, Inc.

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

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccKubernetesEphemeralResource(t *testing.T) {
	const config = `
provider "teleportmwi" {
  proxy_server = "example.com:3080"
  join_method  = "gitlab"
  join_token   = "example-token"
}

ephemeral "teleportmwi_kubernetes" "example" {
  selector = {
    name = "barry"
  } 
}

provider "echo" {
  data = ephemeral.teleportmwi_kubernetes.example.output.host
}

resource "echo" "test" {}
`
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			// Ephemeral resources were introduced in Terraform 1.10.0.
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactoriesWithEcho,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"echo.test",
						tfjsonpath.New("data"),
						knownvalue.StringExact("Hello, barry!"),
					),
				},
			},
		},
	})
}
