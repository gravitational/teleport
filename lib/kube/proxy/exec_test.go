package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/events/eventstest"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	testingwebsocket "github.com/gravitational/teleport/lib/kube/proxy/testing/websocket"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	kubeCluster     = "test_cluster"
	userName        = "test_user"
	roleName        = "kube_role"
	kubeGroups      = []string{"kube"}
	kubeUsers       = []string{"kube"}
	podName         = "teleport"
	podNamespace    = "default"
	containerName   = "teleport"
	commmandExecute = []string{"sh"}
	execMethod      = "POST"
	stdInContent    = []byte("stdin_data")
)

func TestExecKubeService(t *testing.T) {
	kubeMockSrv, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMockSrv.Close() })
	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := setupTestContext(context.TODO(), t, withKubeCluster(kubeCluster, kubeMockSrv.URL))

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	user, _ := testCtx.createUserAndRole(context.TODO(), t, userName, roleName, kubeUsers, kubeGroups)

	// generate a kube client with user certs for auth
	_, config := testCtx.genTestKubeClientTLSCert(
		t,
		user.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

	t.Run("spdy_protocol", func(t *testing.T) {
		var (
			stdInWrite = &bytes.Buffer{}
			stdOut     = &bytes.Buffer{}
			stdErr     = &bytes.Buffer{}
		)

		_, err = stdInWrite.Write(stdInContent)
		require.NoError(t, err)
		stdIn := io.NopCloser(stdInWrite)

		url, err := generateExecURL(
			testCtx.kubeServiceAddress,
			podName,
			podNamespace,
			containerName,
			commmandExecute,
			stdIn,
			stdErr,
			stdOut,
			false,
		)
		require.NoError(t, err)

		exec, err := remotecommand.NewSPDYExecutor(config, execMethod, url)
		require.NoError(t, err)

		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  stdIn,
			Stdout: stdOut,
			Stderr: stdErr,
			Tty:    false,
		})
		require.NoError(t, err)

		require.Equal(t, fmt.Sprintf("%s\n%s", containerName, string(stdInContent)), stdOut.String())
		require.Equal(t, fmt.Sprintf("%s\n%s", containerName, string(stdInContent)), stdErr.String())
	})

	t.Run("websocket_protocol", func(t *testing.T) {
		var (
			stdIn  = io.NopCloser(&bytes.Buffer{})
			stdOut = &bytes.Buffer{}
			stdErr = &bytes.Buffer{}
		)

		url, err := generateExecURL(
			testCtx.kubeServiceAddress,
			podName,
			podNamespace,
			containerName,
			commmandExecute,
			stdIn,
			stdErr,
			stdOut,
			false,
		)
		require.NoError(t, err)
		// we do not validate the websocket payload because go client does not supoort websocket for now.
		// once https://github.com/kubernetes/kubernetes/pull/110142 is merged we can use the websocket client
		require.NoError(t, testingwebsocket.CheckIfWebSocketsAreSupported(config, execMethod, url))
	})

}

func generateExecURL(addr, podName, podNamespace, containerName string, cmd []string, stdIn io.ReadCloser, stdErr, stdOut io.Writer, tty bool) (*url.URL, error) {
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    addr,
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{

			GroupVersion:         &corev1.SchemeGroupVersion,
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
		},

		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	})
	if err != nil {
		return nil, err
	}

	req := restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(podNamespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     stdIn != nil,
		Stdout:    stdOut != nil,
		Stderr:    stdErr != nil,
		TTY:       tty,
	}, scheme.ParameterCodec)

	return req.URL(), nil
}

type testContext struct {
	hostID             string
	clusterName        string
	tlsServer          *auth.TestTLSServer
	authServer         *auth.Server
	authClient         *auth.Client
	kubeConfig         string
	kubeServiceAddress string
	kubeServer         *TLSServer
	emitter            *eventstest.ChannelEmitter
	// clock to override clock in tests.
	clock     clockwork.FakeClock
	ctx       context.Context
	cancelCtx context.CancelFunc
	tempDir   string
}

// startKubeService starts kube service to handle connections.
func (c *testContext) startKubeService(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go c.kubeServer.Serve(listener)
}

