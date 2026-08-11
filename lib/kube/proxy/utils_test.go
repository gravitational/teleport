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
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	kubewatcher "github.com/gravitational/teleport/lib/kube/proxy/watcher"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	sessPkg "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type TestContext struct {
	HostID             string
	ClusterName        string
	Scope              string
	TLSServer          *authtest.TLSServer
	AuthServer         *auth.Server
	AuthClient         *authclient.Client
	ScopedAuthz        authz.ScopedAuthorizer
	KubeServer         *TLSServer
	KubeProxy          *TLSServer
	Emitter            *eventstest.ChannelEmitter
	Context            context.Context
	UploadHandler      *eventstest.MemoryUploader
	kubeServerListener net.Listener
	kubeProxyListener  net.Listener
	cancel             context.CancelFunc
	heartbeatCtx       context.Context
	heartbeatCancel    context.CancelFunc
	lockWatcher        *services.LockWatcher
	// The following fields are owned by the original context and are reused by
	// scoped clones. They are intentionally private: callers should use the
	// clone helper rather than sharing individual test components.
	proxyAuthClient    *authclient.Client
	kubeconfigPath     string
	keyGen             *keygen.Keygen
	clusterFeatures    func() proto.Features
	healthCheckManager healthcheck.Manager
	config             TestConfig
	isClone            bool
	closeOnce          sync.Once
	closeErr           error
}

// KubeClusterConfig defines the cluster to be created
type KubeClusterConfig struct {
	// Name is the cluster name.
	Name string
	// APIEndpoint is the cluster API endpoint.
	APIEndpoint string
}

// TestConfig defines the suite options.
type TestConfig struct {
	Modules              *modulestest.Modules
	Clusters             []KubeClusterConfig
	ResourceMatchers     []services.ResourceMatcher
	OnReconcile          func(types.KubeClusters)
	OnEvent              func(apievents.AuditEvent)
	ClusterFeatures      func() proto.Features
	CreateAuditStreamErr error
	WrapAuthClient       func(authclient.ClientI) authclient.ClientI
	WrapProxyAccessPoint func(authclient.ClientI) authclient.ClientI
	ScopesFeatures       scopes.Features
	Scope                string
}

