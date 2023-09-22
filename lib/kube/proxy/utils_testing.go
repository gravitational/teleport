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
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	sessPkg "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
)

type TestContext struct {
	HostID      string
	ClusterName string
	TLSServer   *auth.TestTLSServer
	AuthServer  *auth.Server
	AuthClient  *auth.Client
	Authz       authz.Authorizer
	KubeServer  *TLSServer
	Emitter     *eventstest.ChannelEmitter
	Context     context.Context
	listener    net.Listener
	cancel      context.CancelFunc
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
	Clusters         []KubeClusterConfig
	ResourceMatchers []services.ResourceMatcher
	OnReconcile      func(types.KubeClusters)
	OnEvent          func(apievents.AuditEvent)
}

// SetupTestContext creates a kube service with clusters configured.
func SetupTestContext(ctx context.Context, t *testing.T, cfg TestConfig) *TestContext {
	ctx, cancel := context.WithCancel(ctx)
	testCtx := &TestContext{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		Context:     ctx,
		cancel:      cancel,
	}
	t.Cleanup(func() { testCtx.Close() })

	kubeConfigLocation := newKubeConfigFile(ctx, t, cfg.Clusters...)

	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: testCtx.ClusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	testCtx.TLSServer, err = authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.TLSServer.Close()) })

	testCtx.AuthServer = testCtx.TLSServer.Auth()

	// Use sync recording to not involve the uploader.
	recConfig, err := authServer.AuthServer.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)
	// Always use *-sync to prevent fileStreamer from running against os.RemoveAll
	// once the test ends.
	recConfig.SetMode(types.RecordAtNodeSync)
	err = authServer.AuthServer.SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Auth client for Kube service.
	testCtx.AuthClient, err = testCtx.TLSServer.NewClient(auth.TestServerID(types.RoleKube, testCtx.HostID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.AuthClient.Close()) })

	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := testCtx.TLSServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	proxyLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    proxyAuthClient,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		proxyLockWatcher.Close()
	})
	testCtx.Authz, err = authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testCtx.ClusterName,
		AccessPoint: proxyAuthClient,
		LockWatcher: proxyLockWatcher,
	})
	require.NoError(t, err)

	// TLS config for kube proxy and Kube service.
	serverIdentity, err := auth.NewServerIdentity(authServer.AuthServer, testCtx.HostID, types.RoleKube)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
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
	keyGen := keygen.New(testCtx.Context)

	// heartbeatsWaitChannel waits for clusters heartbeats to start.
	heartbeatsWaitChannel := make(chan struct{}, len(cfg.Clusters)+1)
	client := newAuthClientWithStreamer(testCtx)
	// Create kubernetes service server.
	testCtx.KubeServer, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			Namespace:   apidefaults.Namespace,
			Keygen:      keyGen,
			ClusterName: testCtx.ClusterName,
			Authz:       testCtx.Authz,
			// fileStreamer continues to write events after the server is shutdown and
			// races against os.RemoveAll leading the test to fail.
			// Using "node-sync" mode to write the events and session recordings
			// directly to AuthClient solves the issue.
			// We wrap the AuthClient with an events.TeeStreamer to send non-disk
			// events like session.end to testCtx.emitter as well.
			AuthClient: client,
			// StreamEmitter is required although not used because we are using
			// "node-sync" as session recording mode.
			StreamEmitter:     testCtx.Emitter,
			DataDir:           t.TempDir(),
			CachingAuthClient: client,
			HostID:            testCtx.HostID,
			Context:           testCtx.Context,
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
		AccessPoint:   client,
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
		ResourceMatchers: cfg.ResourceMatchers,
		OnReconcile:      cfg.OnReconcile,
	})
	require.NoError(t, err)

	// Waits for len(clusters) heartbeats to start
	waitForHeartbeats := len(cfg.Clusters)

	testCtx.startKubeService(t)

	for i := 0; i < waitForHeartbeats; i++ {
		<-heartbeatsWaitChannel
	}

	return testCtx
}

