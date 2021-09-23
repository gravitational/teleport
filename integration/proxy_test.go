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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestALPNSNIProxyMultiCluster tests SSH connection in multi-cluster setup with.
func TestALPNSNIProxyMultiCluster(t *testing.T) {
	testCase := []struct {
		name                      string
		mainClusterPortSetup      *InstancePorts
		secondClusterPortSetup    *InstancePorts
		disableALPNListenerOnRoot bool
		disableALPNListenerOnLeaf bool
	}{
		{
			name:                      "StandardAndOnePortSetupMasterALPNDisabled",
			mainClusterPortSetup:      standardPortSetup(),
			secondClusterPortSetup:    singleProxyPortSetup(),
			disableALPNListenerOnRoot: true,
		},
		{
			name:                   "StandardAndOnePortSetup",
			mainClusterPortSetup:   standardPortSetup(),
			secondClusterPortSetup: singleProxyPortSetup(),
		},
		{
			name:                   "TwoClusterOnePortSetup",
			mainClusterPortSetup:   singleProxyPortSetup(),
			secondClusterPortSetup: singleProxyPortSetup(),
		},
		{
			name:                      "OnePortAndStandardPortSetupLeafALPNDisabled",
			mainClusterPortSetup:      singleProxyPortSetup(),
			secondClusterPortSetup:    standardPortSetup(),
			disableALPNListenerOnLeaf: true,
		},
		{
			name:                   "OnePortAndStandardPortSetup",
			mainClusterPortSetup:   singleProxyPortSetup(),
			secondClusterPortSetup: standardPortSetup(),
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			lib.SetInsecureDevMode(true)
			defer lib.SetInsecureDevMode(false)

			username := mustGetCurrentUser(t).Username

			suite := newProxySuite(t,
				withRootClusterConfig(rootClusterStandardConfig(t), func(config *service.Config) {
					config.Proxy.DisableALPNSNIListener = tc.disableALPNListenerOnRoot
				}),
				withLeafClusterConfig(leafClusterStandardConfig(t), func(config *service.Config) {
					config.Proxy.DisableALPNSNIListener = tc.disableALPNListenerOnRoot
				}),
				withRootClusterPorts(tc.mainClusterPortSetup),
				withLeafClusterPorts(tc.secondClusterPortSetup),
				withRootAndLeafClusterRoles(createAdminRole(username)),
				withStandardRoleMapping(),
			)
			// Run command in root.
			suite.mustConnectToClusterAndRunSSHCommand(t, ClientConfig{
				Login:   username,
				Cluster: suite.root.Secrets.SiteName,
				Host:    Loopback,
				Port:    suite.root.GetPortSSHInt(),
			})
			// Run command in leaf.
			suite.mustConnectToClusterAndRunSSHCommand(t, ClientConfig{
				Login:   username,
				Cluster: suite.leaf.Secrets.SiteName,
				Host:    Loopback,
				Port:    suite.leaf.GetPortSSHInt(),
			})
		})
	}
}

// TestALPNSNIProxyTrustedClusterNode tests ssh connection to a trusted cluster node.
func TestALPNSNIProxyTrustedClusterNode(t *testing.T) {
	testCase := []struct {
		name                      string
		mainClusterPortSetup      *InstancePorts
		secondClusterPortSetup    *InstancePorts
		disableALPNListenerOnRoot bool
		disableALPNListenerOnLeaf bool
	}{
		{
			name:                      "StandardAndOnePortSetupMasterALPNDisabled",
			mainClusterPortSetup:      standardPortSetup(),
			secondClusterPortSetup:    singleProxyPortSetup(),
			disableALPNListenerOnRoot: true,
		},
		{
			name:                   "StandardAndOnePortSetup",
			mainClusterPortSetup:   standardPortSetup(),
			secondClusterPortSetup: singleProxyPortSetup(),
		},
		{
			name:                   "TwoClusterOnePortSetup",
			mainClusterPortSetup:   singleProxyPortSetup(),
			secondClusterPortSetup: singleProxyPortSetup(),
		},
		{
			name:                      "OnePortAndStandardPortSetupLeafALPNDisabled",
			mainClusterPortSetup:      singleProxyPortSetup(),
			secondClusterPortSetup:    standardPortSetup(),
			disableALPNListenerOnLeaf: true,
		},
		{
			name:                   "OnePortAndStandardPortSetup",
			mainClusterPortSetup:   singleProxyPortSetup(),
			secondClusterPortSetup: standardPortSetup(),
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			lib.SetInsecureDevMode(true)
			defer lib.SetInsecureDevMode(false)

			username := mustGetCurrentUser(t).Username

			suite := newProxySuite(t,
				withRootClusterConfig(rootClusterStandardConfig(t)),
				withLeafClusterConfig(leafClusterStandardConfig(t)),
				withRootClusterPorts(tc.mainClusterPortSetup),
				withLeafClusterPorts(tc.secondClusterPortSetup),
				withRootClusterRoles(newRole(t, "maindevs", username)),
				withLeafClusterRoles(newRole(t, "auxdevs", username)),
				withRootAndLeafTrustedClusterReset(),
				withTrustedCluster(),
			)

			nodeHostname := "clusterauxnode"
			suite.addNodeToLeafCluster(t, "clusterauxnode")

			// Try and connect to a node in the Aux cluster from the Root cluster using
			// direct dialing.
			suite.mustConnectToClusterAndRunSSHCommand(t, ClientConfig{
				Login:   username,
				Cluster: suite.leaf.Secrets.SiteName,
				Host:    Loopback,
				Port:    suite.leaf.GetPortSSHInt(),
			})

			// Try and connect to a node in the Aux cluster from the Root cluster using
			// tunnel dialing.
			suite.mustConnectToClusterAndRunSSHCommand(t, ClientConfig{
				Login:   username,
				Cluster: suite.leaf.Secrets.SiteName,
				Host:    nodeHostname,
			})
		})
	}
}