// SetupTestContext creates a kube service with clusters configured.
func SetupTestContext(ctx context.Context, t *testing.T, cfg TestConfig) *TestContext {
	ctx, cancel := context.WithCancel(ctx)
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	testCtx := &TestContext{
		ClusterName:     "root.example.com",
		HostID:          uuid.New().String(),
		Scope:           cfg.Scope,
		Context:         ctx,
		cancel:          cancel,
		heartbeatCtx:    heartbeatCtx,
		heartbeatCancel: heartbeatCancel,
		UploadHandler:   eventstest.NewMemoryUploader(),
		config:          cfg,
	}
	t.Cleanup(func() { testCtx.Close() })

	kubeConfigLocation := newKubeConfigFile(t, cfg.Clusters...)
	testCtx.kubeconfigPath = kubeConfigLocation

	streamer, err := events.NewProtoStreamer(
		events.ProtoStreamerConfig{
			Uploader: testCtx.UploadHandler,
		},
	)
	require.NoError(t, err)

	// Create and start test auth server.
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Clock:          clockwork.NewFakeClockAt(time.Now()),
		ClusterName:    testCtx.ClusterName,
		Streamer:       streamer,
		UploadHandler:  testCtx.UploadHandler,
		Dir:            t.TempDir(),
		Modules:        cmp.Or(cfg.Modules, modulestest.OSSModules()),
		ScopesFeatures: cfg.ScopesFeatures,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	testCtx.TLSServer, err = authServer.NewTestTLSServer(
		// This test context is used by a test that stalls the LockWatcher to
		// simulate the enforcement of the strict lock mode. When the test fakes
		// the stall, the LockWatcher will enter a loop that constantly tries to
		// pull locks from the backend to recover from the stall. This context causes
		// the LockWatcher to hit the connection rate limit and fail with an error
		// different from the expected one. We setup a custom rate limiter to avoid
		// this issue.
		authtest.WithLimiterConfig(
			&limiter.Config{
				MaxConnections: 100000,
			},
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.TLSServer.Close()) })

	testCtx.AuthServer = testCtx.TLSServer.Auth()

	// Use sync recording to not involve the uploader.
	recConfig, err := authServer.AuthServer.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)
	// Always use *-sync to prevent fileStreamer from running against os.RemoveAll
	// once the test ends.
	recConfig.SetMode(types.RecordAtNodeSync)
	_, err = authServer.AuthServer.UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Auth client for Kube service.
	testCtx.AuthClient, err = testCtx.TLSServer.NewClient(authtest.TestScopedServerID(types.RoleKube, testCtx.HostID, cfg.Scope))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.AuthClient.Close()) })

	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := testCtx.TLSServer.NewClient(authtest.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	testCtx.lockWatcher, err = services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    proxyAuthClient,
		},
	})
	testCtx.proxyAuthClient = proxyAuthClient
	require.NoError(t, err)
	t.Cleanup(func() {
		testCtx.lockWatcher.Close()
	})
	testCtx.ScopedAuthz, err = authz.NewScopedAuthorizer(authz.AuthorizerOpts{
		ClusterName:      testCtx.ClusterName,
		AccessPoint:      proxyAuthClient,
		ScopedRoleReader: proxyAuthClient.ScopedRoleReader(),
		LockWatcher:      testCtx.lockWatcher,
		ScopesFeatures:   cfg.ScopesFeatures,
	})
	require.NoError(t, err)

	// TLS config for kube proxy and Kube service.
	serverIdentity, err := authtest.NewScopedServerIdentity(authServer.AuthServer, testCtx.HostID, cfg.Scope, types.RoleKube)
	require.NoError(t, err)
	kubeServiceTLSConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Create test audit events emitter.
	testCtx.Emitter = eventstest.NewChannelEmitter(100)
	go func() {
		for {
			select {
			case evt := <-testCtx.Emitter.C():
				if cfg.OnEvent != nil {
					cfg.OnEvent(evt)
				}
			case <-testCtx.Context.Done():
				return
			}
		}
	}()
	keyGen, err := keygen.New(keygen.Config{BuildType: modules.BuildOSS})
	require.NoError(t, err)
	testCtx.keyGen = keyGen

	client := newAuthClientWithStreamer(testCtx, cfg.CreateAuditStreamErr)

	features := func() proto.Features {
		return proto.Features{
			Entitlements: map[string]*proto.EntitlementInfo{
				string(entitlements.K8s): {Enabled: true},
			},
		}
	}
	if cfg.ClusterFeatures != nil {
		features = cfg.ClusterFeatures
	}
	testCtx.clusterFeatures = features

	testCtx.kubeServerListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	testCtx.kubeProxyListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	inventoryHandle, err := inventory.NewDownstreamHandle(client.InventoryControlStream,
		func(_ context.Context) (*proto.UpstreamInventoryHello, error) {
			return proto.UpstreamInventoryHello_builder{
				ServerID: testCtx.HostID,
				Version:  teleport.Version,
				Services: types.SystemRoles{types.RoleKube}.StringSlice(),
				Hostname: "test",
				Scope:    cfg.Scope,
			}.Build(), nil
		})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inventoryHandle.Close()) })

	healthCheckManager, err := healthcheck.NewManager(
		testCtx.Context,
		healthcheck.ManagerConfig{
			Component:               teleport.ComponentKube,
			Events:                  client,
			HealthCheckConfigReader: client,
		},
	)
	require.NoError(t, err)
	if cfg.Scope == "" {
		err = healthCheckManager.Start(testCtx.Context)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, healthCheckManager.Close()) })
	}
	testCtx.healthCheckManager = healthCheckManager

	var authClient authclient.ClientI = client
	if cfg.WrapAuthClient != nil {
		authClient = cfg.WrapAuthClient(client)
	}

	var accessPoint authclient.ClientI = client
	if cfg.WrapAuthClient != nil {
		accessPoint = cfg.WrapAuthClient(client)
	}
	// The kube service must use its scoped identity to watch kube_cluster
	// resources. The proxy, however, watches kube_servers in every scope.
	proxyAccessPoint := accessPoint
	if cfg.Scope != "" {
		proxyAccessPoint = proxyAuthClient
	}
	if cfg.WrapProxyAccessPoint != nil {
		proxyAccessPoint = cfg.WrapProxyAccessPoint(proxyAccessPoint)
	}

	// Create kubernetes service server.
	testCtx.KubeServer, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			Namespace:   apidefaults.Namespace,
			Keygen:      keyGen,
			ClusterName: testCtx.ClusterName,
			ScopedAuthz: testCtx.ScopedAuthz,
			// fileStreamer continues to write events after the server is shutdown and
			// races against os.RemoveAll leading the test to fail.
			// Using "node-sync" mode to write the events and session recordings
			// directly to AuthClient solves the issue.
			// We wrap the AuthClient with an events.TeeStreamer to send non-disk
			// events like session.end to testCtx.emitter as well.
			AuthClient: authClient,
			// StreamEmitter is required although not used because we are using
			// "node-sync" as session recording mode.
			Emitter:           testCtx.Emitter,
			DataDir:           t.TempDir(),
			CachingAuthClient: accessPoint,
			HostID:            testCtx.HostID,
			Context:           testCtx.Context,
			KubeconfigPath:    kubeConfigLocation,
			KubeServiceType:   KubeService,
			Component:         teleport.ComponentKube,
			LockWatcher:       testCtx.lockWatcher,
			// skip Impersonation validation
			CheckImpersonationPermissions: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
				return nil
			},
			Clock:           clockwork.NewRealClock(),
			ClusterFeatures: features,
			Scope:           cfg.Scope,
		},
		DynamicLabels: nil,
		TLS:           kubeServiceTLSConfig.Clone(),
		AccessPoint:   accessPoint,
		LimiterConfig: limiter.Config{
			MaxConnections: 1000,
		},
		// each time heartbeat is called we insert data into the channel.
		// this is used to make sure that heartbeat started and the clusters
		// are registered in the auth server
		OnHeartbeat:          func(err error) {},
		GetRotation:          func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ResourceMatchers:     cfg.ResourceMatchers,
		OnReconcile:          cfg.OnReconcile,
		Log:                  logtest.NewLogger(),
		InventoryHandle:      inventoryHandle,
		ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
		HealthCheckManager:   healthCheckManager,
	})
	require.NoError(t, err)

	// Create kubernetes proxy server.
	kubeServersWatcher, err := kubewatcher.NewProxyKubeServerWatcher(
		testCtx.Context,
		kubewatcher.ProxyKubeServerWatcherConfig{
			Logger:           logtest.NewLogger(),
			AccessPoint:      proxyAccessPoint,
			FallbackGetter:   proxyAuthClient,
			PrimaryTimeout:   time.Second,
			FallbackInterval: time.Second,
			MaxRetryPeriod:   time.Second,
		},
	)
	require.NoError(t, err)
	t.Cleanup(kubeServersWatcher.Close)

	// TLS config for kube proxy and Kube service.
	proxyServerIdentity, err := authtest.NewServerIdentity(authServer.AuthServer, testCtx.HostID, types.RoleProxy)
	require.NoError(t, err)
	proxyTLSConfig, err := proxyServerIdentity.TLSConfig(nil)
	require.NoError(t, err)
	require.Len(t, proxyTLSConfig.Certificates, 1)
	require.NotNil(t, proxyTLSConfig.RootCAs)

	// Create kubernetes service server.
	testCtx.KubeProxy, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			ReverseTunnelSrv: &reversetunnelclient.FakeServer{
				FakeClusters: []reversetunnelclient.Cluster{
					&fakeCluster{
						FakeCluster: reversetunnelclient.NewFakeCluster(testCtx.ClusterName, client),
						idToAddr: map[string]string{
							testCtx.HostID: testCtx.kubeServerListener.Addr().String(),
						},
					},
				},
			},
			Namespace:   apidefaults.Namespace,
			Keygen:      keyGen,
			ClusterName: testCtx.ClusterName,
			ScopedAuthz: testCtx.ScopedAuthz,
			// fileStreamer continues to write events after the server is shutdown and
			// races against os.RemoveAll leading the test to fail.
			// Using "node-sync" mode to write the events and session recordings
			// directly to AuthClient solves the issue.
			// We wrap the AuthClient with an events.TeeStreamer to send non-disk
			// events like session.end to testCtx.emitter as well.
			AuthClient: authClient,
			// StreamEmitter is required although not used because we are using
			// "node-sync" as session recording mode.
			Emitter:           testCtx.Emitter,
			DataDir:           t.TempDir(),
			CachingAuthClient: client,
			HostID:            testCtx.HostID,
			Context:           testCtx.Context,
			KubeServiceType:   ProxyService,
			Component:         teleport.ComponentKube,
			LockWatcher:       testCtx.lockWatcher,
			Clock:             clockwork.NewRealClock(),
			ClusterFeatures:   features,
			GetConnTLSCertificate: func() (*tls.Certificate, error) {
				return &proxyTLSConfig.Certificates[0], nil
			},
			GetConnTLSRoots: func() (*x509.CertPool, error) {
				return proxyTLSConfig.RootCAs, nil
			},
			PROXYSigner: &multiplexer.PROXYSigner{},
		},
		TLS:                      proxyTLSConfig.Clone(),
		AccessPoint:              proxyAccessPoint,
		KubernetesServersWatcher: kubeServersWatcher,
		LimiterConfig: limiter.Config{
			MaxConnections: 1000,
		},
		Log:             logtest.NewLogger(),
		InventoryHandle: inventoryHandle,
		GetRotation: func(role types.SystemRole) (*types.Rotation, error) {
			return &types.Rotation{}, nil
		},
		ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
		HealthCheckManager:   healthCheckManager,
	})
	require.NoError(t, err)
	require.Zero(t, testCtx.KubeServer.Server.ReadTimeout, "kube server read timeout must be 0 to keep long-running watch streams alive")
	require.Equal(t, defaults.HandshakeReadDeadline, testCtx.KubeServer.Server.WriteTimeout, "kube server write timeout must be HandshakeReadDeadline; it caps the TLS handshake while the outer handler resets the per-request write deadline")

	testCtx.startKubeServices(t)
	// Explicitly send a heartbeat for any configured clusters.
	for _, cluster := range cfg.Clusters {
		select {
		case sender := <-inventoryHandle.Sender():
			server, err := testCtx.KubeServer.GetServerInfo(scopes.QualifiedName{Name: cluster.Name, Scope: cfg.Scope})
			require.NoError(t, err)
			require.NoError(t, sender.Send(ctx, proto.InventoryHeartbeat_builder{
				KubernetesServer: server,
			}.Build()))
		case <-time.After(20 * time.Second):
			t.Fatal("timed out waiting for inventory handle sender")
		}
	}

	// Wait for kube servers to be initialized.
	require.NoError(t, kubeServersWatcher.WaitInitialization())

	// Ensure watcher has the correct list of clusters.
	require.Eventually(t, func() bool {
		return kubeServersWatcher.ResourceCount() == len(cfg.Clusters)
	}, 3*time.Second, time.Millisecond*100)

	return testCtx
}

