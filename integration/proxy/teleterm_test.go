// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/utils"
)

// testTeletermGatewaysCertRenewal is run from within TestALPNSNIProxyDatabaseAccess to amortize the
// cost of setting up clusters in tests.
func testTeletermGatewaysCertRenewal(t *testing.T, pack *dbhelpers.DatabasePack) {
	t.Run("root cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		databaseURI := uri.NewClusterURI(profileName).
			AppendDB(pack.Root.MysqlService.Name)

		testDBGatewayCertRenewal(t, pack, "", databaseURI)
	})
	t.Run("leaf cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		leafClusterName := pack.Leaf.Cluster.Secrets.SiteName
		databaseURI := uri.NewClusterURI(profileName).
			AppendLeafCluster(leafClusterName).
			AppendDB(pack.Leaf.MysqlService.Name)

		testDBGatewayCertRenewal(t, pack, "", databaseURI)
	})
	t.Run("ALPN connection upgrade", func(t *testing.T) {
		// Make a mock ALB which points to the Teleport Proxy Service. Then
		// ALPN local proxies will point to this ALB instead.
		albProxy := helpers.MustStartMockALBProxy(t, pack.Root.Cluster.Web)

		// Note that profile name is taken from tc.WebProxyAddr. Use
		// albProxy.Addr() as profile name in case it's different from
		// pack.Root.Cluster.Web (e.g. 127.0.0.1 vs localhost).
		profileName := mustGetProfileName(t, albProxy.Addr().String())
		databaseURI := uri.NewClusterURI(profileName).
			AppendDB(pack.Root.MysqlService.Name)

		testDBGatewayCertRenewal(t, pack, albProxy.Addr().String(), databaseURI)
	})
}

func testDBGatewayCertRenewal(t *testing.T, pack *dbhelpers.DatabasePack, albAddr string, databaseURI uri.ResourceURI) {
	t.Helper()

	testGatewayCertRenewal(
		t,
		pack.Root.Cluster,
		pack.Root.User.GetName(),
		albAddr,
		daemon.CreateGatewayParams{
			TargetURI:  databaseURI.String(),
			TargetUser: pack.Root.User.GetName(),
		},
		mustConnectDatabaseGateway,
	)
}

type testGatewayConnectionFunc func(*testing.T, gateway.Gateway)

func testGatewayCertRenewal(t *testing.T, inst *helpers.TeleInstance, username, albAddr string, params daemon.CreateGatewayParams, testConnection testGatewayConnectionFunc) {
	t.Helper()

	tc, err := inst.NewClient(helpers.ClientConfig{
		Login:   username,
		Cluster: inst.Secrets.SiteName,
		ALBAddr: albAddr,
	})
	require.NoError(t, err)

	// Save the profile yaml file to disk as NewClientWithCreds doesn't do that by itself.
	err = tc.SaveProfile(false /* makeCurrent */)
	require.NoError(t, err)

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		// Inject a fake clock into clusters.Storage so we can control when the middleware thinks the
		// db cert has expired.
		Clock: fakeClock,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	// Create a mock tshd events service server and have the daemon connect to it,
	// like it would during normal initialization of the app.
	tshdEventsService, tshEventsServerAddr := newMockTSHDEventsServiceServer(t, tc, inst, username)
	err = daemonService.UpdateAndDialTshdEventsServerAddress(tshEventsServerAddr)
	require.NoError(t, err)

	// Here the test setup ends and actual test code starts.
	gateway, err := daemonService.CreateGateway(context.Background(), params)
	require.NoError(t, err, trace.DebugReport(err))

	testConnection(t, gateway)

	// Advance the fake clock to simulate the db cert expiry inside the middleware.
	fakeClock.Advance(time.Hour * 48)

	// Overwrite user certs with expired ones to simulate the user cert expiry.
	expiredCreds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  inst.Process,
		Username: username,
		TTL:      -time.Hour,
	})
	require.NoError(t, err)
	err = helpers.SetupUserCreds(tc, inst.Config.Proxy.SSHAddr.Addr, *expiredCreds)
	require.NoError(t, err)

	// Open a new connection.
	// This should trigger the relogin flow. The middleware will notice that the cert has expired
	// and then it will attempt to reissue the user cert using an expired user cert.
	// The mocked tshdEventsClient will issue a valid user cert, save it to disk, and the middleware
	// will let the connection through.
	testConnection(t, gateway)

	require.Equal(t, 1, tshdEventsService.callCounts["Relogin"],
		"Unexpected number of calls to TSHDEventsClient.Relogin")
	require.Equal(t, 0, tshdEventsService.callCounts["SendNotification"],
		"Unexpected number of calls to TSHDEventsClient.SendNotification")
}

type mockTSHDEventsService struct {
	api.UnimplementedTshdEventsServiceServer

	tc         *libclient.TeleportClient
	inst       *helpers.TeleInstance
	username   string
	callCounts map[string]int
}

