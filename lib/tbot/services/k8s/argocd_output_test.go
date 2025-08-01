/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package k8s

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestArgoCDOutput_String(t *testing.T) {
	t.Parallel()

	svc := &ArgoCDOutput{
		cfg: &ArgoCDOutputConfig{
			Selectors: []*KubernetesSelector{
				{Name: "cluster-1"},
				{Name: "cluster-2"},
				{
					Labels: map[string]string{
						"env":    "prod",
						"region": "eu",
					},
				},
			},
		},
	}
	require.Equal(t, "kubernetes-argo-cd-output (name=cluster-1, name=cluster-2, labels={env=prod, region=eu})", svc.String())
}

func TestArgoCDOutput_EndToEnd(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Spin up a test server.
	process := testenv.MakeTestServer(
		t,
		defaultTestServerOpts(t, log),
		testenv.WithProxyKube(t),
		testenv.WithAuthConfig(func(auth *servicecfg.AuthConfig) {
			auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	registerCluster := func(t *testing.T, name string) {
		t.Helper()

		kubeCluster, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name:   name,
				Labels: map[string]string{"department": "engineering"},
			},
			types.KubernetesClusterSpecV3{},
		)
		require.NoError(t, err)

		kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "host", "1234")
		require.NoError(t, err)

		_, err = rootClient.UpsertKubernetesServer(ctx, kubeServer)
		require.NoError(t, err)
	}

	// Register a kubernetes cluster.
	registerCluster(t, "kube-cluster-1")

	// Create a role giving the bot access to the kubernetes cluster.
	role, err := types.NewRole("bot-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			KubernetesLabels: types.Labels{"*": []string{"*"}},
			KubeGroups:       []string{"system:masters"},
			KubeUsers:        []string{"kubernetes-user"},
		},
	})
	require.NoError(t, err)

	_, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Create the service.
	k8s := fake.NewClientset()
	service := ArgoCDServiceBuilder(
		&ArgoCDOutputConfig{
			SecretNamePrefix: "my-cluster",
			SecretNamespace:  "argocd",
			SecretLabels: map[string]string{
				"team": "billing",
			},
			Selectors: []*KubernetesSelector{
				{Labels: map[string]string{"department": "engineering"}},
			},
			Project:          "my-argo-project",
			Namespaces:       []string{"prod", "dev"},
			ClusterResources: true,
		},
		WithKubernetesClient(k8s),
	)

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	onboarding, _ := makeBot(t, rootClient, "argo-bot", role.GetName())

	botConfig := bot.Config{
		InternalStorage: destination.NewMemory(),
		Connection: connection.Config{
			Address:     proxyAddr.Addr,
			AddressKind: connection.AddressKindProxy,
			Insecure:    true,
		},
		Logger:     log,
		Onboarding: *onboarding,
		Services:   []bot.ServiceBuilder{service},
	}

	// Run the bot in one-shot mode.
	b, err := bot.New(botConfig)
	require.NoError(t, err)
	require.NoError(t, b.OneShot(ctx))

	// Expect the cluster credentials to have been written to a secret.
	list, err := k8s.CoreV1().
		Secrets("argocd").
		List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	secret := list.Items[0]

	// Check we apply the secret name prefix.
	require.True(t, strings.HasPrefix(secret.Name, "my-cluster"))

	// Check we set the correct labels on the secret.
	require.Equal(t, "cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	require.Equal(t, "billing", secret.Labels["team"])

	// Check the user-facing name given to the cluster.
	name := string(secret.Data["name"])
	require.Equal(t, "root-kube-cluster-1", name)

	// Check the server addr.
	server := string(secret.Data["server"])
	serverURL, err := url.Parse(server)
	require.NoError(t, err)

	_, port, _ := strings.Cut(proxyAddr.Addr, ":")
	require.Equal(t, port, serverURL.Port())
	require.Equal(t, "/v1/teleport/cm9vdA/a3ViZS1jbHVzdGVyLTE", serverURL.Path)

	// Check the optional fields.
	require.Equal(t, "my-argo-project", string(secret.Data["project"]))
	require.Equal(t, "prod,dev", string(secret.Data["namespaces"]))
	require.Equal(t, "true", string(secret.Data["clusterResources"]))

	// Check the config.
	var config map[string]any
	require.NoError(t, json.Unmarshal(secret.Data["config"], &config))

	tlsConfig := config["tlsClientConfig"].(map[string]any)
	require.Equal(t, "kube-teleport-proxy-alpn.teleport.cluster.local", tlsConfig["serverName"])
	require.NotEmpty(t, tlsConfig["caData"])
	require.NotEmpty(t, tlsConfig["certData"])
	require.NotEmpty(t, tlsConfig["keyData"])

	require.Equal(t, "root", secret.Annotations["teleport.dev/teleport-cluster-name"])
	require.Equal(t, "argo-bot", secret.Annotations["teleport.dev/bot-name"])
	require.Equal(t, "kube-cluster-1", secret.Annotations["teleport.dev/kubernetes-cluster-name"])

	// Add another cluster and run the bot again.
	registerCluster(t, "kube-cluster-2")

	b, err = bot.New(botConfig)
	require.NoError(t, err)
	require.NoError(t, b.OneShot(ctx))

	// Expect another secret to have been written.
	list, err = k8s.CoreV1().
		Secrets("argocd").
		List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 2)
}
