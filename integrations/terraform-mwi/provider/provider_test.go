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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"teleportmwi": providerserver.NewProtocol6WithError(New()()),
}

var testAccProtoV6ProviderFactoriesWithEcho = map[string]func() (tfprotov6.ProviderServer, error){
	"teleportmwi": providerserver.NewProtocol6WithError(New()()),
	"echo":        echoprovider.NewProviderServer(),
}

func testAccPreCheck(t *testing.T) {
	// Code to run before any acceptance test.
}

func TestProviderConfigure_SkipInitalConnection(t *testing.T) {
	t.Run("no resource", func(t *testing.T) {
		// This test verifies that Configure() succeeds without making
		// a network connection. Before this change, Configure() would
		// fail here because bot.OneShot() would try to connect to a
		// non-existent Teleport proxy.
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					// Provider is configured with a bogus address.
					// With lazy Configure, this should NOT error
					// because no resource triggers a connection.
					Config: `
provider "teleportmwi" {
  proxy_server            = "localhost:0"
  join_method             = "terraform_cloud"
  join_token              = "bogus-token"
  skip_initial_connection = true
}
`,
					// No resources declared — nothing triggers a connection.
					// Terraform should plan successfully.
					PlanOnly: true,
				},
			},
		})
	})
	t.Run("with resource", func(t *testing.T) {
		// This test verifies that when a resource IS used with invalid
		// credentials, the error surfaces at resource Open/Read time.
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: `
provider "teleportmwi" {
  proxy_server            = "localhost:0"
  join_method             = "terraform_cloud"
  join_token              = "bogus-token"
  skip_initial_connection = true
}

data "teleportmwi_kubernetes" "example" {
  selector = {
    name = "test-cluster"
  }
}
`,
					ExpectError: regexp.MustCompile(`Error (creating|running) tbot`),
				},
			},
		})
	})

}