func newMockTSHDEventsServiceServer(t *testing.T, tc *libclient.TeleportClient, inst *helpers.TeleInstance, username string) (service *mockTSHDEventsService, addr string) {
	t.Helper()

	tshdEventsService := &mockTSHDEventsService{
		tc:         tc,
		inst:       inst,
		username:   username,
		callCounts: make(map[string]int),
	}

	ls, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	api.RegisterTshdEventsServiceServer(grpcServer, tshdEventsService)

	serveErr := make(chan error)
	go func() {
		serveErr <- grpcServer.Serve(ls)
	}()

	t.Cleanup(func() {
		grpcServer.GracefulStop()

		// For test cases that did not send any grpc calls, test may finish
		// before grpcServer.Serve is called and grpcServer.Serve will return
		// grpc.ErrServerStopped.
		err := <-serveErr
		if err != grpc.ErrServerStopped {
			assert.NoError(t, err)
		}
	})

	return tshdEventsService, ls.Addr().String()
}

// Relogin simulates the act of the user logging in again in the Electron app by replacing the user
// cert on disk with a valid one.
func (c *mockTSHDEventsService) Relogin(context.Context, *api.ReloginRequest) (*api.ReloginResponse, error) {
	c.callCounts["Relogin"]++
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  c.inst.Process,
		Username: c.username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = helpers.SetupUserCreds(c.tc, c.inst.Config.Proxy.SSHAddr.Addr, *creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.ReloginResponse{}, nil
}

func (c *mockTSHDEventsService) SendNotification(context.Context, *api.SendNotificationRequest) (*api.SendNotificationResponse, error) {
	c.callCounts["SendNotification"]++
	return &api.SendNotificationResponse{}, nil
}

// TestTeletermKubeGateway tests making kube API calls against Teleterm kube
// gateway and reissuing certs.
//
// Note that this test does NOT reuse existing kube test setups as IP Pinning
// is enabled in those tests. User certs with pinned IPs are injected during
// those tests, which is not feasible for Teleterm daemon flow.
func TestTeletermKubeGateway(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	const (
		localK8SNI = constants.KubeTeleportProxyALPNPrefix + "teleport.cluster.local"
		k8User     = "alice@example.com"
		k8RoleName = "kubemaster"
	)

	kubeAPIMockSvr := startKubeAPIMock(t)
	kubeConfigPath := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvr.URL, localK8SNI))

	username := helpers.MustGetCurrentUser(t).Username
	kubeRoleSpec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:           []string{username},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubeGroups:       []string{kube.TestImpersonationGroup},
			KubeUsers:        []string{k8User},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)
	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Version = defaults.TeleportConfigVersionV2
			config.Proxy.Kube.Enabled = true
			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPath
			config.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &config.FileDescriptors))
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Version = defaults.TeleportConfigVersionV2
			config.Proxy.Kube.Enabled = true
			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPath
			config.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &config.FileDescriptors))
		}),
		withRootClusterRoles(kubeRole),
		withLeafClusterRoles(kubeRole),
		withRootAndLeafTrustedClusterReset(),
		withTrustedCluster(),
	)

	t.Run("root", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(t, suite, "", kubeURI)
	})
	t.Run("leaf", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendLeafCluster(suite.leaf.Secrets.SiteName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(t, suite, "", kubeURI)
	})
	t.Run("ALPN connection upgrade", func(t *testing.T) {
		// Make a mock ALB which points to the Teleport Proxy Service. Then
		// ALPN local proxies will point to this ALB instead.
		albProxy := helpers.MustStartMockALBProxy(t, suite.root.Web)

		// Note that profile name is taken from tc.WebProxyAddr. Use
		// albProxy.Addr() as profile name in case it's different from
		// suite.root.Web (e.g. 127.0.0.1 vs localhost).
		profileName := mustGetProfileName(t, albProxy.Addr().String())

		kubeURI := uri.NewClusterURI(profileName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(t, suite, albProxy.Addr().String(), kubeURI)
	})
}

func testKubeGatewayCertRenewal(t *testing.T, suite *Suite, albAddr string, kubeURI uri.ResourceURI) {
	t.Helper()

	var client *kubernetes.Clientset
	var clientOnce sync.Once

	kubeCluster := kubeURI.GetKubeName()
	teleportCluster := suite.root.Secrets.SiteName
	if kubeURI.GetLeafClusterName() != "" {
		teleportCluster = kubeURI.GetLeafClusterName()
	}

	testKubeConnection := func(t *testing.T, gw gateway.Gateway) {
		t.Helper()

		clientOnce.Do(func() {
			kubeGateway, err := gateway.AsKube(gw)
			require.NoError(t, err)

			kubeconfigPath := kubeGateway.KubeconfigPath()
			checkKubeconfigPathInCommandEnv(t, gw, kubeconfigPath)

			client = kubeClientForLocalProxy(t, kubeconfigPath, teleportCluster, kubeCluster)
		})

		mustGetKubePod(t, client)
	}

	testGatewayCertRenewal(
		t,
		suite.root,
		suite.username,
		albAddr,
		daemon.CreateGatewayParams{
			TargetURI: kubeURI.String(),
		},
		testKubeConnection,
	)
}

func checkKubeconfigPathInCommandEnv(t *testing.T, gw gateway.Gateway, wantKubeconfigPath string) {
	t.Helper()

	cmd, err := gw.CLICommand()
	require.NoError(t, err)
	require.Equal(t, []string{"KUBECONFIG=" + wantKubeconfigPath}, cmd.Env)
}
