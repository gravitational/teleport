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
	ctx := context.Background()

	t.Run("root cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		databaseURI := uri.NewClusterURI(profileName).
			AppendDB(pack.Root.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, pack, "", databaseURI)
	})
	t.Run("leaf cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		leafClusterName := pack.Leaf.Cluster.Secrets.SiteName
		databaseURI := uri.NewClusterURI(profileName).
			AppendLeafCluster(leafClusterName).
			AppendDB(pack.Leaf.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, pack, "", databaseURI)
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

		testDBGatewayCertRenewal(ctx, t, pack, albProxy.Addr().String(), databaseURI)
	})
}

func testDBGatewayCertRenewal(ctx context.Context, t *testing.T, pack *dbhelpers.DatabasePack, albAddr string, databaseURI uri.ResourceURI) {
	t.Helper()

	testGatewayCertRenewal(
		ctx,
		t,
		gatewayCertRenewalParams{
			inst:     pack.Root.Cluster,
			username: pack.Root.User.GetName(),
			albAddr:  albAddr,
			createGatewayParams: daemon.CreateGatewayParams{
			TargetURI:  databaseURI.String(),
			TargetUser: pack.Root.User.GetName(),
		},
			testGatewayConnectionFunc: mustConnectDatabaseGateway,
		},
	)
}

type testGatewayConnectionFunc func(*testing.T, *daemon.Service, gateway.Gateway)

type gatewayCertRenewalParams struct {
	inst                      *helpers.TeleInstance
	username                  string
	albAddr                   string
	createGatewayParams       daemon.CreateGatewayParams
	testGatewayConnectionFunc testGatewayConnectionFunc
}

func testGatewayCertRenewal(ctx context.Context, t *testing.T, params gatewayCertRenewalParams) {
	t.Helper()

	tc, err := params.inst.NewClient(helpers.ClientConfig{
		Login:   params.username,
		Cluster: params.inst.Secrets.SiteName,
		ALBAddr: params.albAddr,
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
		Clock:   fakeClock,
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
	tshdEventsService := newMockTSHDEventsServiceServer(t, tc, params.inst, params.username)
	err = daemonService.UpdateAndDialTshdEventsServerAddress(tshdEventsService.addr)
	require.NoError(t, err)

	// Here the test setup ends and actual test code starts.
	gateway, err := daemonService.CreateGateway(ctx, params.createGatewayParams)
	require.NoError(t, err, trace.DebugReport(err))

	params.testGatewayConnectionFunc(t, daemonService, gateway)

	// Advance the fake clock to simulate the db cert expiry inside the middleware.
	fakeClock.Advance(time.Hour * 48)

	// Overwrite user certs with expired ones to simulate the user cert expiry.
	expiredCreds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  params.inst.Process,
		Username: params.username,
		TTL:      -time.Hour,
	})
	require.NoError(t, err)
	err = helpers.SetupUserCreds(tc, params.inst.Config.Proxy.SSHAddr.Addr, *expiredCreds)
	require.NoError(t, err)

	// Open a new connection.
	// This should trigger the relogin flow. The middleware will notice that the cert has expired
	// and then it will attempt to reissue the user cert using an expired user cert.
	// The mocked tshdEventsClient will issue a valid user cert, save it to disk, and the middleware
	// will let the connection through.
	params.testGatewayConnectionFunc(t, daemonService, gateway)

	require.Equal(t, 1, tshdEventsService.callCounts["Relogin"],
		"Unexpected number of calls to TSHDEventsClient.Relogin")
	require.Equal(t, 0, tshdEventsService.callCounts["SendNotification"],
		"Unexpected number of calls to TSHDEventsClient.SendNotification")
}

type mockTSHDEventsService struct {
	*api.UnimplementedTshdEventsServiceServer

	tc         *libclient.TeleportClient
	inst       *helpers.TeleInstance
	username   string
	addr       string
	callCounts map[string]int
}

func newMockTSHDEventsServiceServer(t *testing.T, tc *libclient.TeleportClient, inst *helpers.TeleInstance, username string) (service *mockTSHDEventsService) {
	t.Helper()

	ls, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tshdEventsService := &mockTSHDEventsService{
		tc:         tc,
		inst:       inst,
		username:   username,
		addr:       ls.Addr().String(),
		callCounts: make(map[string]int),
	}

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

	return tshdEventsService
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
	ctx := context.Background()

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
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:   suite,
			kubeURI: kubeURI,
		})
	})
	t.Run("leaf", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendLeafCluster(suite.leaf.Secrets.SiteName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:   suite,
			kubeURI: kubeURI,
		})
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
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:   suite,
			kubeURI: kubeURI,
			albAddr: albProxy.Addr().String(),
		})
	})
}

type kubeGatewayCertRenewalParams struct {
	suite         *Suite
	kubeURI       uri.ResourceURI
	albAddr       string
}

func testKubeGatewayCertRenewal(ctx context.Context, t *testing.T, params kubeGatewayCertRenewalParams) {
	t.Helper()

	var client *kubernetes.Clientset
	var clientOnce sync.Once

	kubeCluster := params.kubeURI.GetKubeName()
	teleportCluster := params.suite.root.Secrets.SiteName
	if params.kubeURI.GetLeafClusterName() != "" {
		teleportCluster = params.kubeURI.GetLeafClusterName()
	}

	testKubeConnection := func(t *testing.T, daemonService *daemon.Service, gw gateway.Gateway) {
		t.Helper()

		clientOnce.Do(func() {
			kubeGateway, err := gateway.AsKube(gw)
			require.NoError(t, err)

			kubeconfigPath := kubeGateway.KubeconfigPath()
			checkKubeconfigPathInCommandEnv(t, daemonService, gw, kubeconfigPath)

			client = kubeClientForLocalProxy(t, kubeconfigPath, teleportCluster, kubeCluster)
		})

		mustGetKubePod(t, client)
	}

	testGatewayCertRenewal(
		ctx,
		t,
		gatewayCertRenewalParams{
			inst:     params.suite.root,
			username: params.suite.username,
			albAddr:  params.albAddr,
			createGatewayParams: daemon.CreateGatewayParams{
				TargetURI: params.kubeURI.String(),
			},
			testGatewayConnectionFunc: testKubeConnection,
		},
	)
}

func checkKubeconfigPathInCommandEnv(t *testing.T, daemonService *daemon.Service, gw gateway.Gateway, wantKubeconfigPath string) {
	t.Helper()

	cmd, err := daemonService.GetGatewayCLICommand(gw)
	require.NoError(t, err)
	require.Equal(t, []string{"KUBECONFIG=" + wantKubeconfigPath}, cmd.Env)
}
