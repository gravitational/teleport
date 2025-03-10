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
	"cmp"
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/integration/appaccess"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/utils"
)

// testTeletermDbGatewaysCertRenewal is run from within TestALPNSNIProxyDatabaseAccess to amortize the
// cost of setting up clusters in tests.
func testTeletermDbGatewaysCertRenewal(t *testing.T, pack *dbhelpers.DatabasePack) {
	ctx := context.Background()

	t.Run("root cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		databaseURI := uri.NewClusterURI(profileName).
			AppendDB(pack.Root.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, dbGatewayCertRenewalParams{
			pack:        pack,
			databaseURI: databaseURI,
		})
	})
	t.Run("leaf cluster", func(t *testing.T) {
		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		leafClusterName := pack.Leaf.Cluster.Secrets.SiteName
		databaseURI := uri.NewClusterURI(profileName).
			AppendLeafCluster(leafClusterName).
			AppendDB(pack.Leaf.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, dbGatewayCertRenewalParams{
			pack:        pack,
			databaseURI: databaseURI,
		})
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

		testDBGatewayCertRenewal(ctx, t, dbGatewayCertRenewalParams{
			pack:        pack,
			databaseURI: databaseURI,
			albAddr:     albProxy.Addr().String(),
		})
	})
	t.Run("root cluster with per-session MFA", func(t *testing.T) {
		requireSessionMFAAuthPref(ctx, t, pack.Root.Cluster.Process.GetAuthServer(), "127.0.0.1")
		webauthnLogin := setupUserMFA(ctx, t, pack.Root.Cluster.Process.GetAuthServer(), pack.Root.User.GetName(), "127.0.0.1")

		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		databaseURI := uri.NewClusterURI(profileName).
			AppendDB(pack.Root.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, dbGatewayCertRenewalParams{
			pack:          pack,
			databaseURI:   databaseURI,
			webauthnLogin: webauthnLogin,
		})
	})
	t.Run("leaf cluster with per-session MFA", func(t *testing.T) {
		requireSessionMFAAuthPref(ctx, t, pack.Root.Cluster.Process.GetAuthServer(), "127.0.0.1")
		requireSessionMFAAuthPref(ctx, t, pack.Leaf.Cluster.Process.GetAuthServer(), "127.0.0.1")
		webauthnLogin := setupUserMFA(ctx, t, pack.Root.Cluster.Process.GetAuthServer(), pack.Root.User.GetName(), "127.0.0.1")

		profileName := mustGetProfileName(t, pack.Root.Cluster.Web)
		leafClusterName := pack.Leaf.Cluster.Secrets.SiteName
		databaseURI := uri.NewClusterURI(profileName).
			AppendLeafCluster(leafClusterName).
			AppendDB(pack.Leaf.MysqlService.Name)

		testDBGatewayCertRenewal(ctx, t, dbGatewayCertRenewalParams{
			pack:          pack,
			databaseURI:   databaseURI,
			webauthnLogin: webauthnLogin,
		})
	})
}

type dbGatewayCertRenewalParams struct {
	pack          *dbhelpers.DatabasePack
	albAddr       string
	databaseURI   uri.ResourceURI
	webauthnLogin libclient.WebauthnLoginFunc
}

