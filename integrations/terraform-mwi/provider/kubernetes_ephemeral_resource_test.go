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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integrations/lib/testing/fakejoin"
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

	fakeJoinSigner, err := fakejoin.NewKubernetesSigner(
		clockwork.NewRealClock(),
	)
	require.NoError(t, err)

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
	// TODO: Just for debugging - remove if necessary.
	kubeServers, err := rootClient.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, kubeServers, 1)

	// Create Join Token, Role and Bot
	role, err := types.NewRole("kube-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			KubeGroups: []string{
				"system:masters",
			},
			KubernetesLabels: map[string]apiutils.Strings{
				"*": {"*"},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.CreateRole(ctx, role)
	require.NoError(t, err)

	bot, err := rootClient.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Metadata: &headerv1.Metadata{
				Name: "test-bot",
			},
			Spec: &machineidv1.BotSpec{
				Roles: []string{role.GetName()},
			},
		},
	})
	require.NoError(t, err)

	marshalledJWKS, err := fakeJoinSigner.GetMarshaledJWKS()
	require.NoError(t, err)
	pt, err := types.NewProvisionTokenFromSpec(
		"test-bot",
		time.Time{},
		types.ProvisionTokenSpecV2{
			BotName:    bot.Metadata.Name,
			Roles:      []types.SystemRole{types.RoleBot},
			JoinMethod: types.JoinMethodKubernetes,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				Type: types.KubernetesJoinTypeStaticJWKS,
				StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
					JWKS: marshalledJWKS,
				},
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: "default:bot",
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, rootClient.CreateToken(ctx, pt))

	// Setup file/environment variable to point Bot to the join token.
	joinJWT, err := fakeJoinSigner.SignServiceAccountJWT(
		"my-pod",
		"default",
		"bot",
		"root",
	)
	require.NoError(t, err)
	joinJWTPath := filepath.Join(t.TempDir(), "join")
	err = os.WriteFile(joinJWTPath, []byte(joinJWT), 0666)
	require.NoError(t, err)
	require.NoError(t, os.Setenv("KUBERNETES_TOKEN_PATH", joinJWTPath))
	t.Cleanup(func() {
		require.NoError(t, os.Unsetenv("KUBERNETES_TOKEN_PATH"))
	})

	config := fmt.Sprintf(`
provider "teleportmwi" {
  proxy_server = "%s"
  join_method  = "kubernetes"
  join_token   = "test-bot"
  insecure     = true
}

ephemeral "teleportmwi_kubernetes" "example" {
  selector = {
    name = "test-cluster"
  } 
}

provider "kubernetes" {
  host                   = ephemeral.teleportmwi_kubernetes.example.output.host
  tls_server_name        = ephemeral.teleportmwi_kubernetes.example.output.tls_server_name
  client_certificate     = ephemeral.teleportmwi_kubernetes.example.output.client_certificate
  client_key             = ephemeral.teleportmwi_kubernetes.example.output.client_key
  cluster_ca_certificate = ephemeral.teleportmwi_kubernetes.example.output.cluster_ca_certificate
}

resource "kubernetes_namespace" "ns" {
  metadata {
    name = "tf-mwi-test"
  }
}
`, process.Config.Proxy.WebAddr.String())

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			// Ephemeral resources were introduced in Terraform 1.10.0.
			tfversion.SkipBelow(tfversion.Version1_10_0),
		},
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactoriesWithEcho,
		ExternalProviders: map[string]resource.ExternalProvider{
			"kubernetes": {
				Source: "hashicorp/kubernetes",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(state *terraform.State) error {
					containsCreatedNamespace := slices.ContainsFunc(
						kubeMock.ListNamespaces().Items,
						func(ns corev1.Namespace) bool {
							return ns.Name == "tf-mwi-test"
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