// CloneTestContext creates a scoped kube service and proxy that share the
// expensive auth and proxy-side test infrastructure of an unscoped context.
// The clone owns its listeners, scoped client, inventory stream, watcher, and
// TLS servers; closing it does not close parent or its shared backend.
func CloneTestContext(t *testing.T, parent *TestContext, scope string) *TestContext {
	t.Helper()
	require.NotNil(t, parent)
	require.Empty(t, parent.Scope, "only unscoped TestContexts can be cloned")
	require.NotEmpty(t, scope)

	ctx, cancel := context.WithCancel(parent.Context)
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	clone := &TestContext{
		ClusterName:     parent.ClusterName,
		HostID:          uuid.NewString(),
		Scope:           scope,
		TLSServer:       parent.TLSServer,
		AuthServer:      parent.AuthServer,
		ScopedAuthz:     parent.ScopedAuthz,
		Emitter:         parent.Emitter,
		UploadHandler:   parent.UploadHandler,
		Context:         ctx,
		cancel:          cancel,
		heartbeatCtx:    heartbeatCtx,
		heartbeatCancel: heartbeatCancel,
		lockWatcher:     parent.lockWatcher,
		proxyAuthClient: parent.proxyAuthClient,
		kubeconfigPath:  parent.kubeconfigPath,
		keyGen:          parent.keyGen,
		clusterFeatures: parent.clusterFeatures,
		config:          parent.config,
		isClone:         true,
	}
	clone.config.Scope = scope
	t.Cleanup(func() { require.NoError(t, clone.Close()) })

	var err error
	clone.AuthClient, err = clone.TLSServer.NewClient(authtest.TestScopedServerID(types.RoleKube, clone.HostID, scope))
	require.NoError(t, err)

	client := newAuthClientWithStreamer(clone, clone.config.CreateAuditStreamErr)
	var authClient authclient.ClientI = client
	if clone.config.WrapAuthClient != nil {
		authClient = clone.config.WrapAuthClient(client)
	}
	var accessPoint authclient.ClientI = client
	if clone.config.WrapAuthClient != nil {
		accessPoint = clone.config.WrapAuthClient(client)
	}
	proxyAccessPoint := authclient.ClientI(clone.proxyAuthClient)
	if clone.config.WrapProxyAccessPoint != nil {
		proxyAccessPoint = clone.config.WrapProxyAccessPoint(proxyAccessPoint)
	}

	clone.kubeServerListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	clone.kubeProxyListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	inventoryHandle, err := inventory.NewDownstreamHandle(client.InventoryControlStream,
		func(context.Context) (*proto.UpstreamInventoryHello, error) {
			return proto.UpstreamInventoryHello_builder{
				ServerID: clone.HostID,
				Version:  teleport.Version,
				Services: types.SystemRoles{types.RoleKube}.StringSlice(),
				Hostname: "test",
				Scope:    scope,
			}.Build(), nil
		})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inventoryHandle.Close()) })

	kubeIdentity, err := authtest.NewScopedServerIdentity(clone.AuthServer, clone.HostID, scope, types.RoleKube)
	require.NoError(t, err)
	kubeTLS, err := kubeIdentity.TLSConfig(nil)
	require.NoError(t, err)

	clone.KubeServer, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			Namespace:         apidefaults.Namespace,
			Keygen:            clone.keyGen,
			ClusterName:       clone.ClusterName,
			ScopedAuthz:       clone.ScopedAuthz,
			AuthClient:        authClient,
			Emitter:           clone.Emitter,
			DataDir:           t.TempDir(),
			CachingAuthClient: accessPoint,
			HostID:            clone.HostID,
			Context:           clone.Context,
			KubeconfigPath:    clone.kubeconfigPath,
			KubeServiceType:   KubeService,
			Component:         teleport.ComponentKube,
			LockWatcher:       clone.lockWatcher,
			CheckImpersonationPermissions: func(context.Context, string, authztypes.SelfSubjectAccessReviewInterface) error {
				return nil
			},
			Clock:           clockwork.NewRealClock(),
			ClusterFeatures: clone.clusterFeatures,
			Scope:           scope,
		},
		TLS:                  kubeTLS.Clone(),
		AccessPoint:          accessPoint,
		LimiterConfig:        limiter.Config{MaxConnections: 1000},
		OnHeartbeat:          func(error) {},
		GetRotation:          func(types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ResourceMatchers:     clone.config.ResourceMatchers,
		OnReconcile:          clone.config.OnReconcile,
		Log:                  logtest.NewLogger(),
		InventoryHandle:      inventoryHandle,
		ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
		// Scoped kube services do not run health checks. The proxy still needs
		// a manager to satisfy its server configuration, but it never registers
		// this scoped service as a health-check target.
		HealthCheckManager: nil,
	})
	require.NoError(t, err)

	kubeServersWatcher, err := kubewatcher.NewProxyKubeServerWatcher(clone.Context, kubewatcher.ProxyKubeServerWatcherConfig{
		Logger:           logtest.NewLogger(),
		AccessPoint:      proxyAccessPoint,
		FallbackGetter:   clone.proxyAuthClient,
		PrimaryTimeout:   time.Second,
		FallbackInterval: time.Second,
		MaxRetryPeriod:   time.Second,
	})
	require.NoError(t, err)

	proxyIdentity, err := authtest.NewServerIdentity(clone.AuthServer, clone.HostID, types.RoleProxy)
	require.NoError(t, err)
	proxyTLS, err := proxyIdentity.TLSConfig(nil)
	require.NoError(t, err)

	clone.KubeProxy, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			ReverseTunnelSrv: &reversetunnelclient.FakeServer{FakeClusters: []reversetunnelclient.Cluster{&fakeCluster{
				FakeCluster: reversetunnelclient.NewFakeCluster(clone.ClusterName, client),
				idToAddr:    map[string]string{clone.HostID: clone.kubeServerListener.Addr().String()},
			}}},
			Namespace:             apidefaults.Namespace,
			Keygen:                clone.keyGen,
			ClusterName:           clone.ClusterName,
			ScopedAuthz:           clone.ScopedAuthz,
			AuthClient:            authClient,
			Emitter:               clone.Emitter,
			DataDir:               t.TempDir(),
			CachingAuthClient:     client,
			HostID:                clone.HostID,
			Context:               clone.Context,
			KubeServiceType:       ProxyService,
			Component:             teleport.ComponentKube,
			LockWatcher:           clone.lockWatcher,
			Clock:                 clockwork.NewRealClock(),
			ClusterFeatures:       clone.clusterFeatures,
			GetConnTLSCertificate: func() (*tls.Certificate, error) { return &proxyTLS.Certificates[0], nil },
			GetConnTLSRoots:       func() (*x509.CertPool, error) { return proxyTLS.RootCAs, nil },
			PROXYSigner:           &multiplexer.PROXYSigner{},
		},
		TLS:                      proxyTLS.Clone(),
		AccessPoint:              proxyAccessPoint,
		KubernetesServersWatcher: kubeServersWatcher,
		LimiterConfig:            limiter.Config{MaxConnections: 1000},
		Log:                      logtest.NewLogger(),
		InventoryHandle:          inventoryHandle,
		GetRotation:              func(types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ConnectedProxyGetter:     reversetunnel.NewConnectedProxyGetter(),
		HealthCheckManager:       parent.healthCheckManager,
	})
	require.NoError(t, err)

	clone.startKubeServices(t)
	for _, cluster := range clone.config.Clusters {
		select {
		case sender := <-inventoryHandle.Sender():
			server, err := clone.KubeServer.GetServerInfo(scopes.QualifiedName{Name: cluster.Name, Scope: scope})
			require.NoError(t, err)
			require.NoError(t, sender.Send(clone.Context, proto.InventoryHeartbeat_builder{KubernetesServer: server}.Build()))
		case <-time.After(20 * time.Second):
			t.Fatal("timed out waiting for inventory handle sender")
		}
	}
	require.NoError(t, kubeServersWatcher.WaitInitialization())
	require.Eventually(t, func() bool { return kubeServersWatcher.ResourceCount() >= len(clone.config.Clusters) }, 3*time.Second, 100*time.Millisecond)

	return clone
}