func testDBGatewayCertRenewal(ctx context.Context, t *testing.T, params dbGatewayCertRenewalParams) {
	t.Helper()

	tc, err := params.pack.Root.Cluster.NewClient(helpers.ClientConfig{
		Login:   params.pack.Root.User.GetName(),
		Cluster: params.pack.Root.Cluster.Secrets.SiteName,
		ALBAddr: params.albAddr,
	})
	require.NoError(t, err)

	testGatewayCertRenewal(
		ctx,
		t,
		gatewayCertRenewalParams{
			tc:      tc,
			albAddr: params.albAddr,
			createGatewayParams: daemon.CreateGatewayParams{
				TargetURI:  params.databaseURI.String(),
				TargetUser: params.pack.Root.User.GetName(),
			},
			testGatewayConnection: mustConnectDatabaseGateway,
			webauthnLogin:         params.webauthnLogin,
			generateAndSetupUserCreds: func(t *testing.T, tc *libclient.TeleportClient, ttl time.Duration) {
				creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
					Process:  params.pack.Root.Cluster.Process,
					Username: tc.Username,
					TTL:      ttl,
				})
				require.NoError(t, err)
				err = helpers.SetupUserCreds(tc, params.pack.Root.Cluster.Process.Config.Proxy.SSHAddr.Addr, *creds)
				require.NoError(t, err)
			},
		},
	)
}

type generateAndSetupUserCredsFunc func(t *testing.T, tc *libclient.TeleportClient, ttl time.Duration)

type gatewayCertRenewalParams struct {
	tc                        *libclient.TeleportClient
	albAddr                   string
	createGatewayParams       daemon.CreateGatewayParams
	testGatewayConnection     testGatewayConnectionFunc
	webauthnLogin             libclient.WebauthnLoginFunc
	generateAndSetupUserCreds generateAndSetupUserCredsFunc
	wantPromptMFACallCount    int
	customCertsExpireFunc     func(gateway.Gateway)
	expectNoRelogin           bool
}

func testGatewayCertRenewal(ctx context.Context, t *testing.T, params gatewayCertRenewalParams) {
	t.Helper()

	// The test can potentially hang forever if something is wrong with the MFA prompt, add a timeout.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	t.Cleanup(cancel)

	tc := params.tc

	// Save the profile yaml file to disk as test helpers like helpers.NewClientWithCreds don't do
	// that by themselves.
	err := tc.SaveProfile(false /* makeCurrent */)
	require.NoError(t, err)

	tshdEventsService := newMockTSHDEventsServiceServer(t, tc, params.generateAndSetupUserCreds)

	var webauthLoginCalls atomic.Uint32
	webauthnLogin := func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		t.Helper()

		updatedWebauthnLoginCalls := webauthLoginCalls.Add(1)

		// Ensure that the mfa prompt in lib/teleterm has sent a message to the Electron app.
		// This simulates a flow where the user was notified about the need to tap the key through the
		// UI and then taps the key.
		//
		// This also makes sure that the goroutine which handles hardware key taps doesn't finish
		// before the goroutine that sends the message to the Electron app, allowing us to assert later
		// in tests that PromptMFA on tshd events service has been called.
		assert.EventuallyWithT(t, func(t *assert.CollectT) {
			// Each call to webauthnLogin should have an equivalent call to PromptMFA and there should be
			// no multiple concurrent calls.
			assert.Equal(t, updatedWebauthnLoginCalls, tshdEventsService.promptMFACallCount.Load(),
				"Expected each call to webauthnLogin to have an equivalent call to PromptMFA")
		}, 5*time.Second, 50*time.Millisecond)

		resp, credentialUser, err := params.webauthnLogin(ctx, origin, assertion, prompt, opts)
		return resp, credentialUser, err
	}

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		// Inject a fake clock into clusters.Storage so we can control when the middleware thinks the
		// db cert has expired.
		Clock:              fakeClock,
		WebauthnLogin:      webauthnLogin,
		HardwareKeyService: keys.NewHardwareKeyService(nil /*prompt*/),
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Clock:   fakeClock,
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		CreateClientCacheFunc: func(newClient clientcache.NewClientFunc) (daemon.ClientCache, error) {
			return clientcache.NewNoCache(newClient), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	// Connect the daemon to the tshd events service, like it would
	// during normal initialization of the app.
	err = daemonService.UpdateAndDialTshdEventsServerAddress(tshdEventsService.addr)
	require.NoError(t, err)

	// Here the test setup ends and actual test code starts.
	gateway, err := daemonService.CreateGateway(ctx, params.createGatewayParams)
	require.NoError(t, err, trace.DebugReport(err))

	params.testGatewayConnection(ctx, t, daemonService, gateway)

	if params.customCertsExpireFunc != nil {
		params.customCertsExpireFunc(gateway)
	} else {
		// Advance the fake clock to simulate the db cert expiry inside the middleware.
		fakeClock.Advance(time.Hour * 48)

		// Overwrite user certs with expired ones to simulate the user cert expiry.
		params.generateAndSetupUserCreds(t, tc, -time.Hour)
	}

	// Open a new connection.
	// This should trigger the relogin flow. The middleware will notice that the cert has expired
	// and then it will attempt to reissue the user cert using an expired user cert.
	// The mocked tshdEventsClient will issue a valid user cert, save it to disk, and the middleware
	// will let the connection through.
	params.testGatewayConnection(ctx, t, daemonService, gateway)

	expectedReloginCalls := uint32(1)
	if params.expectNoRelogin {
		expectedReloginCalls = uint32(0)
	}
	require.Equal(t, expectedReloginCalls, tshdEventsService.reloginCallCount.Load(),
		"Unexpected number of calls to TSHDEventsClient.Relogin")
	require.Equal(t, uint32(0), tshdEventsService.sendNotificationCallCount.Load(),
		"Unexpected number of calls to TSHDEventsClient.SendNotification")
	if params.webauthnLogin != nil {
		// By default, there are two calls, one to issue the certs when creating the gateway and then
		// another to reissue them after relogin.
		wantCallCount := cmp.Or(params.wantPromptMFACallCount, 2)
		require.Equal(t, uint32(wantCallCount), tshdEventsService.promptMFACallCount.Load(),
			"Unexpected number of calls to TSHDEventsClient.PromptMFA")
	}
}

