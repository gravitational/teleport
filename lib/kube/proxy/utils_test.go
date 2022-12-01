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

package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type testContext struct {
	hostID      string
	clusterName string
	tlsServer   *auth.TestTLSServer
	authServer  *auth.Server
	authClient  *auth.Client
	kubeServer  *TLSServer
	emitter     *eventstest.ChannelEmitter
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
}

// kubeClusterConfig defines the cluster to be created
type kubeClusterConfig struct {
	name        string
	apiEndpoint string
}

// testConfig defines the suite options.
type testConfig struct {
	clusters         []kubeClusterConfig
	resourceMatchers []services.ResourceMatcher
	onReconcile      func(types.KubeClusters)
	onEvent          func(apievents.AuditEvent)
}

// setupTestContext creates a kube service with clusters configured.
func setupTestContext(ctx context.Context, t *testing.T, cfg testConfig) *testContext {
	ctx, cancel := context.WithCancel(ctx)
	testCtx := &testContext{
		clusterName: "root.example.com",
		hostID:      uuid.New().String(),
		ctx:         ctx,
		cancel:      cancel,
	}
	t.Cleanup(func() { testCtx.Close() })

	kubeConfigLocation := newKubeConfigFile(ctx, t, cfg.clusters...)

	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: testCtx.clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	testCtx.tlsServer, err = authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.tlsServer.Close()) })

	testCtx.authServer = testCtx.tlsServer.Auth()

	// Use sync recording to not involve the uploader.
	recConfig, err := authServer.AuthServer.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)
	recConfig.SetMode(types.RecordAtNode)
	err = authServer.AuthServer.SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Auth client for Kube service.
	testCtx.authClient, err = testCtx.tlsServer.NewClient(auth.TestServerID(types.RoleKube, testCtx.hostID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.authClient.Close()) })

	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := testCtx.tlsServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	proxyLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    proxyAuthClient,
		},
	})
	require.NoError(t, err)
	proxyAuthorizer, err := auth.NewAuthorizer(testCtx.clusterName, proxyAuthClient, proxyLockWatcher)
	require.NoError(t, err)

	// TLS config for kube proxy and Kube service.
	serverIdentity, err := auth.NewServerIdentity(authServer.AuthServer, testCtx.hostID, types.RoleKube)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Create test audit events emitter.
	testCtx.emitter = eventstest.NewChannelEmitter(100)
	go func() {
		for {
			select {
			case evt := <-testCtx.emitter.C():
				if cfg.onEvent != nil {
					cfg.onEvent(evt)
				}
			case <-testCtx.ctx.Done():
				return
			}
		}
	}()
	keyGen := native.New(testCtx.ctx)

	// heartbeatsWaitChannel waits for clusters heartbeats to start.
	heartbeatsWaitChannel := make(chan struct{}, len(cfg.clusters)+1)

	// Create kubernetes service server.
	testCtx.kubeServer, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			Namespace:         apidefaults.Namespace,
			Keygen:            keyGen,
			ClusterName:       testCtx.clusterName,
			Authz:             proxyAuthorizer,
			AuthClient:        testCtx.authClient,
			StreamEmitter:     testCtx.emitter,
			DataDir:           t.TempDir(),
			CachingAuthClient: testCtx.authClient,
			HostID:            testCtx.hostID,
			Context:           testCtx.ctx,
			KubeconfigPath:    kubeConfigLocation,
			KubeServiceType:   KubeService,
			Component:         teleport.ComponentKube,
			LockWatcher:       proxyLockWatcher,
			// skip Impersonation validation
			CheckImpersonationPermissions: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
				return nil
			},
			Clock: clockwork.NewRealClock(),
		},
		DynamicLabels: nil,
		TLS:           tlsConfig,
		AccessPoint:   testCtx.authClient,
		LimiterConfig: limiter.Config{
			MaxConnections:   1000,
			MaxNumberOfUsers: 1000,
		},
		// each time heartbeat is called we insert data into the channel.
		// this is used to make sure that heartbeat started and the clusters
		// are registered in the auth server
		OnHeartbeat: func(err error) {
			require.NoError(t, err)
			select {
			case heartbeatsWaitChannel <- struct{}{}:
			default:
			}
		},
		GetRotation:      func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ResourceMatchers: cfg.resourceMatchers,
		OnReconcile:      cfg.onReconcile,
	})
	require.NoError(t, err)
	// create session recording path
	// testCtx.kubeServer.DataDir/log/upload/streaming/default
	err = os.MkdirAll(
		filepath.Join(
			testCtx.kubeServer.DataDir,
			teleport.LogsDir,
			teleport.ComponentUpload,
			events.StreamingLogsDir,
			apidefaults.Namespace,
		), os.ModePerm)
	require.NoError(t, err)

	// Waits for len(clusters) heartbeats to start
	waitForHeartbeats := len(cfg.clusters)

	testCtx.startKubeService(t)

	for i := 0; i < waitForHeartbeats; i++ {
		<-heartbeatsWaitChannel
	}

	return testCtx
}

