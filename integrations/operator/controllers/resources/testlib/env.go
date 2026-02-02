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
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	apiresources "github.com/gravitational/teleport/integrations/operator/apis/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// scheme is our own test-specific scheme to avoid using the global
// unprotected scheme.Scheme that triggers the race detector
var scheme = controllers.Scheme

func createNamespaceForTest(t *testing.T, kc kclient.Client) *core.Namespace {
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ValidRandomResourceName("ns-")},
	}

	err := kc.Create(context.Background(), ns)
	require.NoError(t, err)

	return ns
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
	modulestest.SetTestModules(t, modulestest.Modules{
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
			NodeLabels:     types.Labels{"*": []string{"*"}},
			AppLabels:      types.Labels{"*": []string{"*"}},
			DatabaseLabels: types.Labels{"*": []string{"*"}},
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
				types.NewRule(types.KindBot, unrestricted),
				types.NewRule(types.KindWorkloadIdentity, unrestricted),
				types.NewRule(types.KindAutoUpdateConfig, unrestricted),
				types.NewRule(types.KindAutoUpdateVersion, unrestricted),
				types.NewRule(types.KindApp, unrestricted),
				types.NewRule(types.KindDatabase, unrestricted),
				types.NewRule(types.KindInferenceModel, unrestricted),
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
	TeleportServer           *helpers.TeleInstance
	ResourceName             string
	Context                  context.Context
}

// StartKubernetesOperator creates and start a new operator
func (s *TestSetup) StartKubernetesOperator(t *testing.T) {
	// If there was an operator running previously we make sure it is stopped
	if s.OperatorCancel != nil {
		s.StopKubernetesOperator()
	}

	if s.K8sRestConfig == nil {
		require.FailNow(t, "K8sRestConfig is required to start the operator, you cannot run a full test against a fake cluster.")
	}
	k8sManager, err := ctrl.NewManager(s.K8sRestConfig, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: ctrlconfig.Controller{
			// disable name validation for tests to allow re-using the same name between tests
			SkipNameValidation: ptr.To(true),
		},
		// We enable cache to ensure the tests are close to how the manager is created when running in a real cluster
		Client: kclient.Options{Cache: &kclient.CacheOptions{Unstructured: true}},
	})
	require.NoError(t, err)

	slogLogger := s.log
	if slogLogger == nil {
		slogLogger = logtest.NewLogger()
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
	setup.TeleportServer = teleportServer
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

// WithResourceName makes the test resource name static instead of letting the test generate a random one.
// This is used if the resource name much match a specific pattern, or a fixture that was created beforehand
// (e.g. trusted cluster).
func WithResourceName(resourceName string) TestOption {
	return func(setup *TestSetup) {
		setup.ResourceName = resourceName
	}
}

// SetupFakeKubeTestEnv is like SetupTestEnv but creates a fake Kubernetes
// cluster by using controller-runtime's fake package.
// This is way faster than using testEnv to spin up a full kube test cluster
// on every test.
// This does not support tests starting a full controller manager and can only be
// used with Synchronous tests.
func SetupFakeKubeTestEnv(t *testing.T, opts ...TestOption) *TestSetup {
	builder := fake.NewClientBuilder()
	builder.WithScheme(scheme)
	// Every CR kind must be registered as "WithStatusSubresource" so he fake client implements
	// the status updates properly.
	customScheme, err := apiresources.NewScheme()
	require.NoError(t, err)
	knownTypes := customScheme.AllKnownTypes()
	for _, reflectType := range knownTypes {
		reflectValue := reflect.New(reflectType).Interface()
		obj, ok := reflectValue.(kclient.Object)
		if !ok {
			continue
		}
		builder.WithStatusSubresource(obj)
	}
	k8sClient := builder.Build()
	ns := createNamespaceForTest(t, k8sClient)

	setup := &TestSetup{
		Context:      t.Context(),
		K8sClient:    k8sClient,
		Namespace:    ns,
		ResourceName: ValidRandomResourceName("resource-"),
	}

	for _, opt := range opts {
		opt(setup)
	}

	setupTeleportClient(t, setup)

	return setup
}