type mockTSHDEventsService struct {
	api.UnimplementedTshdEventsServiceServer

	t                         *testing.T
	tc                        *libclient.TeleportClient
	addr                      string
	reloginCallCount          atomic.Uint32
	sendNotificationCallCount atomic.Uint32
	promptMFACallCount        atomic.Uint32
	generateAndSetupUserCreds generateAndSetupUserCredsFunc
}

func newMockTSHDEventsServiceServer(t *testing.T, tc *libclient.TeleportClient, generateAndSetupUserCreds generateAndSetupUserCredsFunc) (service *mockTSHDEventsService) {
	t.Helper()

	ls, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tshdEventsService := &mockTSHDEventsService{
		t:                         t,
		tc:                        tc,
		addr:                      ls.Addr().String(),
		generateAndSetupUserCreds: generateAndSetupUserCreds,
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
		if !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err)
		}
	})

	return tshdEventsService
}

// Relogin simulates the act of the user logging in again in the Electron app by replacing the user
// cert on disk with a valid one.
func (c *mockTSHDEventsService) Relogin(context.Context, *api.ReloginRequest) (*api.ReloginResponse, error) {
	c.reloginCallCount.Add(1)

	// Generate valid certs with the default TTL.
	c.generateAndSetupUserCreds(c.t, c.tc, 0 /* ttl */)

	return &api.ReloginResponse{}, nil
}

func (c *mockTSHDEventsService) SendNotification(context.Context, *api.SendNotificationRequest) (*api.SendNotificationResponse, error) {
	c.sendNotificationCallCount.Add(1)
	return &api.SendNotificationResponse{}, nil
}