// TestALPNSNIProxyMultiCluster tests if the reverse tunnel uses http_proxy
// on a single proxy port setup.
func TestALPNSNIHTTPSProxy(t *testing.T) {
	// start the http proxy
	ps := &proxyServer{}
	ts := httptest.NewServer(ps)
	defer ts.Close()

	// set the http_proxy environment variable
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	os.Setenv("http_proxy", u.Host)
	defer os.Setenv("http_proxy", "")

	username := mustGetCurrentUser(t).Username

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterPorts(singleProxyPortSetup()),
		withLeafClusterPorts(singleProxyPortSetup()),
		withRootAndLeafClusterRoles(createAdminRole(username)),
		withStandardRoleMapping(),
	)
	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	utils.RetryStaticFor(time.Second*10, time.Millisecond*200, func() error {
		for len(checkGetClusters(t, suite.root.Tunnel)) < 2 && len(checkGetClusters(t, suite.leaf.Tunnel)) < 2 {
			return errors.New("two sites do not see each other: tunnels are not working")
		}
		return nil
	})
	require.Greater(t, ps.Count(), 0, "proxy did not intercept any connection")
}

// TestAlpnSniProxyKube tests Kubernetes access with custom Kube API mock where traffic is forwarded via
//SNI ALPN proxy service to Kubernetes service based on TLS SNI value.
func TestALPNSNIProxyKube(t *testing.T) {
	const (
		localK8SNI = "kube.teleport.cluster.local"
		k8User     = "alice@example.com"
		k8RoleName = "kubemaster"
	)

	kubeAPIMockSvr := startKubeAPIMock(t)
	kubeConfigPath := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvr.URL, localK8SNI))

	username := mustGetCurrentUser(t).Username
	kubeRoleSpec := types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: []string{testImpersonationGroup},
			KubeUsers:  []string{k8User},
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *service.Config) {
			config.Proxy.Kube.Enabled = true
			config.Proxy.Kube.KubeconfigPath = kubeConfigPath
			config.Proxy.Kube.LegacyKubeProxy = true
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootAndLeafClusterRoles(kubeRole),
		withStandardRoleMapping(),
	)

	k8Client, _, err := kubeProxyClient(kubeProxyConfig{
		t:                   suite.root,
		username:            kubeRoleSpec.Allow.Logins[0],
		kubeUsers:           kubeRoleSpec.Allow.KubeGroups,
		kubeGroups:          kubeRoleSpec.Allow.KubeUsers,
		customTLSServerName: localK8SNI,
		targetAddress:       suite.root.Config.Proxy.WebAddr,
	})
	require.NoError(t, err)

	resp, err := k8Client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Items), "pods item length mismatch")
}

