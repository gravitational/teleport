/*
Copyright 2021 Gravitational, Inc.

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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/breaker"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

type Suite struct {
	root *helpers.TeleInstance
	leaf *helpers.TeleInstance
}

type suiteOptions struct {
	rootConfigFunc func(suite *Suite) *servicecfg.Config
	leafConfigFunc func(suite *Suite) *servicecfg.Config

	rootConfigModFunc []func(config *servicecfg.Config)
	leafConfigModFunc []func(config *servicecfg.Config)

	rootClusterNodeName string
	leafClusterNodeName string

	rootClusterListeners helpers.InstanceListenerSetupFunc
	leafClusterListeners helpers.InstanceListenerSetupFunc

	rootTrustedSecretFunc func(suite *Suite) []*helpers.InstanceSecrets
	leafTrustedFunc       func(suite *Suite) []*helpers.InstanceSecrets

	rootClusterRoles      []types.Role
	leafClusterRoles      []types.Role
	updateRoleMappingFunc func(t *testing.T, suite *Suite)

	trustedCluster types.TrustedCluster
}

func newSuite(t *testing.T, opts ...proxySuiteOptionsFunc) *Suite {
	options := suiteOptions{
		rootClusterNodeName:  helpers.Host,
		leafClusterNodeName:  helpers.Host,
		rootClusterListeners: helpers.SingleProxyPortSetupOn(helpers.Host),
		leafClusterListeners: helpers.SingleProxyPortSetupOn(helpers.Host),
	}
	for _, opt := range opts {
		opt(&options)
	}

	rCfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    options.rootClusterNodeName,
		Log:         utils.NewLoggerForTests(),
	}
	rCfg.Listeners = options.rootClusterListeners(t, &rCfg.Fds)
	rc := helpers.NewInstance(t, rCfg)

	// Create leaf cluster.
	lCfg := helpers.InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
		NodeName:    options.leafClusterNodeName,
		Priv:        rc.Secrets.PrivKey,
		Pub:         rc.Secrets.PubKey,
		Log:         utils.NewLoggerForTests(),
	}
	lCfg.Listeners = options.leafClusterListeners(t, &lCfg.Fds)
	lc := helpers.NewInstance(t, lCfg)
	suite := &Suite{
		root: rc,
		leaf: lc,
	}

	user := helpers.MustGetCurrentUser(t)
	for _, role := range options.rootClusterRoles {
		rc.AddUserWithRole(user.Username, role)
	}
	for _, role := range options.leafClusterRoles {
		lc.AddUserWithRole(user.Username, role)
	}

	rootTrustedSecrets := lc.Secrets.AsSlice()
	if options.rootTrustedSecretFunc != nil {
		rootTrustedSecrets = options.rootTrustedSecretFunc(suite)
	}

	rootConfig := options.rootConfigFunc(suite)
	for _, v := range options.rootConfigModFunc {
		v(rootConfig)
	}
	err := rc.CreateEx(t, rootTrustedSecrets, rootConfig)
	require.NoError(t, err)

	leafTrustedSecrets := rc.Secrets.AsSlice()
	if options.leafTrustedFunc != nil {
		leafTrustedSecrets = options.leafTrustedFunc(suite)
	}
	leafConfig := options.leafConfigFunc(suite)
	for _, v := range options.leafConfigModFunc {
		v(leafConfig)
	}
	err = lc.CreateEx(t, leafTrustedSecrets, leafConfig)
	require.NoError(t, err)

	require.NoError(t, rc.Start())
	t.Cleanup(func() {
		rc.StopAll()
	})

	require.NoError(t, lc.Start())
	t.Cleanup(func() {
		lc.StopAll()
	})

	if options.updateRoleMappingFunc != nil {
		options.updateRoleMappingFunc(t, suite)
	}

	if options.trustedCluster != nil {
		helpers.TryCreateTrustedCluster(t, suite.leaf.Process.GetAuthServer(), options.trustedCluster)
		helpers.WaitForTunnelConnections(t, suite.root.Process.GetAuthServer(), suite.leaf.Secrets.SiteName, 1)
	}

	return suite
}

func (p *Suite) addNodeToLeafCluster(t *testing.T, tunnelNodeHostname string) {
	nodeConfig := func() *servicecfg.Config {
		tconf := servicecfg.MakeDefaultConfig()
		tconf.Console = nil
		tconf.Log = utils.NewLoggerForTests()
		tconf.Hostname = tunnelNodeHostname
		tconf.SetToken("token")
		tconf.SetAuthServerAddress(utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        p.leaf.Web,
		})
		tconf.Auth.Enabled = false
		tconf.Proxy.Enabled = false
		tconf.SSH.Enabled = true
		tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
		return tconf
	}
	_, err := p.leaf.StartNode(nodeConfig())
	require.NoError(t, err)

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, helpers.WaitForClusters(p.root.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")
	require.Eventually(t, helpers.WaitForClusters(p.leaf.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	// Wait for both nodes to show up before attempting to dial to them.
	err = helpers.WaitForNodeCount(context.Background(), p.root, p.leaf.Secrets.SiteName, 2)
	require.NoError(t, err)
}

func (p *Suite) mustConnectToClusterAndRunSSHCommand(t *testing.T, config helpers.ClientConfig) {
	const (
		deadline         = time.Second * 20
		nextIterWaitTime = time.Millisecond * 100
	)

	tc, err := p.root.NewClient(config)
	require.NoError(t, err)

	output := &bytes.Buffer{}
	tc.Stdout = output
	require.NoError(t, err)

	cmd := []string{"echo", "hello world"}
	err = retryutils.RetryStaticFor(deadline, nextIterWaitTime, func() error {
		err = tc.SSH(context.TODO(), cmd, false)
		return trace.Wrap(err)
	})
	require.NoError(t, err)

	require.Equal(t, "hello world\n", output.String())
}

type proxySuiteOptionsFunc func(*suiteOptions)

func withRootClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.rootClusterRoles = roles
	}
}

func withLeafClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.leafClusterRoles = roles
	}
}

func withRootAndLeafClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		withRootClusterRoles(roles...)(options)
		withLeafClusterRoles(roles...)(options)
	}
}

func withLeafClusterConfig(fn func(suite *Suite) *servicecfg.Config, configModFunctions ...func(config *servicecfg.Config)) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.leafConfigFunc = fn
		options.leafConfigModFunc = append(options.leafConfigModFunc, configModFunctions...)
	}
}

func withRootClusterConfig(fn func(suite *Suite) *servicecfg.Config, configModFunctions ...func(config *servicecfg.Config)) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.rootConfigFunc = fn
		options.rootConfigModFunc = append(options.rootConfigModFunc, configModFunctions...)
	}
}

func withRootAndLeafTrustedClusterReset() proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.rootTrustedSecretFunc = func(suite *Suite) []*helpers.InstanceSecrets {
			return nil
		}
		options.leafTrustedFunc = func(suite *Suite) []*helpers.InstanceSecrets {
			return nil
		}
	}
}

func withRootClusterNodeName(nodeName string) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.rootClusterNodeName = nodeName
	}
}

func withLeafClusterNodeName(nodeName string) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.leafClusterNodeName = nodeName
	}
}

func withRootClusterListeners(fn helpers.InstanceListenerSetupFunc) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.rootClusterListeners = fn
	}
}

func withLeafClusterListeners(fn helpers.InstanceListenerSetupFunc) proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.leafClusterListeners = fn
	}
}

func newRole(t *testing.T, roleName string, username string) types.Role {
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)
	return role
}

func rootClusterStandardConfig(t *testing.T) func(suite *Suite) *servicecfg.Config {
	return func(suite *Suite) *servicecfg.Config {
		rc := suite.root
		config := servicecfg.MakeDefaultConfig()
		config.DataDir = t.TempDir()
		config.Auth.Enabled = true
		config.Auth.Preference.SetSecondFactor("off")
		config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		config.Proxy.Enabled = true
		config.Proxy.WebAddr.Addr = rc.Web
		config.Proxy.DisableWebService = false
		config.Proxy.DisableWebInterface = true
		config.SSH.Enabled = true
		config.SSH.Addr.Addr = rc.SSH
		config.SSH.Labels = map[string]string{"env": "integration"}
		config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
		return config
	}
}

func leafClusterStandardConfig(t *testing.T) func(suite *Suite) *servicecfg.Config {
	return func(suite *Suite) *servicecfg.Config {
		lc := suite.leaf
		config := servicecfg.MakeDefaultConfig()
		config.DataDir = t.TempDir()
		config.Auth.Enabled = true
		config.Auth.Preference.SetSecondFactor("off")
		config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		config.Proxy.Enabled = true
		config.Proxy.WebAddr.Addr = lc.Web
		config.Proxy.DisableWebService = false
		config.Proxy.DisableWebInterface = true
		config.SSH.Enabled = true
		config.SSH.Addr.Addr = lc.SSH
		config.SSH.Labels = map[string]string{"env": "integration"}
		config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
		return config
	}
}

func createTestRole(username string) types.Role {
	role := services.NewImplicitRole()
	role.SetName("test")
	role.SetLogins(types.Allow, []string{username})
	role.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"env": []string{"{{external.testing}}"}})
	return role
}

func withStandardRoleMapping() proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.updateRoleMappingFunc = func(t *testing.T, suite *Suite) {
			ctx := context.Background()
			rc := suite.root
			lc := suite.leaf
			role := suite.root.Secrets.Users[helpers.MustGetCurrentUser(t).Username].Roles[0]
			ca, err := lc.Process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
				Type:       types.UserCA,
				DomainName: rc.Secrets.SiteName,
			}, false)
			require.NoError(t, err)
			ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
			ca.SetRoleMap(types.RoleMap{{Remote: role.GetName(), Local: []string{role.GetName()}}})
			err = lc.Process.GetAuthServer().UpsertCertAuthority(ctx, ca)
			require.NoError(t, err)
		}
	}
}

func withTrustedCluster() proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		options.updateRoleMappingFunc = func(t *testing.T, suite *Suite) {
			root := suite.root
			rootRole := suite.root.Secrets.Users[helpers.MustGetCurrentUser(t).Username].Roles[0]
			secondRole := suite.leaf.Secrets.Users[helpers.MustGetCurrentUser(t).Username].Roles[0]

			trustedClusterToken := "trustedclustertoken"
			err := root.Process.GetAuthServer().UpsertToken(context.Background(),
				types.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
			require.NoError(t, err)
			trustedCluster := root.AsTrustedCluster(trustedClusterToken, types.RoleMap{
				{Remote: rootRole.GetName(), Local: []string{secondRole.GetName()}},
			})
			err = trustedCluster.CheckAndSetDefaults()
			require.NoError(t, err)

			options.trustedCluster = trustedCluster
		}
	}
}

// withTrustedClusterBehindALB creates a local server that simulates a layer 7
// LB and puts it infront of the root cluster when the leaf connects through
// the reverse tunnel.
func withTrustedClusterBehindALB() proxySuiteOptionsFunc {
	return func(options *suiteOptions) {
		originalSetup := options.updateRoleMappingFunc

		options.updateRoleMappingFunc = func(t *testing.T, suite *Suite) {
			t.Helper()

			if originalSetup != nil {
				originalSetup(t, suite)
			}
			require.NotNil(t, options.trustedCluster)

			albProxy := helpers.MustStartMockALBProxy(t, suite.root.Config.Proxy.WebAddr.Addr)
			options.trustedCluster.SetProxyAddress(albProxy.Addr().String())
			options.trustedCluster.SetReverseTunnelAddress(albProxy.Addr().String())
		}
	}
}

func mustRunPostgresQuery(t *testing.T, client *pgconn.PgConn) {
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
}

func mustClosePostgresClient(t *testing.T, client *pgconn.PgConn) {
	err := client.Close(context.Background())
	require.NoError(t, err)
}

const (
	kubeClusterName             = "gke_project_europecentral2a_cluster1"
	kubeClusterDefaultNamespace = "default"
	kubePodName                 = "firstcontainer-66b6c48dd-bqmwk"
)

func k8ClientConfig(serverAddr, sni string) clientcmdapi.Config {
	return clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			kubeClusterName: {
				Server:                serverAddr,
				InsecureSkipTLSVerify: true,
				TLSServerName:         sni,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			kubeClusterName: {
				Cluster:  kubeClusterName,
				AuthInfo: kubeClusterName,
			},
		},
		CurrentContext: kubeClusterName,
	}
}

func mkPodList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodList",
			APIVersion: "v1",
		},
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubePodName,
					Namespace: kubeClusterDefaultNamespace,
				},
			},
		},
	}
}

func startKubeAPIMock(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/authorization.k8s.io/v1/selfsubjectaccessreviews", func(rw http.ResponseWriter, request *http.Request) {
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(rw http.ResponseWriter, request *http.Request) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(mkPodList())
		require.NoError(t, err)
	})

	svr := httptest.NewTLSServer(mux)
	t.Cleanup(func() {
		svr.Close()
	})
	return svr
}

func mustCreateKubeConfigFile(t *testing.T, config clientcmdapi.Config) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := clientcmd.WriteToFile(config, configPath)
	require.NoError(t, err)
	return configPath
}

func mustCreateKubeLocalProxyListener(t *testing.T, teleportCluster string, caCert, caKey []byte) net.Listener {
	t.Helper()

	ca, err := tls.X509KeyPair(caCert, caKey)
	require.NoError(t, err)

	listener, err := alpnproxy.NewKubeListener(map[string]tls.Certificate{
		teleportCluster: ca,
	})
	require.NoError(t, err)
	return listener
}

func mustStartALPNLocalProxy(t *testing.T, addr string, protocol alpncommon.Protocol) *alpnproxy.LocalProxy {
	t.Helper()

	return mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    addr,
		Protocols:          []alpncommon.Protocol{protocol},
		InsecureSkipVerify: true,
	})
}

func mustStartALPNLocalProxyWithConfig(t *testing.T, config alpnproxy.LocalProxyConfig) *alpnproxy.LocalProxy {
	t.Helper()

	if config.Listener == nil {
		config.Listener = helpers.MustCreateListener(t)
	}
	if config.ParentContext == nil {
		config.ParentContext = context.TODO()
	}

	lp, err := alpnproxy.NewLocalProxy(config)
	require.NoError(t, err)
	t.Cleanup(func() {
		lp.Close()
	})

	go func() {
		var err error
		if config.HTTPMiddleware == nil {
			err = lp.Start(context.Background())
		} else {
			err = lp.StartHTTPAccessProxy(context.Background())
		}
		assert.NoError(t, err)
	}()
	return lp
}

func mustStartKubeForwardProxy(t *testing.T, lpAddr string) *alpnproxy.ForwardProxy {
	t.Helper()

	fp, err := alpnproxy.NewKubeForwardProxy(alpnproxy.KubeForwardProxyConfig{
		ForwardAddr: lpAddr,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		fp.Close()
	})

	go func() {
		assert.NoError(t, fp.Start())
	}()
	return fp
}

func mustCreateKubeLocalProxyMiddleware(t *testing.T, teleportCluster, kubeCluster string, userCert, userKey []byte) alpnproxy.LocalProxyHTTPMiddleware {
	t.Helper()

	cert, err := tls.X509KeyPair(userCert, userKey)
	require.NoError(t, err)
	certs := make(alpnproxy.KubeClientCerts)
	certs.Add(teleportCluster, kubeCluster, cert)

	return alpnproxy.NewKubeMiddleware(certs, func(ctx context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error) {
		return tls.Certificate{}, nil
	}, clockwork.NewRealClock(), nil)
}

func makeNodeConfig(nodeName, proxyAddr string) *servicecfg.Config {
	nodeConfig := servicecfg.MakeDefaultConfig()
	nodeConfig.Version = defaults.TeleportConfigVersionV3
	nodeConfig.Hostname = nodeName
	nodeConfig.SetToken("token")
	nodeConfig.ProxyServer = *utils.MustParseAddr(proxyAddr)
	nodeConfig.Auth.Enabled = false
	nodeConfig.Proxy.Enabled = false
	nodeConfig.SSH.Enabled = true
	nodeConfig.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	return nodeConfig
}

// waitForActivePeerProxyConnections waits for remote cluster to report a minimum number of active proxy peer connections
func waitForActivePeerProxyConnections(t *testing.T, tunnel reversetunnelclient.Server, expectedCount int) { //nolint:unused // Only used by skipped test TestProxyTunnelStrategyProxyPeering
	require.Eventually(t, func() bool {
		return tunnel.GetProxyPeerClient().GetConnectionsCount() >= expectedCount
	},
		30*time.Second,
		time.Second,
		"Peer proxy connections did not reach %v in the expected time frame %v", expectedCount, 30*time.Second,
	)
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u
}

// fakeSTSClient is a fake HTTP client used to fake STS responses when Auth
// server sends out pre-signed STS requests for IAM join verification.
type fakeSTSClient struct {
	accountID   string
	arn         string
	credentials *credentials.Credentials
}

func (f fakeSTSClient) Do(req *http.Request) (*http.Response, error) {
	if err := awsutils.VerifyAWSSignature(req, f.credentials); err != nil {
		return nil, trace.Wrap(err)
	}
	response := fmt.Sprintf(`{"GetCallerIdentityResponse": {"GetCallerIdentityResult": {"Account": "%s", "Arn": "%s" }}}`, f.accountID, f.arn)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(response)),
	}, nil
}

func mustCreateIAMJoinProvisionToken(t *testing.T, name, awsAccountID, allowedARN string) types.ProvisionToken {
	t.Helper()

	provisionToken, err := types.NewProvisionTokenFromSpec(
		name,
		time.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{types.RoleNode},
			Allow: []*types.TokenRule{
				{
					AWSAccount: awsAccountID,
					AWSARN:     allowedARN,
				},
			},
			JoinMethod: types.JoinMethodIAM,
		},
	)
	require.NoError(t, err)
	return provisionToken
}

func mustRegisterUsingIAMMethod(t *testing.T, proxyAddr utils.NetAddr, token string, credentials *credentials.Credentials) {
	t.Helper()

	cred, err := credentials.Get()
	require.NoError(t, err)

	t.Setenv("AWS_ACCESS_KEY_ID", cred.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", cred.SecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", cred.SessionToken)
	t.Setenv("AWS_REGION", "us-west-2")

	privateKey, err := ssh.ParseRawPrivateKey([]byte(fixtures.SSHCAPrivateKey))
	require.NoError(t, err)
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	node := uuid.NewString()
	_, err = auth.Register(auth.RegisterParams{
		Token: token,
		ID: auth.IdentityID{
			Role:     types.RoleNode,
			HostUUID: node,
			NodeName: node,
		},
		ProxyServer:  proxyAddr,
		JoinMethod:   types.JoinMethodIAM,
		PublicTLSKey: pubTLS,
		PublicSSHKey: []byte(fixtures.SSHCAPublicKey),
	})
	require.NoError(t, err, trace.DebugReport(err))
}

func mustFindKubePod(t *testing.T, tc *client.TeleportClient) {
	t.Helper()

	serviceClient, err := tc.NewKubernetesServiceClient(context.Background(), tc.SiteName)
	require.NoError(t, err)

	response, err := serviceClient.ListKubernetesResources(context.Background(), &kubeproto.ListKubernetesResourcesRequest{
		ResourceType:        types.KindKubePod,
		KubernetesCluster:   kubeClusterName,
		KubernetesNamespace: kubeClusterDefaultNamespace,
		TeleportCluster:     tc.SiteName,
	})
	require.NoError(t, err)
	require.Len(t, response.Resources, 1)
	require.Equal(t, types.KindKubePod, response.Resources[0].Kind)
	require.Equal(t, kubePodName, response.Resources[0].GetName())
}
