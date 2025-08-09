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
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	corev1 "k8s.io/api/core/v1"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestAccKubernetesDataSource(t *testing.T) {
	log := logtest.NewLogger()
	ctx := t.Context()

	process, kubeMock := setupKubernetesHarness(t, log)
	rootClient, err := testenv.NewDefaultAuthClient(process)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}
	t.Cleanup(func() { _ = rootClient.Close() })

	_, pt := setupKubernetesAccessBot(ctx, t, rootClient)

	config := fmt.Sprintf(`
provider "teleportmwi" {
  proxy_server = "%[1]s"
  join_method  = "kubernetes"
  join_token   = "%[2]s"
  insecure     = true
}

data "teleportmwi_kubernetes" "example" {
  selector = {
    name = "%[3]s"
  } 
}

provider "kubernetes" {
  host                   = data.teleportmwi_kubernetes.example.output.host
  tls_server_name        = data.teleportmwi_kubernetes.example.output.tls_server_name
  client_certificate     = data.teleportmwi_kubernetes.example.output.client_certificate
  client_key             = data.teleportmwi_kubernetes.example.output.client_key
  cluster_ca_certificate = data.teleportmwi_kubernetes.example.output.cluster_ca_certificate
}

resource "kubernetes_namespace" "ns" {
  metadata {
    name = "tf-mwi-ds-test"
  }
}
`, process.Config.Proxy.WebAddr.String(), pt.GetName(), kubeClusterName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"kubernetes": {
				Source: "hashicorp/kubernetes",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(state *terraform.State) error {
					// Check that the namespace was actually created!
					containsCreatedNamespace := slices.ContainsFunc(
						kubeMock.ListNamespaces().Items,
						func(ns corev1.Namespace) bool {
							return ns.Name == "tf-mwi-ds-test"
						},
					)
					if !containsCreatedNamespace {
						return fmt.Errorf("expected to find created namespace in TF state")
					}
					return nil
				},
			},
		},
	})
}