func (c *mockTSHDEventsService) PromptMFA(context.Context, *api.PromptMFARequest) (*api.PromptMFAResponse, error) {
	c.promptMFACallCount.Add(1)

	// PromptMFAResponse returns the TOTP code, so PromptMFA itself
	// needs to be implemented only once we implement TOTP MFA.
	return nil, trace.NotImplemented("mockTSHDEventsService does not implement PromptMFA")
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

	// MFA tests.
	// They update user's authentication to Webauthn so they must run after tests which do not use MFA.
	requireSessionMFARole(ctx, t, suite.root.Process.GetAuthServer(), "localhost", kubeRole)
	requireSessionMFARole(ctx, t, suite.leaf.Process.GetAuthServer(), "localhost", kubeRole)
	webauthnLogin := setupUserMFA(ctx, t, suite.root.Process.GetAuthServer(), username, "localhost")

	t.Run("root with per-session MFA", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:         suite,
			kubeURI:       kubeURI,
			webauthnLogin: webauthnLogin,
		})
	})
	t.Run("leaf with per-session MFA", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendLeafCluster(suite.leaf.Secrets.SiteName).AppendKube(kubeClusterName)
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:         suite,
			kubeURI:       kubeURI,
			webauthnLogin: webauthnLogin,
		})
	})
	t.Run("reissue cert after clearing it for root kube", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendKube(kubeClusterName)
		// The test can potentially hang forever if something is wrong with the MFA prompt, add a timeout.
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		t.Cleanup(cancel)
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:         suite,
			kubeURI:       kubeURI,
			webauthnLogin: webauthnLogin,
			customCertsExpireFunc: func(gw gateway.Gateway) {
				kubeGw, err := gateway.AsKube(gw)
				require.NoError(t, err)
				kubeGw.ClearCerts()
			},
			expectNoRelogin: true,
		})
	})
	t.Run("reissue cert after clearing it for leaf kube", func(t *testing.T) {
		profileName := mustGetProfileName(t, suite.root.Web)
		kubeURI := uri.NewClusterURI(profileName).AppendLeafCluster(suite.leaf.Secrets.SiteName).AppendKube(kubeClusterName)
		// The test can potentially hang forever if something is wrong with the MFA prompt, add a timeout.
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		t.Cleanup(cancel)
		testKubeGatewayCertRenewal(ctx, t, kubeGatewayCertRenewalParams{
			suite:         suite,
			kubeURI:       kubeURI,
			webauthnLogin: webauthnLogin,
			customCertsExpireFunc: func(gw gateway.Gateway) {
				kubeGw, err := gateway.AsKube(gw)
				require.NoError(t, err)
				kubeGw.ClearCerts()
			},
			expectNoRelogin: true,
		})
	})
}

type kubeGatewayCertRenewalParams struct {
	suite                 *Suite
	kubeURI               uri.ResourceURI
	albAddr               string
	webauthnLogin         libclient.WebauthnLoginFunc
	customCertsExpireFunc func(gateway.Gateway)
	expectNoRelogin       bool
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

	tc, err := params.suite.root.NewClient(helpers.ClientConfig{
		Login:   params.suite.username,
		Cluster: params.suite.root.Secrets.SiteName,
		ALBAddr: params.albAddr,
	})
	require.NoError(t, err)

	testKubeConnection := func(ctx context.Context, t *testing.T, daemonService *daemon.Service, gw gateway.Gateway) {
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
			albAddr: params.albAddr,
			tc:      tc,
			createGatewayParams: daemon.CreateGatewayParams{
				TargetURI: params.kubeURI.String(),
			},
			testGatewayConnection: testKubeConnection,
			webauthnLogin:         params.webauthnLogin,
			customCertsExpireFunc: params.customCertsExpireFunc,
			expectNoRelogin:       params.expectNoRelogin,
			generateAndSetupUserCreds: func(t *testing.T, tc *libclient.TeleportClient, ttl time.Duration) {
				creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
					Process:  params.suite.root.Process,
					Username: tc.Username,
					TTL:      ttl,
				})
				require.NoError(t, err)
				err = helpers.SetupUserCreds(tc, params.suite.root.Process.Config.Proxy.SSHAddr.Addr, *creds)
				require.NoError(t, err)
			},
		},
	)
}