func TestCloneTestContext(t *testing.T) {
	ctx := t.Context()
	parent := SetupTestContext(ctx, t, TestConfig{
		Clusters: []KubeClusterConfig{{Name: "kube", APIEndpoint: "https://127.0.0.1:1"}},
		ScopesFeatures: scopes.Features{
			Enabled:         true,
			AgentPinEnabled: true,
		},
	})
	clone := CloneTestContext(t, parent, "/staging")

	require.Same(t, parent.TLSServer, clone.TLSServer)
	require.Same(t, parent.AuthServer, clone.AuthServer)
	require.Same(t, parent.ScopedAuthz, clone.ScopedAuthz)
	require.Same(t, parent.Emitter, clone.Emitter)
	require.Same(t, parent.UploadHandler, clone.UploadHandler)
	require.Same(t, parent.lockWatcher, clone.lockWatcher)
	require.Equal(t, parent.kubeconfigPath, clone.kubeconfigPath)
	require.NotEqual(t, parent.HostID, clone.HostID)
	require.NotSame(t, parent.AuthClient, clone.AuthClient)
	require.NotEqual(t, parent.KubeProxyAddress(), clone.KubeProxyAddress())

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		servers, err := parent.AuthServer.GetKubernetesServers(ctx)
		require.NoError(t, err)
		found := make(map[string]bool)
		for _, server := range servers {
			if server.GetCluster().GetName() == "kube" {
				found[server.GetCluster().GetScope()] = true
			}
		}
		require.True(t, found[""], "missing unscoped kube server")
		require.True(t, found["/staging"], "missing scoped kube server")
	}, 5*time.Second, 100*time.Millisecond)

	require.NoError(t, clone.Close())
	_, err := parent.AuthServer.GetClusterName(ctx)
	require.NoError(t, err, "closing a clone must not close the shared auth server")
}

