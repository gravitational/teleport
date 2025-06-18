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
	"context"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/integration/helpers"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

const kubeClusterName = "test-cluster"

func k8ClientConfig(serverAddr, sni string) clientcmdapi.Config {
	return clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			kubeClusterName: {
				Server:                serverAddr,
				InsecureSkipTLSVerify: true,
				TLSServerName:         sni,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			kubeClusterName: {
				Cluster:  kubeClusterName,
				AuthInfo: kubeClusterName,
			},
		},
		CurrentContext: kubeClusterName,
	}
}

func startKubeAPIMock(t *testing.T) *testingkubemock.KubeMockServer {
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })
	return kubeMock
}

func mustCreateKubeConfigFile(t *testing.T, config clientcmdapi.Config) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := clientcmd.WriteToFile(config, configPath)
	require.NoError(t, err)
	return configPath
}

const localK8SSNI = constants.KubeTeleportProxyALPNPrefix + "teleport.cluster.local"

func TestAccKubernetesEphemeralResource(t *testing.T) {
	log := utils.NewSlogLoggerForTests()
	ctx := context.Background()

	kubeMock := startKubeAPIMock(t)
	kubeConfigPath := mustCreateKubeConfigFile(t, k8ClientConfig(
		kubeMock.URL,
		localK8SSNI,
	))

	process := testenv.MakeTestServer(
		t,
		func(o *testenv.TestServersOpts) {
			testenv.WithClusterName(t, "root")(o)
			testenv.WithConfig(func(cfg *servicecfg.Config) {
				cfg.Logger = log
				cfg.Proxy.PublicAddrs = []utils.NetAddr{
					{
						AddrNetwork: "tcp",
						Addr: net.JoinHostPort(
							"localhost",
							strconv.Itoa(cfg.Proxy.WebAddr.Port(0)),
						),
					},
				}
				cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{
					cfg.Proxy.ReverseTunnelListenAddr,
				}

				// Seemingly not needed.
				//cfg.Proxy.Kube.Enabled = true
				//cfg.Proxy.Kube.ListenAddr = utils.NetAddr{
				//	AddrNetwork: "tcp",
				//	Addr: testenv.NewTCPListener(
				//		t, service.ListenerProxyKube, &cfg.FileDescriptors,
				//	),
				//}

				cfg.Kube.Enabled = true
				cfg.Kube.KubeconfigPath = kubeConfigPath
				cfg.Kube.ListenAddr = utils.MustParseAddr(
					helpers.NewListener(t, service.ListenerKube, &cfg.FileDescriptors))
			})(o)
		},
		testenv.WithProxyKube(t),
	)
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	time.Sleep(10 * time.Second)
	kubeServers, err := rootClient.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, kubeServers, 1)

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