func checkKubeconfigPathInCommandEnv(t *testing.T, daemonService *daemon.Service, gw gateway.Gateway, wantKubeconfigPath string) {
	t.Helper()

	cmds, err := daemonService.GetGatewayCLICommand(context.Background(), gw)
	require.NoError(t, err)
	require.Equal(t, []string{"KUBECONFIG=" + wantKubeconfigPath}, cmds.Exec.Env)
	require.Equal(t, []string{"KUBECONFIG=" + wantKubeconfigPath}, cmds.Preview.Env)
}

// setupUserMFA registers a mock MFA device for the user and returns a corresponding WebauthnLoginFunc
// that can be passed to the client for MFA checks.
//
// Assumes that MFA is already enabled for the cluster. Per-session MFA should be configured separately.
//
// Based on setupUserMFA from e/tool/tsh/tsh_test.go.
func setupUserMFA(ctx context.Context, t *testing.T, authServer *auth.Server, username string, rpid string) libclient.WebauthnLoginFunc {
	t.Helper()

	// Configure user account.
	origin := "https://" + rpid
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	token, err := authServer.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: username,
	})
	require.NoError(t, err)

	tokenID := token.GetName()
	res, err := authServer.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:     tokenID,
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)
	cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

	ccr, err := device.SignCredentialCreation(origin, cc)
	require.NoError(t, err)
	_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID: tokenID,
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err)

	// webauthnLogin is not safe for concurrent use, partly due to the implementation of device, but
	// mostly because Teleport itself doesn't allow for more than one in-flight MFA challenge. This is
	// an arbitrary limitation which in theory we could change. But for now, parallel tests that use
	// webauthnLogin must use a separate user for each test and not trigger parallel MFA prompts.
	webauthnLogin := func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, "", err
		}

		carProto := wantypes.CredentialAssertionResponseToProto(car)

		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: carProto,
			},
		}, "", nil
	}

	return webauthnLogin
}

func requireSessionMFAAuthPref(ctx context.Context, t *testing.T, authServer *auth.Server, rpid string) {
	t.Helper()

	oldAuthPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		helpers.UpsertAuthPrefAndWaitForCache(t, ctx, authServer, oldAuthPref)
	})

	// Enable optional MFA with per session MFA enabled.
	helpers.UpsertAuthPrefAndWaitForCache(t, ctx, authServer, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: rpid,
			},
			RequireMFAType: types.RequireMFAType_SESSION,
		},
	})
}

func requireSessionMFARole(ctx context.Context, t *testing.T, authServer *auth.Server, rpid string, role types.Role) {
	t.Helper()

	// Enable optional MFA.
	helpers.UpsertAuthPrefAndWaitForCache(t, ctx, authServer, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: rpid,
			},
		},
	})

	// Configure role to require session MFA.
	options := role.GetOptions()
	options.RequireMFAType = types.RequireMFAType_SESSION
	role.SetOptions(options)
	_, err := authServer.UpsertRole(ctx, role)
	require.NoError(t, err)
}

type makeTCAndWebauthnLoginFunc func(t *testing.T) (*libclient.TeleportClient, mfa.WebauthnLoginFunc)