// startKubeServices starts kube service and kube proxy to handle connections.
func (c *TestContext) startKubeServices(t *testing.T) {
	go func() {
		err := c.KubeServer.Serve(c.kubeServerListener)
		// ignore server closed error returned when .Close is called.
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return
		}
		assert.NoError(t, err)
	}()

	go func() {
		err := c.KubeProxy.Serve(c.kubeProxyListener)
		// ignore server closed error returned when .Close is called.
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return
		}
		assert.NoError(t, err)
	}()
}

// Close closes resources associated with the test context.
func (c *TestContext) Close() error {
	c.closeOnce.Do(func() {
		// Cancel the heartbeat context to stop validating heartbeat-not-found
		// errors while the service unregisters itself.
		if c.heartbeatCancel != nil {
			c.heartbeatCancel()
		}
		var errs []error
		if c.KubeServer != nil {
			errs = append(errs, c.KubeServer.Close())
		}
		if c.KubeProxy != nil {
			errs = append(errs, c.KubeProxy.Close())
		}
		if c.AuthClient != nil {
			errs = append(errs, c.AuthClient.Close())
		}
		c.cancel()

		// A clone borrows the auth server and proxy-side infrastructure from its
		// parent. It must only release the resources it created itself.
		if c.isClone {
			c.closeErr = trace.NewAggregate(errs...)
			return
		}
		if c.AuthServer != nil {
			errs = append(errs, c.AuthServer.Close())
		}
		c.closeErr = trace.NewAggregate(errs...)
	})
	return c.closeErr
}