// createUserAndRole creates Teleport user and role with specified names
func (c *testContext) createUserAndRole(ctx context.Context, t *testing.T, userName, roleName string, kubeUsers, kubeGroups []string) (types.User, types.Role) {
	user, role, err := auth.CreateUserAndRole(c.tlsServer.Auth(), userName, []string{roleName})
	require.NoError(t, err)
	role.SetKubeUsers(types.Allow, kubeUsers)
	role.SetKubeGroups(types.Allow, kubeGroups)
	err = c.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, role
}

// Close closes resources associated with the test context.
func (c *testContext) Close() error {
	c.cancelCtx()
	return c.kubeServer.Close()
}

func newKubeConfigFile(ctx context.Context, t *testing.T, clusters ...withKubernetesOption) (string, string) {
	dir := t.TempDir()
	t.Cleanup(func() { os.RemoveAll(dir) })

	f, err := os.CreateTemp(dir, "*")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	kubeConf := clientcmdapi.NewConfig()
	for _, cluster := range clusters {
		cluster(t, ctx, kubeConf)
	}
	err = clientcmd.WriteToFile(*kubeConf, f.Name())
	require.NoError(t, err)
	return dir, f.Name()
}

func setupTestContext(ctx context.Context, t *testing.T, clusters ...withKubernetesOption) *testContext {
	testCtx := &testContext{
		clusterName: "root.example.com",
		hostID:      uuid.New().String(),
		clock:       clockwork.NewFakeClockAt(time.Now()),
	}
	t.Cleanup(func() { testCtx.Close() })

	testCtx.tempDir, testCtx.kubeConfig = newKubeConfigFile(ctx, t, clusters...)

	testCtx.ctx, testCtx.cancelCtx = context.WithCancel(ctx)
	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       testCtx.clock,
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
	recConfig.SetMode(types.RecordAtNodeSync)
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
	keyGen := native.New(testCtx.ctx)
	// Create kubernetes service server.
	testCtx.kubeServer, err = NewTLSServer(TLSServerConfig{
		ForwarderConfig: ForwarderConfig{
			Namespace:         apidefaults.Namespace,
			Keygen:            keyGen,
			ClusterName:       testCtx.clusterName,
			Authz:             proxyAuthorizer,
			AuthClient:        testCtx.authClient,
			StreamEmitter:     testCtx.authClient,
			DataDir:           testCtx.tempDir,
			CachingAuthClient: testCtx.authClient,
			HostID:            testCtx.hostID,
			Context:           testCtx.ctx,
			KubeconfigPath:    testCtx.kubeConfig,
			KubeServiceType:   KubeService,
			Component:         teleport.ComponentKube,
			DynamicLabels:     nil,
			LockWatcher:       proxyLockWatcher,
			// skip Impersonation validation
			CheckImpersonationPermissions: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
				return nil
			},
			Clock: clockwork.NewRealClock(),
		},
		TLS:         tlsConfig,
		AccessPoint: testCtx.authClient,
		// Empty config means no limit.
		LimiterConfig: limiter.Config{
			MaxConnections:   1000,
			MaxNumberOfUsers: 1000,
		},
		OnHeartbeat: func(err error) {},
		GetRotation: func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
	})
	require.NoError(t, err)

	testCtx.startKubeService(t)

	time.Sleep(1 * time.Second)

	testCtx.kubeServiceAddress = testCtx.kubeServer.listener.Addr().String()
	return testCtx
}

type withKubernetesOption func(t *testing.T, ctx context.Context, kubeConf *clientcmdapi.Config)

func withKubeCluster(name, addr string) withKubernetesOption {
	return func(t *testing.T, ctx context.Context, kubeConf *clientcmdapi.Config) {

		kubeConf.Clusters[name] = &clientcmdapi.Cluster{
			Server:                addr,
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[name] = &clientcmdapi.Context{
			Cluster:  name,
			AuthInfo: name,
		}
	}
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

	ca, err := authServer.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	require.NoError(t, err)

	caCert, signer, err := authServer.GetKeyStore().GetTLSCertAndSigner(ca)
	require.NoError(t, err)

	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	require.NoError(t, err)

	privPEM, _, err := native.GenerateKeyPair()
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
		Host:            "https://" + c.kubeServiceAddress,
		TLSClientConfig: tlsClientConfig,
	}

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	return client, restConfig
}