func testTeletermAppGateway(t *testing.T, pack *appaccess.Pack, makeTCAndWebauthnLogin makeTCAndWebauthnLoginFunc) {
	ctx := context.Background()

	t.Run("root cluster", func(t *testing.T) {
		t.Parallel()

		t.Run("web app", func(t *testing.T) {
			t.Parallel()

			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).AppendApp(pack.RootAppName())

			testAppGatewayCertRenewal(ctx, t, pack, makeTCAndWebauthnLogin, appURI)
		})

		t.Run("TCP app", func(t *testing.T) {
			t.Parallel()

			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).AppendApp(pack.RootTCPAppName())

			tc, webauthnLogin := makeTCAndWebauthnLogin(t)

			testGatewayCertRenewal(
				ctx,
				t,
				gatewayCertRenewalParams{
					tc:                        tc,
					createGatewayParams:       daemon.CreateGatewayParams{TargetURI: appURI.String()},
					testGatewayConnection:     makeMustConnectTCPAppGateway(pack.RootTCPMessage()),
					generateAndSetupUserCreds: pack.GenerateAndSetupUserCreds,
					webauthnLogin:             webauthnLogin,
				},
			)
		})

		t.Run("multi-port TCP app", func(t *testing.T) {
			t.Parallel()
			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).AppendApp(pack.RootTCPMultiPortAppName())

			tc, webauthnLogin := makeTCAndWebauthnLogin(t)

			testGatewayCertRenewal(
				ctx,
				t,
				gatewayCertRenewalParams{
					tc: tc,
					createGatewayParams: daemon.CreateGatewayParams{
						TargetURI:             appURI.String(),
						TargetSubresourceName: strconv.Itoa(pack.RootTCPMultiPortAppPortAlpha()),
					},
					testGatewayConnection: makeMustConnectMultiPortTCPAppGateway(
						pack.RootTCPMultiPortMessageAlpha(), pack.RootTCPMultiPortAppPortBeta(), pack.RootTCPMultiPortMessageBeta(),
					),
					generateAndSetupUserCreds: pack.GenerateAndSetupUserCreds,
					webauthnLogin:             webauthnLogin,
					// First MFA prompt is made when creating the gateway. Then makeMustConnectMultiPortTCPAppGateway
					// changes the target port twice, which means two more prompts.
					//
					// Then testGatewayCertRenewal expires the certs and calls
					// makeMustConnectMultiPortTCPAppGateway. The first connection refreshes the expired cert,
					// then the function changes the target port twice again, resulting in two more prompts.
					wantPromptMFACallCount: 3 + 3,
				},
			)
		})
	})

	t.Run("leaf cluster", func(t *testing.T) {
		t.Parallel()

		t.Run("web app", func(t *testing.T) {
			t.Parallel()

			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).
				AppendLeafCluster(pack.LeafAppClusterName()).
				AppendApp(pack.LeafAppName())

			testAppGatewayCertRenewal(ctx, t, pack, makeTCAndWebauthnLogin, appURI)
		})

		t.Run("TCP app", func(t *testing.T) {
			t.Parallel()

			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).AppendLeafCluster(pack.LeafAppClusterName()).AppendApp(pack.LeafTCPAppName())

			tc, webauthnLogin := makeTCAndWebauthnLogin(t)

			testGatewayCertRenewal(
				ctx,
				t,
				gatewayCertRenewalParams{
					tc:                        tc,
					createGatewayParams:       daemon.CreateGatewayParams{TargetURI: appURI.String()},
					testGatewayConnection:     makeMustConnectTCPAppGateway(pack.LeafTCPMessage()),
					generateAndSetupUserCreds: pack.GenerateAndSetupUserCreds,
					webauthnLogin:             webauthnLogin,
				},
			)
		})

		t.Run("multi-port TCP app", func(t *testing.T) {
			t.Parallel()

			profileName := mustGetProfileName(t, pack.RootWebAddr())
			appURI := uri.NewClusterURI(profileName).AppendLeafCluster(pack.LeafAppClusterName()).AppendApp(pack.LeafTCPMultiPortAppName())

			tc, webauthnLogin := makeTCAndWebauthnLogin(t)

			testGatewayCertRenewal(
				ctx,
				t,
				gatewayCertRenewalParams{
					tc: tc,
					createGatewayParams: daemon.CreateGatewayParams{
						TargetURI:             appURI.String(),
						TargetSubresourceName: strconv.Itoa(pack.LeafTCPMultiPortAppPortAlpha()),
					},
					testGatewayConnection: makeMustConnectMultiPortTCPAppGateway(
						pack.LeafTCPMultiPortMessageAlpha(), pack.LeafTCPMultiPortAppPortBeta(), pack.LeafTCPMultiPortMessageBeta(),
					),
					generateAndSetupUserCreds: pack.GenerateAndSetupUserCreds,
					webauthnLogin:             webauthnLogin,
					// First MFA prompt is made when creating the gateway. Then makeMustConnectMultiPortTCPAppGateway
					// changes the target port twice, which means two more prompts.
					//
					// Then testGatewayCertRenewal expires the certs and calls
					// makeMustConnectMultiPortTCPAppGateway. The first connection refreshes the expired cert,
					// then the function changes the target port twice again, resulting in two more prompts.
					wantPromptMFACallCount: 3 + 3,
				},
			)
		})
	})
}