// KubeProxyAddress returns the address of the kube proxy.
func (c *TestContext) KubeProxyAddress() string {
	return c.kubeProxyListener.Addr().String()
}

// RoleSpec defiens the role name and kube details to be created.
type RoleSpec struct {
	Name           string
	KubeUsers      []string
	KubeGroups     []string
	SessionRequire []*types.SessionRequirePolicy
	SessionJoin    []*types.SessionJoinPolicy
	SetupRoleFunc  func(types.Role) // If nil all pods are allowed.
}

// CreateUserWithTraitsAndRole creates Teleport user and role with specified names
func (c *TestContext) CreateUserWithTraitsAndRole(ctx context.Context, t *testing.T, username string, userTraits map[string][]string, roleSpec RoleSpec) (types.User, types.Role) {
	return c.CreateUserWithTraitsAndRoleVersion(ctx, t, username, userTraits, types.DefaultRoleVersion, roleSpec)
}

func (c *TestContext) CreateUserWithTraitsAndRoleVersion(ctx context.Context, t *testing.T, username string, userTraits map[string][]string, roleVersion string, roleSpec RoleSpec) (types.User, types.Role) {
	user, role, err := authtest.CreateUserAndRole(
		c.TLSServer.Auth(),
		username,
		[]string{roleSpec.Name},
		nil,
		authtest.WithUserMutator(func(user types.User) {
			user.SetTraits(userTraits)
		}),
		authtest.WithRoleVersion(roleVersion),
	)
	require.NoError(t, err)
	role.SetKubeUsers(types.Allow, roleSpec.KubeUsers)
	role.SetKubeGroups(types.Allow, roleSpec.KubeGroups)
	role.SetSessionRequirePolicies(roleSpec.SessionRequire)
	role.SetSessionJoinPolicies(roleSpec.SessionJoin)

	if roleSpec.SetupRoleFunc == nil {
		role.SetKubeResources(types.Allow, []types.KubernetesResource{{Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: ""}})
	} else {
		roleSpec.SetupRoleFunc(role)
	}
	upsertedRole, err := c.TLSServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, upsertedRole
}

// CreateUserAndRole creates Teleport user and role with specified names
func (c *TestContext) CreateUserAndRole(ctx context.Context, t *testing.T, username string, roleSpec RoleSpec) (types.User, types.Role) {
	return c.CreateUserAndRoleVersion(ctx, t, username, types.DefaultRoleVersion, roleSpec)
}

// CreateUserAndRoleVersion creates Teleport user and role with specified names and role version.
func (c *TestContext) CreateUserAndRoleVersion(ctx context.Context, t *testing.T, username, roleVersion string, roleSpec RoleSpec) (types.User, types.Role) {
	return c.CreateUserWithTraitsAndRoleVersion(ctx, t, username, nil, roleVersion, roleSpec)
}