// TestALPNSNIProxyDatabaseAccess test DB connection forwarded through local SNI ALPN proxy where
// DB protocol is wrapped into TLS and forwarded to proxy ALPN SNI service and routed to appropriate db service.
func TestALPNSNIProxyDatabaseAccess(t *testing.T) {
	pack := setupDatabaseTest(t,
		withPortSetupDatabaseTest(singleProxyPortSetup),
	)
	pack.waitForLeaf(t)

	t.Run("mysql", func(t *testing.T) {
		lp := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolMySQL)
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.root.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.root.mysqlService.Name,
					Protocol:    pack.root.mysqlService.Protocol,
					Username:    "root",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			result, err := client.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			// Disconnect.
			err = client.Close()
			require.NoError(t, err)

		})
		t.Run("connect to leaf cluster via proxy", func(t *testing.T) {
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.leaf.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.leaf.mysqlService.Name,
					Protocol:    pack.leaf.mysqlService.Protocol,
					Username:    "root",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			result, err := client.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			// Disconnect.
			err = client.Close()
			require.NoError(t, err)
		})
	})

	t.Run("postgres", func(t *testing.T) {
		lp := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolPostgres)
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.root.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.root.postgresService.Name,
					Protocol:    pack.root.postgresService.Protocol,
					Username:    "postgres",
					Database:    "test",
				},
			})
			require.NoError(t, err)
			mustRunPostgresQuery(t, client)
			mustClosePostgresClient(t, client)
		})
		t.Run("connect to leaf cluster via proxy", func(t *testing.T) {
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.leaf.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.leaf.postgresService.Name,
					Protocol:    pack.leaf.postgresService.Protocol,
					Username:    "postgres",
					Database:    "test",
				},
			})
			require.NoError(t, err)
			mustRunPostgresQuery(t, client)
			mustClosePostgresClient(t, client)
		})
	})

	t.Run("mongo", func(t *testing.T) {
		lp := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolMongoDB)
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.root.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.root.mongoService.Name,
					Protocol:    pack.root.mongoService.Protocol,
					Username:    "admin",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
			require.NoError(t, err)

			// Disconnect.
			err = client.Disconnect(context.Background())
			require.NoError(t, err)
		})
		t.Run("connect to leaf cluster via proxy", func(t *testing.T) {
			client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.leaf.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.leaf.mongoService.Name,
					Protocol:    pack.leaf.mongoService.Protocol,
					Username:    "admin",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
			require.NoError(t, err)

			// Disconnect.
			err = client.Disconnect(context.Background())
			require.NoError(t, err)
		})
	})
}

// TestALPNSNIProxyAppAccess tests application access via ALPN SNI proxy service.
func TestALPNSNIProxyAppAccess(t *testing.T) {
	pack := setupWithOptions(t, appTestOptions{
		rootClusterPorts: singleProxyPortSetup(),
		leafClusterPorts: singleProxyPortSetup(),
	})

	sess := pack.createAppSession(t, pack.rootAppPublicAddr, pack.rootAppClusterName)
	status, _, err := pack.makeRequest(sess, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	sess = pack.createAppSession(t, pack.leafAppPublicAddr, pack.leafAppClusterName)
	status, _, err = pack.makeRequest(sess, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
}

// TestALPNProxyRootLeafAuthDial tests dialing local/remote auth service based on ALPN
// teleport-auth protocol and ServerName as encoded cluster name.
func TestALPNProxyRootLeafAuthDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	username := mustGetCurrentUser(t).Username

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterPorts(singleProxyPortSetup()),
		withLeafClusterPorts(singleProxyPortSetup()),
		withRootClusterRoles(newRole(t, "rootdevs", username)),
		withLeafClusterRoles(newRole(t, "leafdevs", username)),
		withRootAndLeafTrustedClusterReset(),
		withTrustedCluster(),
	)

	client, err := suite.root.NewClient(t, ClientConfig{
		Login:   username,
		Cluster: suite.root.Hostname,
	})
	require.NoError(t, err)

	ctx := context.Background()
	proxyClient, err := client.ConnectToProxy(context.Background())
	require.NoError(t, err)

	// Dial root auth service.
	rootAuthClient, err := proxyClient.ConnectToAuthServiceThroughALPNSNIProxy(ctx, "root.example.com")
	require.NoError(t, err)
	pr, err := rootAuthClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, "root.example.com", pr.ClusterName)
	err = rootAuthClient.Close()
	require.NoError(t, err)

	// Dial leaf auth service.
	leafAuthClient, err := proxyClient.ConnectToAuthServiceThroughALPNSNIProxy(ctx, "leaf.example.com")
	require.NoError(t, err)
	pr, err = leafAuthClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, "leaf.example.com", pr.ClusterName)
	err = leafAuthClient.Close()
	require.NoError(t, err)
}
