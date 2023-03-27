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
	"crypto/x509/pkix"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
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

func mustRunPostgresQuery(t *testing.T, client *pgconn.PgConn) {
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
}

func mustClosePostgresClient(t *testing.T, client *pgconn.PgConn) {
	err := client.Close(context.Background())
	require.NoError(t, err)
}

func k8ClientConfig(serverAddr, sni string) clientcmdapi.Config {
	const clusterName = "gke_project_europecentral2a_cluster1"
	return clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                serverAddr,
				InsecureSkipTLSVerify: true,
				TLSServerName:         sni,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: clusterName,
			},
		},
		CurrentContext: clusterName,
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
					Name: "firstcontainer-66b6c48dd-bqmwk",
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
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := clientcmd.WriteToFile(config, configPath)
	require.NoError(t, err)
	return configPath
}

func mustCreateListener(t *testing.T) net.Listener {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	t.Cleanup(func() {
		listener.Close()
	})
	return listener
}

func mustStartALPNLocalProxy(t *testing.T, addr string, protocol alpncommon.Protocol) *alpnproxy.LocalProxy {
	return mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    addr,
		Protocols:          []alpncommon.Protocol{protocol},
		InsecureSkipVerify: true,
	})
}

func mustStartALPNLocalProxyWithConfig(t *testing.T, config alpnproxy.LocalProxyConfig) *alpnproxy.LocalProxy {
	if config.Listener == nil {
		config.Listener = mustCreateListener(t)
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
		err := lp.Start(context.Background())
		require.NoError(t, err)
	}()
	return lp
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

func mustCreateSelfSignedCert(t *testing.T) tls.Certificate {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	cert, err := tls.X509KeyPair(caCert, caKey)
	require.NoError(t, err)
	return cert
}

// mockAWSALBProxy is a mock proxy server that simulates an AWS application
// load balancer where ALPN is not supported. Note that this mock does not
// actually balance traffic.
type mockAWSALBProxy struct {
	net.Listener
	proxyAddr string
	cert      tls.Certificate
}

func (m *mockAWSALBProxy) serve(ctx context.Context, t *testing.T) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := m.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return
			}
			require.NoError(t, err)
			return
		}

		go func() {
			// Handshake with incoming client and drops ALPN.
			downstreamConn := tls.Server(conn, &tls.Config{
				Certificates: []tls.Certificate{m.cert},
			})
			require.NoError(t, downstreamConn.HandshakeContext(ctx))

			// Make a connection to the proxy server with ALPN protos.
			upstreamConn, err := tls.Dial("tcp", m.proxyAddr, &tls.Config{
				InsecureSkipVerify: true,
			})
			require.NoError(t, err)

			utils.ProxyConn(ctx, downstreamConn, upstreamConn)
		}()
	}
}

func mustStartMockALBProxy(t *testing.T, proxyAddr string) *mockAWSALBProxy {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := &mockAWSALBProxy{
		proxyAddr: proxyAddr,
		Listener:  mustCreateListener(t),
		cert:      mustCreateSelfSignedCert(t),
	}
	go m.serve(ctx, t)
	return m
}

// waitForActivePeerProxyConnections waits for remote cluster to report a minimum number of active proxy peer connections
func waitForActivePeerProxyConnections(t *testing.T, tunnel reversetunnel.Server, expectedCount int) { //nolint:unused // Only used by skipped test TestProxyTunnelStrategyProxyPeering
	require.Eventually(t, func() bool {
		return tunnel.GetProxyPeerClient().GetConnectionsCount() >= expectedCount
	},
		30*time.Second,
		time.Second,
		"Peer proxy connections did not reach %v in the expected time frame %v", expectedCount, 30*time.Second,
	)
}