// CreateUserScopedRole creates a Teleport user and a scoped role assigned
// to them.
func (c *TestContext) CreateUserAndScopedRole(t *testing.T, username, scope string, roleSpec *accessv1.ScopedRoleSpec) (types.User, *accessv1.CreateScopedRoleAssignmentResponse) {
	user, err := authtest.CreateUser(t.Context(), c.TLSServer.Auth(), username)
	require.NoError(t, err)

	return user, c.CreateAndAssignScopedRole(t, username, scope, roleSpec)
}

// CreateAndAssignScopedRole creates a scoped role and assigns it to the given username.
func (c *TestContext) CreateAndAssignScopedRole(t *testing.T, username, scope string, roleSpec *accessv1.ScopedRoleSpec) *accessv1.CreateScopedRoleAssignmentResponse {
	scopedAccess := c.TLSServer.Auth().ScopedAccess()
	role, err := scopedAccess.CreateScopedRole(t.Context(), accessv1.CreateScopedRoleRequest_builder{
		Role: accessv1.ScopedRole_builder{
			Kind:    access.KindScopedRole,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: username,
			}.Build(),
			Scope: scope,
			Spec:  roleSpec,
		}.Build(),
	}.Build())
	require.NoError(t, err)

	assignment, err := scopedAccess.CreateScopedRoleAssignment(t.Context(), accessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: accessv1.ScopedRoleAssignment_builder{
			Kind:    access.KindScopedRoleAssignment,
			Version: types.V1,
			SubKind: access.SubKindDynamic,
			Scope:   scope,
			Metadata: headerv1.Metadata_builder{
				Name: uuid.New().String(),
			}.Build(),
			Spec: accessv1.ScopedRoleAssignmentSpec_builder{
				User: username,
				Assignments: []*accessv1.Assignment{
					accessv1.Assignment_builder{
						Role:  scopes.QualifiedName{Scope: role.GetRole().GetScope(), Name: role.GetRole().GetMetadata().GetName()}.String(),
						Scope: scope,
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	return assignment
}

func newKubeConfigFile(t *testing.T, clusters ...KubeClusterConfig) string {
	tmpDir := t.TempDir()

	kubeConf := clientcmdapi.NewConfig()
	for _, cluster := range clusters {
		kubeConf.Clusters[cluster.Name] = &clientcmdapi.Cluster{
			Server:                cluster.APIEndpoint,
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[cluster.Name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[cluster.Name] = &clientcmdapi.Context{
			Cluster:  cluster.Name,
			AuthInfo: cluster.Name,
		}
	}
	kubeConfigLocation := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*kubeConf, kubeConfigLocation)
	require.NoError(t, err)
	return kubeConfigLocation
}

// GenTestKubeClientTLSCertOptions is a function that can be used to modify the
// identity used to generate the kube client certificate.
type GenTestKubeClientTLSCertOptions func(*tlsca.Identity)

// WithResourceAccessRequests adds resource access requests to the identity.
func WithResourceAccessRequests(r ...types.ResourceAccessID) GenTestKubeClientTLSCertOptions {
	return func(identity *tlsca.Identity) {
		identity.AllowedResourceAccessIDs = r
	}
}

// WithIdentityRoute allows the user to reset the identity's RouteToCluster
// and KubernetesCluster fields to empty strings. This is useful when the user
// wants to test path routing.
func WithIdentityRoute(routeToCluster, kubernetesCluster string) GenTestKubeClientTLSCertOptions {
	return func(identity *tlsca.Identity) {
		identity.RouteToCluster = routeToCluster
		identity.KubernetesCluster = kubernetesCluster
	}
}

// WithMFAVerified sets the MFAVerified identity field,
func WithMFAVerified() GenTestKubeClientTLSCertOptions {
	return func(i *tlsca.Identity) {
		i.MFAVerified = "fake"
	}
}

// GenTestKubeClientTLSCert generates a kube client to access kube service
func (c *TestContext) GenTestKubeClientTLSCert(t *testing.T, userName string, kubeClusterSQN scopes.QualifiedName, opts ...GenTestKubeClientTLSCertOptions) (*kubernetes.Clientset, *rest.Config) {
	client, _, cfg := c.GenTestKubeClientsTLSCert(t, userName, kubeClusterSQN, opts...)
	return client, cfg
}

// GenTestKubeClientsTLSCert generates a "regular" kube client and a dynamic one to access kube service
func (c *TestContext) GenTestKubeClientsTLSCert(t *testing.T, userName string, kubeClusterSQN scopes.QualifiedName, opts ...GenTestKubeClientTLSCertOptions) (*kubernetes.Clientset, *dynamic.DynamicClient, *rest.Config) {
	authServer := c.AuthServer
	clusterName, err := authServer.GetClusterName(t.Context())
	require.NoError(t, err)

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(context.Background(), userName, false)
	require.NoError(t, err)

	roles, err := services.FetchRoles(user.GetRoles(), authServer, user.GetTraits())
	require.NoError(t, err)

	ttl := roles.AdjustSessionTTL(10 * time.Minute)

	ca, err := authServer.GetCertAuthority(c.Context, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	require.NoError(t, err)

	caCert, signer, err := authServer.GetKeyStore().GetTLSCertAndSigner(c.Context, ca)
	require.NoError(t, err)

	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	require.NoError(t, err)

	priv, err := cryptosuites.GenerateKey(context.Background(),
		cryptosuites.GetCurrentSuiteFromAuthPreference(authServer),
		cryptosuites.UserTLS)
	require.NoError(t, err)
	// Sanity check we generated an ECDSA key.
	require.IsType(t, &ecdsa.PrivateKey{}, priv)
	privPEM, err := keys.MarshalPrivateKey(priv)
	require.NoError(t, err)

	id := tlsca.Identity{
		Username:          user.GetName(),
		Groups:            user.GetRoles(),
		KubernetesUsers:   user.GetKubeUsers(),
		KubernetesGroups:  user.GetKubeGroups(),
		KubernetesCluster: kubeClusterSQN.String(),
		RouteToCluster:    c.ClusterName,
		Traits:            user.GetTraits(),
	}
	for _, opt := range opts {
		opt(&id)
	}
	subj, err := id.Subject()
	require.NoError(t, err)

	cert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     authServer.GetClock(),
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  authServer.GetClock().Now().Add(ttl),
	})
	require.NoError(t, err)

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     ca.GetActiveKeys().TLS[0].Cert,
		CertData:   cert,
		KeyData:    privPEM,
		ServerName: "teleport.cluster.local",
	}
	restConfig := &rest.Config{
		Host:            "https://" + c.KubeProxyAddress(),
		TLSClientConfig: tlsClientConfig,
	}

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	dynClient, err := dynamic.NewForConfig(restConfig)
	require.NoError(t, err)

	return client, dynClient, restConfig
}

// NewJoiningSession creates a new session stream for joining an existing session.
func (c *TestContext) NewJoiningSession(cfg *rest.Config, sessionID string, mode types.SessionParticipantMode) (*streamproto.SessionStream, error) {
	ws, err := newWebSocketClient(cfg, http.MethodPost, &url.URL{
		Scheme: "wss",
		Host:   c.KubeProxyAddress(),
		Path:   "/api/v1/teleport/join/" + sessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = ws.connectViaWebsocket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stream, err := streamproto.NewSessionStream(ws.conn, streamproto.ClientHandshake{Mode: mode})
	return stream, trace.Wrap(err)
}

// authClientWithStreamer wraps auth.Client and replaces the CreateAuditStream
// and ResumeAuditStream methods to use a events.TeeStreamer to leverage the StreamEmitter
// even when recording mode is *-sync.
type authClientWithStreamer struct {
	*authclient.Client
	streamer             events.Streamer
	createAuditStreamErr error
}

// newAuthClientWithStreamer creates a new authClient wrapper.
func newAuthClientWithStreamer(testCtx *TestContext, createAuditStreamErr error) *authClientWithStreamer {
	return &authClientWithStreamer{Client: testCtx.AuthClient, streamer: testCtx.AuthClient, createAuditStreamErr: createAuditStreamErr}
}

func (a *authClientWithStreamer) CreateAuditStream(ctx context.Context, sID sessPkg.ID) (apievents.Stream, error) {
	if a.createAuditStreamErr != nil {
		return nil, trace.Wrap(a.createAuditStreamErr)
	}
	return a.streamer.CreateAuditStream(ctx, sID)
}

func (a *authClientWithStreamer) ResumeAuditStream(ctx context.Context, sID sessPkg.ID, uploadID string) (apievents.Stream, error) {
	return a.streamer.ResumeAuditStream(ctx, sID, uploadID)
}

type fakeClient struct {
	authclient.ClientI
	closeC chan struct{}
}

func (f *fakeClient) CreateSessionTracker(ctx context.Context, st types.SessionTracker) (types.SessionTracker, error) {
	select {
	case <-f.closeC:
		return nil, trace.ConnectionProblem(nil, "closed")
	default:
		return f.ClientI.CreateSessionTracker(ctx, st)
	}
}

// fakeCluster is a fake cluster that uses a map to map server IDs to
// addresses to simulate reverse tunneling.
type fakeCluster struct {
	*reversetunnelclient.FakeCluster
	idToAddr map[string]string
}

func (f *fakeCluster) DialTCP(p reversetunnelclient.DialParams) (conn net.Conn, err error) {
	// The server ID is the first part of the address.
	addr, ok := f.idToAddr[strings.Split(p.ServerID, ".")[0]]
	if !ok {
		return nil, trace.NotFound("server %q not found", p.ServerID)
	}
	conn, err = net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	return conn, nil
}

func (c *TestContext) GetScopePinForUser(t *testing.T, username, scope string) *scopesv1.Pin {
	pin := scopesv1.Pin_builder{
		Kind:  scopesv1.PinKind_PIN_KIND_USER,
		Scope: scope,
	}.Build()
	err := c.AuthServer.ScopedAccessCache.PopulatePinnedAssignmentsForUser(t.Context(), username, pin)
	require.NoError(t, err)

	return pin
}