// startKubeService starts kube service to handle connections.
func (c *TestContext) startKubeService(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c.listener = listener
	go func() {
		err := c.KubeServer.Serve(listener)
		// ignore server closed error returned when .Close is called.
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		assert.NoError(t, err)
	}()
}

// Close closes resources associated with the test context.
func (c *TestContext) Close() error {
	// kubeServer closes the listener
	err := c.KubeServer.Close()
	authCErr := c.AuthClient.Close()
	authSErr := c.AuthServer.Close()
	c.cancel()
	return trace.NewAggregate(err, authCErr, authSErr)
}

// KubeServiceAddress returns the address of the kube service
func (c *TestContext) KubeServiceAddress() string {
	return c.listener.Addr().String()
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

// CreateUserAndRole creates Teleport user and role with specified names
func (c *TestContext) CreateUserAndRole(ctx context.Context, t *testing.T, username string, roleSpec RoleSpec) (types.User, types.Role) {
	user, role, err := auth.CreateUserAndRole(c.TLSServer.Auth(), username, []string{roleSpec.Name})
	require.NoError(t, err)
	role.SetKubeUsers(types.Allow, roleSpec.KubeUsers)
	role.SetKubeGroups(types.Allow, roleSpec.KubeGroups)
	role.SetSessionRequirePolicies(roleSpec.SessionRequire)
	role.SetSessionJoinPolicies(roleSpec.SessionJoin)
	if roleSpec.SetupRoleFunc == nil {
		role.SetKubeResources(types.Allow, []types.KubernetesResource{{Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard}})
	} else {
		roleSpec.SetupRoleFunc(role)
	}
	err = c.TLSServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, role
}

func newKubeConfigFile(ctx context.Context, t *testing.T, clusters ...KubeClusterConfig) string {
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
func WithResourceAccessRequests(r ...types.ResourceID) GenTestKubeClientTLSCertOptions {
	return func(identity *tlsca.Identity) {
		identity.AllowedResourceIDs = r
	}
}

// GenTestKubeClientTLSCert generates a kube client to access kube service
func (c *TestContext) GenTestKubeClientTLSCert(t *testing.T, userName, kubeCluster string, opts ...GenTestKubeClientTLSCertOptions) (*kubernetes.Clientset, *rest.Config) {
	authServer := c.AuthServer
	clusterName, err := authServer.GetClusterName()
	require.NoError(t, err)

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(userName, false)
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
		RouteToCluster:    c.ClusterName,
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
		Host:            "https://" + c.KubeServiceAddress(),
		TLSClientConfig: tlsClientConfig,
	}

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	return client, restConfig
}

// NewJoiningSession creates a new session stream for joining an existing session.
func (c *TestContext) NewJoiningSession(cfg *rest.Config, sessionID string, mode types.SessionParticipantMode) (*streamproto.SessionStream, error) {
	ws, err := newWebSocketClient(cfg, http.MethodPost, &url.URL{
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

// authClientWithStreamer wraps auth.Client and replaces the CreateAuditStream
// and ResumeAuditStream methods to use a events.TeeStreamer to leverage the StreamEmitter
// even when recording mode is *-sync.
type authClientWithStreamer struct {
	*auth.Client
	streamer *events.TeeStreamer
}

// newAuthClientWithStreamer creates a new authClient wrapper.
func newAuthClientWithStreamer(testCtx *TestContext) *authClientWithStreamer {
	return &authClientWithStreamer{Client: testCtx.AuthClient, streamer: events.NewTeeStreamer(testCtx.AuthClient, testCtx.Emitter)}
}

func (a *authClientWithStreamer) CreateAuditStream(ctx context.Context, sID sessPkg.ID) (apievents.Stream, error) {
	return a.streamer.CreateAuditStream(ctx, sID)
}

func (a *authClientWithStreamer) ResumeAuditStream(ctx context.Context, sID sessPkg.ID, uploadID string) (apievents.Stream, error) {
	return a.streamer.ResumeAuditStream(ctx, sID, uploadID)
}