func testTeletermAppGatewayTargetPortValidation(t *testing.T, pack *appaccess.Pack, makeTCAndWebauthnLogin makeTCAndWebauthnLoginFunc) {
	t.Run("target port validation", func(t *testing.T) {
		t.Parallel()

		tc, _ := makeTCAndWebauthnLogin(t)
		err := tc.SaveProfile(false /* makeCurrent */)
		require.NoError(t, err)

		storage, err := clusters.NewStorage(clusters.Config{
			Dir:                tc.KeysDir,
			InsecureSkipVerify: tc.InsecureSkipVerify,
			HardwareKeyService: keys.NewHardwareKeyService(nil /*prompt*/),
		})
		require.NoError(t, err)
		daemonService, err := daemon.New(daemon.Config{
			Storage: storage,
			CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
				return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
			},
			CreateClientCacheFunc: func(newClient clientcache.NewClientFunc) (daemon.ClientCache, error) {
				return clientcache.NewNoCache(newClient), nil
			},
			KubeconfigsDir: t.TempDir(),
			AgentsDir:      t.TempDir(),
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			daemonService.Stop()
		})

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		t.Cleanup(cancel)

		// Here the test setup ends and actual test code starts.
		profileName := mustGetProfileName(t, pack.RootWebAddr())
		appURI := uri.NewClusterURI(profileName).AppendApp(pack.RootTCPMultiPortAppName())

		_, err = daemonService.CreateGateway(ctx, daemon.CreateGatewayParams{
			TargetURI: appURI.String(),
			// 42 shouldn't be handed out to a non-root user when creating a listener on port 0, so it's
			// unlikely that 42 is going to end up in the app spec.
			TargetSubresourceName: "42",
		})
		require.True(t, trace.IsBadParameter(err), "Expected BadParameter, got %v", err)
		require.ErrorContains(t, err, "not included in target ports")

		gateway, err := daemonService.CreateGateway(ctx, daemon.CreateGatewayParams{
			TargetURI:             appURI.String(),
			TargetSubresourceName: strconv.Itoa(pack.RootTCPMultiPortAppPortAlpha()),
		})
		require.NoError(t, err)

		_, err = daemonService.SetGatewayTargetSubresourceName(ctx, gateway.URI().String(), "42")
		require.True(t, trace.IsBadParameter(err), "Expected BadParameter, got %v", err)
		require.ErrorContains(t, err, "not included in target ports")
	})
}

func testAppGatewayCertRenewal(ctx context.Context, t *testing.T, pack *appaccess.Pack, makeTCAndWebauthnLogin makeTCAndWebauthnLoginFunc, appURI uri.ResourceURI) {
	t.Helper()
	tc, webauthnLogin := makeTCAndWebauthnLogin(t)

	testGatewayCertRenewal(
		ctx,
		t,
		gatewayCertRenewalParams{
			tc: tc,
			createGatewayParams: daemon.CreateGatewayParams{
				TargetURI: appURI.String(),
			},
			testGatewayConnection:     mustConnectWebAppGateway,
			generateAndSetupUserCreds: pack.GenerateAndSetupUserCreds,
			webauthnLogin:             webauthnLogin,
		},
	)
}
