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
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/oidc/fakeissuer"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

const (
	kubeClusterName = "test-cluster"
	teleClusterName = "root"
)

func k8ClientConfig(serverAddr string) clientcmdapi.Config {
	return clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			kubeClusterName: {
				Server:                serverAddr,
				InsecureSkipTLSVerify: true,
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

func setupKubernetesHarness(
	t *testing.T, log *slog.Logger,
) (
	*service.TeleportProcess,
	*testingkubemock.KubeMockServer,
) {
	kubeMock := startKubeAPIMock(t)
	kubeConfigPath := mustCreateKubeConfigFile(t, k8ClientConfig(kubeMock.URL))

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithClusterName(teleClusterName),
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

			cfg.Kube.Enabled = true
			cfg.Kube.KubeconfigPath = kubeConfigPath
			cfg.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &cfg.FileDescriptors))
		}),
		testenv.WithProxyKube(),
	)
	if err != nil {
		t.Fatalf("failed to create Teleport process: %v", err)
	}

	return process, kubeMock
}

func setupKubernetesAccessBot(
	ctx context.Context,
	t *testing.T,
	client *authclient.Client,
) (*machineidv1.Bot, types.ProvisionToken) {
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
	role, err = client.CreateRole(ctx, role)
	require.NoError(t, err)
	bot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
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

	fakeJoinSigner, err := fakeissuer.NewKubernetesSigner(
		clockwork.NewRealClock(),
	)
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
	require.NoError(t, client.CreateToken(ctx, pt))

	joinJWT, err := fakeJoinSigner.SignServiceAccountJWT(
		"my-pod",
		// default:bot as in provision token above
		"default",
		"bot",
		teleClusterName,
	)
	require.NoError(t, err)
	joinJWTPath := filepath.Join(t.TempDir(), "join")
	err = os.WriteFile(joinJWTPath, []byte(joinJWT), 0666)
	require.NoError(t, err)
	t.Setenv("KUBERNETES_TOKEN_PATH", joinJWTPath)

	return bot, pt
}
