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

//nolint:gci // Remove when GCI is fixed upstream https://github.com/daixiang0/gci/issues/135
package resources

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func fastEventually(t *testing.T, condition func() bool) {
	require.Eventually(t, condition, time.Second, 100*time.Millisecond)
}

func clientForTeleport(t *testing.T, teleportServer *helpers.TeleInstance, userName string) *client.Client {
	identityFilePath := helpers.MustCreateUserIdentityFile(t, teleportServer, userName, time.Hour)
	creds := client.LoadIdentityFile(identityFilePath)
	return clientWithCreds(t, teleportServer.Auth, creds)
}

func clientWithCreds(t *testing.T, authAddr string, creds client.Credentials) *client.Client {
	c, err := client.New(context.Background(), client.Config{
		Addrs:       []string{authAddr},
		Credentials: []client.Credentials{creds},
	})
	require.NoError(t, err)
	return c
}

func defaultTeleportServiceConfig(t *testing.T) (*helpers.TeleInstance, string) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			OIDC: true,
			SAML: true,
		},
	})

	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Log:         logrus.StandardLogger(),
	})

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Version = "v2"

	roleName := validRandomResourceName("role-")
	unrestricted := []string{"list", "create", "read", "update", "delete"}
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule("role", unrestricted),
				types.NewRule("user", unrestricted),
				types.NewRule("auth_connector", unrestricted),
				types.NewRule("login_rule", unrestricted),
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

// startKubernetesOperator creates and start a new operator
func (s *testSetup) startKubernetesOperator(t *testing.T) {
	// If there was an operator running previously we make sure it is stopped
	if s.operatorCancel != nil {
		s.stopKubernetesOperator()
	}

	// We have to create a new Manager on each start because the Manager does not support to be restarted
	clientAccessor := func(ctx context.Context) (*client.Client, error) {
		return s.tClient, nil
	}

	k8sManager, err := ctrl.NewManager(s.k8sRestConfig, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	require.NoError(t, err)

	err = (&RoleReconciler{
		Client:                 s.k8sClient,
		Scheme:                 k8sManager.GetScheme(),
		TeleportClientAccessor: clientAccessor,
	}).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = NewUserReconciler(s.k8sClient, clientAccessor).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = NewGithubConnectorReconciler(s.k8sClient, clientAccessor).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = NewOIDCConnectorReconciler(s.k8sClient, clientAccessor).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = NewSAMLConnectorReconciler(s.k8sClient, clientAccessor).SetupWithManager(k8sManager)
	require.NoError(t, err)

	err = NewLoginRuleReconciler(s.k8sClient, clientAccessor).SetupWithManager(k8sManager)
	require.NoError(t, err)

	ctx, ctxCancel := context.WithCancel(context.Background())

	s.operator = k8sManager
	s.operatorCancel = ctxCancel

	go func() {
		err := s.operator.Start(ctx)
		assert.NoError(t, err)
	}()
}

func (s *testSetup) stopKubernetesOperator() {
	s.operatorCancel()
}

func createNamespaceForTest(t *testing.T, kc kclient.Client) *core.Namespace {
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: validRandomResourceName("ns-")},
	}

	err := kc.Create(context.Background(), ns)
	require.NoError(t, err)

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

type testSetup struct {
	tClient        *client.Client
	k8sClient      kclient.Client
	k8sRestConfig  *rest.Config
	namespace      *core.Namespace
	operator       manager.Manager
	operatorCancel context.CancelFunc
	operatorName   string
}

// setupTeleportClient creates and runs a Teleport server, returning a connected
// client and the username of the client identity.
func setupTeleportClient(t *testing.T) (tClient *client.Client, operatorName string) {
	if addr := os.Getenv("OPERATOR_TEST_TELEPORT_ADDR"); addr != "" {
		creds := client.LoadProfile("", "")
		tClient = clientWithCreds(t, addr, creds)
		return tClient, ""
	}

	teleportServer, operatorName := defaultTeleportServiceConfig(t)
	require.NoError(t, teleportServer.Start())
	tClient = clientForTeleport(t, teleportServer, operatorName)
	t.Cleanup(func() {
		err := teleportServer.StopAll()
		require.NoError(t, err)
	})

	t.Cleanup(func() {
		err := tClient.Close()
		require.NoError(t, err)
	})

	return tClient, operatorName
}

// setupTestEnv creates a Kubernetes server, a teleport server and starts the operator
func setupTestEnv(t *testing.T) *testSetup {
	tClient, operatorName := setupTeleportClient(t)

	// Create a Kubernetes server, its client and the namespace we are testing in
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = resourcesv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = resourcesv5.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = resourcesv2.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = resourcesv3.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	k8sClient, err := kclient.New(cfg, kclient.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	ns := createNamespaceForTest(t, k8sClient)

	t.Cleanup(func() {
		deleteNamespaceForTest(t, k8sClient, ns)
		err = testEnv.Stop()
		require.NoError(t, err)
	})

	setup := &testSetup{
		tClient:       tClient,
		k8sClient:     k8sClient,
		namespace:     ns,
		k8sRestConfig: cfg,
		operatorName:  operatorName,
	}

	// Create and start the Kubernetes operator
	setup.startKubernetesOperator(t)

	t.Cleanup(func() {
		setup.stopKubernetesOperator()
	})

	return setup
}

func teleportCreateDummyRole(ctx context.Context, roleName string, tClient *client.Client) error {
	// The role is created in Teleport
	tRole, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"a", "b"},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := tRole.GetMetadata()
	metadata.Labels = map[string]string{types.OriginLabel: types.OriginKubernetes}
	tRole.SetMetadata(metadata)

	return trace.Wrap(tClient.UpsertRole(ctx, tRole))
}

func teleportResourceToMap[T types.Resource](resource T) (map[string]interface{}, error) {
	resourceJSON, _ := json.Marshal(resource)
	resourceMap := make(map[string]interface{})
	err := json.Unmarshal(resourceJSON, &resourceMap)
	return resourceMap, trace.Wrap(err)
}
