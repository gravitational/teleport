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

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testlog"

	"github.com/jackc/pgconn"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

type ProxySuite struct {
	main *TeleInstance
	leaf *TeleInstance
}

type proxySuiteOptions struct {
	mainConfigFunc   func(suite *ProxySuite) *service.Config
	secondConfigFunc func(suite *ProxySuite) *service.Config

	mainConfigModFunc   []func(config *service.Config)
	secondConfigModFunc []func(config *service.Config)

	mainClusterPorts *InstancePorts
	leafClusterPorts *InstancePorts

	mainTrustedSecretFunc func(suite *ProxySuite) []*InstanceSecrets
	secondTrustedFunc     func(suite *ProxySuite) []*InstanceSecrets

	mainClusterRoles      []types.Role
	secondClusterRoles    []types.Role
	updateRoleMappingFunc func(t *testing.T, suite *ProxySuite)

	trustedCluster types.TrustedCluster
}

func newProxySuite(t *testing.T, opts ...proxySuiteOptionsFunc) *ProxySuite {
	options := proxySuiteOptions{
		mainClusterPorts: singleProxyPortSetup(),
		leafClusterPorts: singleProxyPortSetup(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		log:         testlog.FailureOnly(t),
		Ports:       options.mainClusterPorts,
	})

	// Create leaf cluster.
	lc := NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		Priv:        rc.Secrets.PrivKey,
		Pub:         rc.Secrets.PubKey,
		log:         testlog.FailureOnly(t),
		Ports:       options.leafClusterPorts,
	})
	suite := &ProxySuite{
		main: rc,
		leaf: lc,
	}

	user := mustGetCurrentUser(t)
	for _, role := range options.mainClusterRoles {
		rc.AddUserWithRole(user.Username, role)
	}
	for _, role := range options.secondClusterRoles {
		lc.AddUserWithRole(user.Username, role)
	}

	mainTrustedSecrets := lc.Secrets.AsSlice()
	if options.mainTrustedSecretFunc != nil {
		mainTrustedSecrets = options.mainTrustedSecretFunc(suite)
	}

	mainConfig := options.mainConfigFunc(suite)
	for _, v := range options.mainConfigModFunc {
		v(mainConfig)
	}
	err := rc.CreateEx(t, mainTrustedSecrets, mainConfig)
	require.NoError(t, err)

	secondTrustedSecrets := rc.Secrets.AsSlice()
	if options.secondTrustedFunc != nil {
		secondTrustedSecrets = options.secondTrustedFunc(suite)
	}
	secondConfig := options.secondConfigFunc(suite)
	for _, v := range options.secondConfigModFunc {
		v(mainConfig)
	}
	err = lc.CreateEx(t, secondTrustedSecrets, secondConfig)
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
		tryCreateTrustedCluster(t, suite.leaf.Process.GetAuthServer(), options.trustedCluster)
		waitForTunnelConnections(t, suite.main.Process.GetAuthServer(), suite.leaf.Secrets.SiteName, 1)
	}

	return suite
}

func (p *ProxySuite) addNodeToLeafCluster(t *testing.T, tunnelNodeHostname string) {
	const (
		deadline         = time.Second * 10
		nextIterWaitTime = time.Second * 2
	)

	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Console = nil
		tconf.Log = testlog.FailureOnly(t)
		tconf.Hostname = tunnelNodeHostname
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			{
				AddrNetwork: "tcp",
				Addr:        net.JoinHostPort(Loopback, p.leaf.GetPortWeb()),
			},
		}
		tconf.Auth.Enabled = false
		tconf.Proxy.Enabled = false
		tconf.SSH.Enabled = true
		return tconf
	}
	_, err := p.leaf.StartNode(nodeConfig())
	require.NoError(t, err)

	err = utils.RetryStaticFor(deadline, nextIterWaitTime, func() error {
		if len(checkGetClusters(t, p.main.Tunnel)) < 2 && len(checkGetClusters(t, p.leaf.Tunnel)) < 2 {
			return trace.NotFound("two clusters do not see each other: tunnels are not working")
		}
		return nil
	})
	require.NoError(t, err)

	// Wait for both nodes to show up before attempting to dial to them.
	err = waitForNodeCount(context.Background(), p.main, p.leaf.Secrets.SiteName, 2)
	require.NoError(t, err)
}

func (p *ProxySuite) mustConnectToClusterAndRunSSHCommand(t *testing.T, config ClientConfig) {
	const (
		deadline         = time.Second
		nextIterWaitTime = time.Millisecond * 100
	)

	tc, err := p.main.NewClient(t, config)
	require.NoError(t, err)

	output := &bytes.Buffer{}
	tc.Stdout = output
	require.NoError(t, err)

	cmd := []string{"echo", "hello world"}
	err = utils.RetryStaticFor(deadline, nextIterWaitTime, func() error {
		err = tc.SSH(context.TODO(), cmd, false)
		return trace.Wrap(err)
	})
	require.NoError(t, err)

	require.Equal(t, "hello world\n", output.String())
}

type proxySuiteOptionsFunc func(*proxySuiteOptions)

func withMainClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.mainClusterRoles = roles
	}
}

func withLeafClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.secondClusterRoles = roles
	}
}

func withMainAndLeafClusterRoles(roles ...types.Role) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		withMainClusterRoles(roles...)(options)
		withLeafClusterRoles(roles...)(options)

	}
}

