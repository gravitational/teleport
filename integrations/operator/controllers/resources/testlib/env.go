/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package testlib

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// scheme is our own test-specific scheme to avoid using the global
// unprotected scheme.Scheme that triggers the race detector
var scheme = controllers.Scheme

func init() {
	utilruntime.Must(core.AddToScheme(scheme))
	utilruntime.Must(resourcesv1.AddToScheme(scheme))
	utilruntime.Must(resourcesv2.AddToScheme(scheme))
	utilruntime.Must(resourcesv3.AddToScheme(scheme))
	utilruntime.Must(resourcesv5.AddToScheme(scheme))
}

func createNamespaceForTest(t *testing.T, kc kclient.Client) *core.Namespace {
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ValidRandomResourceName("ns-")},
	}

	err := kc.Create(context.Background(), ns)
	require.NoError(t, err)

	return ns
}

func deleteNamespaceForTest(t *testing.T, kc kclient.Client, ns *core.Namespace) {
	err := kc.Delete(context.Background(), ns)
	require.NoError(t, err)
}

func ValidRandomResourceName(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz1234567890"
	b := make([]byte, 5)
	for i := range b {
		b[i] = letters[rand.N(len(letters))]
	}
	return prefix + string(b)
}

func defaultTeleportServiceConfig(t *testing.T) (*helpers.TeleInstance, string) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OIDC: {Enabled: true},
				entitlements.SAML: {Enabled: true},
			},
		},
	})

	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      slog.Default(),
	})

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Version = "v2"

	roleName := ValidRandomResourceName("role-")
	unrestricted := []string{"list", "create", "read", "update", "delete"}
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			// the operator has wildcard noe labs to be able to see them
			// but has no login allowed, so it cannot SSH into them
			NodeLabels: types.Labels{"*": []string{"*"}},
			Rules: []types.Rule{
				types.NewRule(types.KindRole, unrestricted),
				types.NewRule(types.KindUser, unrestricted),
				types.NewRule(types.KindAuthConnector, unrestricted),
				types.NewRule(types.KindLoginRule, unrestricted),
				types.NewRule(types.KindToken, unrestricted),
				types.NewRule(types.KindOktaImportRule, unrestricted),
				types.NewRule(types.KindAccessList, unrestricted),
				types.NewRule(types.KindNode, unrestricted),
				types.NewRule(types.KindTrustedCluster, unrestricted),
			},
		},
	})
	require.NoError(t, err)

	operatorName := ValidRandomResourceName("operator-")
	_ = teleportServer.AddUserWithRole(operatorName, role)

	err = teleportServer.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	return teleportServer, operatorName
}

func FastEventually(t *testing.T, condition func() bool) {
	require.Eventually(t, condition, time.Second, 100*time.Millisecond)
}

func FastEventuallyWithT(t *testing.T, condition func(collectT *assert.CollectT)) {
	require.EventuallyWithT(t, condition, time.Second, 100*time.Millisecond)
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

type TestSetup struct {
	TeleportClient           *client.Client
	K8sClient                kclient.Client
	K8sRestConfig            *rest.Config
	Namespace                *core.Namespace
	Operator                 manager.Manager
	OperatorCancel           context.CancelFunc
	OperatorName             string
	stepByStepReconciliation bool
	log                      *slog.Logger
}

// StartKubernetesOperator creates and start a new operator
func (s *TestSetup) StartKubernetesOperator(t *testing.T) {
	// If there was an operator running previously we make sure it is stopped
	if s.OperatorCancel != nil {
		s.StopKubernetesOperator()
	}

	k8sManager, err := ctrl.NewManager(s.K8sRestConfig, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: ctrlconfig.Controller{
			// disable name validation for tests to allow re-using the same name between tests
			SkipNameValidation: ptr.To(true),
		},
		// We enable cache to ensure the tests are close to how the manager is created when running in a real cluster
		Client: ctrlclient.Options{Cache: &ctrlclient.CacheOptions{Unstructured: true}},
	})
	require.NoError(t, err)

	slogLogger := s.log
	if slogLogger == nil {
		slogLogger = utils.NewSlogLoggerForTests()
	}

	logger := logr.FromSlogHandler(slogLogger.Handler())
	ctrl.SetLogger(logger)
	setupLog := logger.WithName("setup")

	pong, err := s.TeleportClient.Ping(context.Background())
	require.NoError(t, err)
	err = resources.SetupAllControllers(setupLog, k8sManager, s.TeleportClient, pong.ServerFeatures)
	require.NoError(t, err)

	ctx, ctxCancel := context.WithCancel(context.Background())

	s.Operator = k8sManager
	s.OperatorCancel = ctxCancel

	go func() {
		err := s.Operator.Start(ctx)
		assert.NoError(t, err)
	}()
}

func (s *TestSetup) StopKubernetesOperator() {
	s.OperatorCancel()
}

func setupTeleportClient(t *testing.T, setup *TestSetup) {
	// Override teleport client with client to locally connected teleport
	// cluster (with default tsh credentials).
	if addr := os.Getenv("OPERATOR_TEST_TELEPORT_ADDR"); addr != "" {
		creds := client.LoadProfile("", "")
		setup.TeleportClient = clientWithCreds(t, addr, creds)
		return
	}

	// A TestOption already provided a TeleportClient, return.
	if setup.TeleportClient != nil {
		return
	}

	// Start a Teleport server for the test and set up a client connected to
	// that server.
	teleportServer, operatorName := defaultTeleportServiceConfig(t)
	require.NoError(t, teleportServer.Start())
	setup.TeleportClient = clientForTeleport(t, teleportServer, operatorName)
	setup.OperatorName = operatorName
	t.Cleanup(func() {
		err := teleportServer.StopAll()
		require.NoError(t, err)
	})

	t.Cleanup(func() {
		err := setup.TeleportClient.Close()
		require.NoError(t, err)
	})
}

type TestOption func(*TestSetup)

func WithTeleportClient(clt *client.Client) TestOption {
	return func(setup *TestSetup) {
		setup.TeleportClient = clt
	}
}

func StepByStep(setup *TestSetup) {
	setup.stepByStepReconciliation = true
}

// SetupTestEnv creates a Kubernetes server, a teleport server and starts the operator
func SetupTestEnv(t *testing.T, opts ...TestOption) *TestSetup {
	// Hack to get the path of this file in order to find the crd path no matter
	// where this is called from.
	_, thisFileName, _, _ := runtime.Caller(0)
	crdPath := filepath.Join(filepath.Dir(thisFileName), "..", "..", "..", "config", "crd", "bases")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdPath},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	k8sClient, err := kclient.New(cfg, kclient.Options{Scheme: scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	ns := createNamespaceForTest(t, k8sClient)

	t.Cleanup(func() {
		deleteNamespaceForTest(t, k8sClient, ns)
		err = testEnv.Stop()
		require.NoError(t, err)
	})

	setup := &TestSetup{
		K8sClient:     k8sClient,
		Namespace:     ns,
		K8sRestConfig: cfg,
	}

	for _, opt := range opts {
		opt(setup)
	}

	setupTeleportClient(t, setup)

	// If the test wants to do step by step reconciliation, we don't start
	// an operator in the background.
	if !setup.stepByStepReconciliation {
		setup.StartKubernetesOperator(t)
		t.Cleanup(func() {
			setup.StopKubernetesOperator()
		})
	}

	return setup
}
