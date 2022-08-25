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

package resources

import (
	"context"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/service"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration"
	"github.com/gravitational/teleport/integration/helpers"
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
	//+kubebuilder:scaffold:imports
)

func fastEventually(t *testing.T, condition func() bool) {
	require.Eventually(t, condition, time.Second, 100*time.Millisecond)
}

func clientForTeleport(t *testing.T, teleportServer *helpers.TeleInstance, userName string) auth.ClientI {
	identityFilePath := integration.MustCreateUserIdentityFile(t, teleportServer, userName, time.Hour)
	id, err := identityfile.ReadFile(identityFilePath)
	require.NoError(t, err)
	addr, err := utils.ParseAddr(teleportServer.Auth)
	require.NoError(t, err)
	tlsConfig, err := id.TLSConfig()
	require.NoError(t, err)
	sshConfig, err := id.SSHClientConfig()
	require.NoError(t, err)
	authClientConfig := &authclient.Config{
		TLS:                  tlsConfig,
		SSH:                  sshConfig,
		AuthServers:          []utils.NetAddr{*addr},
		Log:                  logrus.StandardLogger(),
		CircuitBreakerConfig: breaker.Config{},
	}

	c, err := authclient.Connect(context.Background(), authClientConfig)
	require.NoError(t, err)

	return c
}

func defaultTeleportServiceConfig(t *testing.T) (*helpers.TeleInstance, string) {
	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    integration.Loopback,
	})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Version = "v2"

	roleName := validRandomResourceName("role-")
	unrestricted := []string{"list", "create", "read", "update", "delete"}
	role, err := types.NewRole(roleName, types.RoleSpecV5{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("role", unrestricted),
				types.NewRule("user", unrestricted),
			},
		},
	})
	require.NoError(t, err)

	operatorName := validRandomResourceName("operator-")
	_ = teleportServer.AddUserWithRole(operatorName, role)

	err = teleportServer.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	return teleportServer, operatorName
}

func startKubernetesOperator(t *testing.T, teleportClient auth.ClientI) kclient.Client {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = resourcesv5.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = resourcesv2.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	k8sClient, err := kclient.New(cfg, kclient.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	clientAccessor := func(ctx context.Context) (auth.ClientI, error) {
		return teleportClient, nil
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	require.NoError(t, err)

	err = (&RoleReconciler{
		Client:                 k8sClient,
		Scheme:                 k8sManager.GetScheme(),
		TeleportClientAccessor: clientAccessor,
	}).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = (&UserReconciler{
		Client:                 k8sClient,
		Scheme:                 k8sManager.GetScheme(),
		TeleportClientAccessor: clientAccessor,
	}).SetupWithManager(k8sManager)
	require.NoError(t, err)

	ctx, ctxCancel := context.WithCancel(context.Background())
	go func() {
		err = k8sManager.Start(ctx)
		require.NoError(t, err)
	}()

	t.Cleanup(func() {
		ctxCancel()
		err = testEnv.Stop()
		require.NoError(t, err)
	})

	return k8sClient
}

func createNamespaceForTest(t *testing.T, kc kclient.Client) *core.Namespace {
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: validRandomResourceName("ns-")},
	}

	err := kc.Create(context.Background(), ns)
	require.NoError(t, err)

	t.Cleanup(func() {
		deleteNamespaceForTest(t, kc, ns)
	})

	return ns
}

func deleteNamespaceForTest(t *testing.T, kc kclient.Client, ns *core.Namespace) {
	err := kc.Delete(context.Background(), ns)
	require.NoError(t, err)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func validRandomResourceName(prefix string) string {
	b := make([]rune, 5)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return prefix + string(b)
}