func withLeafClusterConfig(fn func(suite *ProxySuite) *service.Config, configModFunctions ...func(config *service.Config)) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.secondConfigFunc = fn
		options.secondConfigModFunc = append(options.secondConfigModFunc, configModFunctions...)
	}
}

func withMainClusterConfig(fn func(suite *ProxySuite) *service.Config, configModFunctions ...func(config *service.Config)) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.mainConfigFunc = fn
		options.mainConfigModFunc = append(options.mainConfigModFunc, configModFunctions...)
	}
}

func withMainAndLeafTrustedClusterReset() proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.mainTrustedSecretFunc = func(suite *ProxySuite) []*InstanceSecrets {
			return nil
		}
		options.secondTrustedFunc = func(suite *ProxySuite) []*InstanceSecrets {
			return nil
		}
	}
}

func withMainClusterPorts(ports *InstancePorts) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.mainClusterPorts = ports
	}
}

func withLeafClusterPorts(ports *InstancePorts) proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.leafClusterPorts = ports
	}
}

func newRole(t *testing.T, roleName string, username string) types.Role {
	role, err := types.NewRole(roleName, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	return role
}

func mainClusterStandardConfig(t *testing.T) func(suite *ProxySuite) *service.Config {
	return func(suite *ProxySuite) *service.Config {
		rc := suite.main
		config := service.MakeDefaultConfig()
		config.DataDir = t.TempDir()
		config.Auth.Enabled = true
		config.Auth.Preference.SetSecondFactor("off")
		config.Proxy.Enabled = true
		config.Proxy.WebAddr.Addr = net.JoinHostPort(rc.Hostname, rc.GetPortWeb())
		config.Proxy.DisableWebService = false
		config.Proxy.DisableWebInterface = true
		config.SSH.Enabled = true
		config.SSH.Addr.Addr = net.JoinHostPort(rc.Hostname, rc.GetPortSSH())
		config.SSH.Labels = map[string]string{"env": "integration"}
		return config
	}
}

func leafClusterStandardConfig(t *testing.T) func(suite *ProxySuite) *service.Config {
	return func(suite *ProxySuite) *service.Config {
		lc := suite.leaf
		config := service.MakeDefaultConfig()
		config.DataDir = t.TempDir()
		config.Auth.Enabled = true
		config.Auth.Preference.SetSecondFactor("off")
		config.Proxy.Enabled = true
		config.Proxy.WebAddr.Addr = net.JoinHostPort(lc.Hostname, lc.GetPortWeb())
		config.Proxy.DisableWebService = false
		config.Proxy.DisableWebInterface = true
		config.SSH.Enabled = true
		config.SSH.Addr.Addr = net.JoinHostPort(lc.Hostname, lc.GetPortSSH())
		config.SSH.Labels = map[string]string{"env": "integration"}
		return config
	}
}

func createAdminRole(username string) types.Role {
	role := services.NewAdminRole()
	role.SetName("test")
	role.SetLogins(types.Allow, []string{username})
	role.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"env": []string{"{{external.testing}}"}})
	return role
}

func withStandardRoleMapping() proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.updateRoleMappingFunc = func(t *testing.T, suite *ProxySuite) {
			rc := suite.main
			lc := suite.leaf
			role := suite.main.Secrets.Users[mustGetCurrentUser(t).Username].Roles[0]
			ca, err := lc.Process.GetAuthServer().GetCertAuthority(types.CertAuthID{
				Type:       types.UserCA,
				DomainName: rc.Secrets.SiteName,
			}, false)
			require.NoError(t, err)
			ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
			ca.SetRoleMap(types.RoleMap{{Remote: role.GetName(), Local: []string{role.GetName()}}})
			err = lc.Process.GetAuthServer().UpsertCertAuthority(ca)
			require.NoError(t, err)

		}
	}
}

func withTrustedCluster() proxySuiteOptionsFunc {
	return func(options *proxySuiteOptions) {
		options.updateRoleMappingFunc = func(t *testing.T, suite *ProxySuite) {
			main := suite.main
			mainRole := suite.main.Secrets.Users[mustGetCurrentUser(t).Username].Roles[0]
			secondRole := suite.leaf.Secrets.Users[mustGetCurrentUser(t).Username].Roles[0]

			trustedClusterToken := "trustedclustertoken"
			err := main.Process.GetAuthServer().UpsertToken(context.Background(),
				types.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
			require.NoError(t, err)
			trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
				{Remote: mainRole.GetName(), Local: []string{secondRole.GetName()}},
			})
			err = trustedCluster.CheckAndSetDefaults()
			require.NoError(t, err)

			options.trustedCluster = trustedCluster

		}
	}
}

func mustGetCurrentUser(t *testing.T) *user.User {
	user, err := user.Current()
	require.NoError(t, err)
	return user
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

func mustStartALPNLocalProxy(t *testing.T, addr string, protocol alpnproxy.Protocol) *alpnproxy.LocalProxy {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	address, err := utils.ParseAddr(addr)
	require.NoError(t, err)
	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    addr,
		Protocol:           protocol,
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      context.Background(),
		SNI:                address.Host(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		lp.Close()
	})

	startSyncC := make(chan struct{})
	go func() {
		close(startSyncC)
		err := lp.Start(context.Background())
		require.NoError(t, err)
	}()
	<-startSyncC
	return lp
}