// startKubeService starts kube service to handle connections.
func (c *testContext) startKubeService(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	go func() {
		err := c.kubeServer.Serve(listener)
		// ignore server closed error returned when .Close is called.
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		assert.NoError(t, err)
	}()
}

// Close closes resources associated with the test context.
func (c *testContext) Close() error {
	// kubeServer closes the listener
	err := c.kubeServer.Close()
	authCErr := c.authClient.Close()
	authSErr := c.authServer.Close()
	c.cancel()
	return trace.NewAggregate(err, authCErr, authSErr)
}

// KubeServiceAddress returns the address of the kube service
func (c *testContext) KubeServiceAddress() string {
	return c.listener.Addr().String()
}

// roleSpec defiens the role name and kube details to be created.
type roleSpec struct {
	name           string
	kubeUsers      []string
	kubeGroups     []string
	sessionRequire []*types.SessionRequirePolicy
	sessionJoin    []*types.SessionJoinPolicy
}

// createUserAndRole creates Teleport user and role with specified names
func (c *testContext) createUserAndRole(ctx context.Context, t *testing.T, username string, roleSpec roleSpec) (types.User, types.Role) {
	user, role, err := auth.CreateUserAndRole(c.tlsServer.Auth(), username, []string{roleSpec.name})
	require.NoError(t, err)
	role.SetKubeUsers(types.Allow, roleSpec.kubeUsers)
	role.SetKubeGroups(types.Allow, roleSpec.kubeGroups)
	role.SetSessionRequirePolicies(roleSpec.sessionRequire)
	role.SetSessionJoinPolicies(roleSpec.sessionJoin)
	err = c.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, role
}

func newKubeConfigFile(ctx context.Context, t *testing.T, clusters ...kubeClusterConfig) string {
	tmpDir := t.TempDir()

	kubeConf := clientcmdapi.NewConfig()
	for _, cluster := range clusters {
		kubeConf.Clusters[cluster.name] = &clientcmdapi.Cluster{
			Server:                cluster.apiEndpoint,
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[cluster.name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[cluster.name] = &clientcmdapi.Context{
			Cluster:  cluster.name,
			AuthInfo: cluster.name,
		}
	}
	kubeConfigLocation := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*kubeConf, kubeConfigLocation)
	require.NoError(t, err)
	return kubeConfigLocation
}

// genTestKubeClientTLSCert generates a kube client to access kube service
func (c *testContext) genTestKubeClientTLSCert(t *testing.T, userName, kubeCluster string) (*kubernetes.Clientset, *rest.Config) {
	authServer := c.authServer
	clusterName, err := authServer.GetClusterName()
	require.NoError(t, err)

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(userName, false)
	require.NoError(t, err)

	roles, err := services.FetchRoles(user.GetRoles(), authServer, user.GetTraits())
	require.NoError(t, err)

	ttl := roles.AdjustSessionTTL(10 * time.Minute)

	ca, err := authServer.GetCertAuthority(c.ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	require.NoError(t, err)

	caCert, signer, err := authServer.GetKeyStore().GetTLSCertAndSigner(c.ctx, ca)
	require.NoError(t, err)

	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	require.NoError(t, err)

	privPEM, _, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	priv, err := tlsca.ParsePrivateKeyPEM(privPEM)
	require.NoError(t, err)

	id := tlsca.Identity{
		Username:          user.GetName(),
		Groups:            user.GetRoles(),
		KubernetesUsers:   user.GetKubeUsers(),
		KubernetesGroups:  user.GetKubeGroups(),
		KubernetesCluster: kubeCluster,
		RouteToCluster:    c.clusterName,
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
		Host:            "https://" + c.KubeServiceAddress(),
		TLSClientConfig: tlsClientConfig,
	}

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	return client, restConfig
}

func (c *testContext) newJoiningSession(cfg *rest.Config, sessionID string, mode types.SessionParticipantMode) (*streamproto.SessionStream, error) {
	ws, err := newWebSocketExecutor(cfg, http.MethodPost, &url.URL{
		Scheme: "wss",
		Host:   c.KubeServiceAddress(),
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
