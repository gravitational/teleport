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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
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
					config.Proxy.DisableALPNSNIListener = tc.disableALPNListenerOnLeaf
				}),
				withRootClusterPorts(tc.mainClusterPortSetup),
				withLeafClusterPorts(tc.secondClusterPortSetup),
				withRootAndLeafClusterRoles(createTestRole(username)),
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
	ph := &helpers.ProxyHandler{}
	ts := httptest.NewServer(ph)
	defer ts.Close()

	// set the http_proxy environment variable
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	t.Setenv("http_proxy", u.Host)

	username := mustGetCurrentUser(t).Username
	// httpproxy won't proxy when target address is localhost, so use this instead.
	addr, err := getLocalIP()
	require.NoError(t, err)

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterNodeName(addr),
		withLeafClusterNodeName(addr),
		withRootClusterPorts(singleProxyPortSetup()),
		withLeafClusterPorts(singleProxyPortSetup()),
		withRootAndLeafClusterRoles(createTestRole(username)),
		withStandardRoleMapping(),
	)

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, waitForClusters(suite.root.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")
	require.Eventually(t, waitForClusters(suite.leaf.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	require.Greater(t, ph.Count(), 0, "proxy did not intercept any connection")
}

// TestMultiPortHTTPSProxy tests if the reverse tunnel uses http_proxy
// on a multiple proxy port setup.
func TestMultiPortHTTPSProxy(t *testing.T) {
	// start the http proxy
	ph := &helpers.ProxyHandler{}
	ts := httptest.NewServer(ph)
	defer ts.Close()

	// set the http_proxy environment variable
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	t.Setenv("http_proxy", u.Host)

	username := mustGetCurrentUser(t).Username
	// httpproxy won't proxy when target address is localhost, so use this instead.
	addr, err := getLocalIP()
	require.NoError(t, err)

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterNodeName(addr),
		withLeafClusterNodeName(addr),
		withRootClusterPorts(standardPortSetup()),
		withLeafClusterPorts(standardPortSetup()),
		withRootAndLeafClusterRoles(createTestRole(username)),
		withStandardRoleMapping(),
	)

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, waitForClusters(suite.root.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")
	require.Eventually(t, waitForClusters(suite.leaf.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	require.Greater(t, ph.Count(), 0, "proxy did not intercept any connection")
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
	kubeRoleSpec := types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: []string{testImpersonationGroup},
			KubeUsers:  []string{k8User},
		},
	}
	kubeRole, err := types.NewRoleV3(k8RoleName, kubeRoleSpec)
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

// TestALPNSNIProxyKubeV2Leaf tests remove cluster kubernetes configuration where root and leaf proxies
// are using V2 configuration with Multiplex proxy listener.
func TestALPNSNIProxyKubeV2Leaf(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	const (
		localK8SNI = "kube.teleport.cluster.local"
		k8User     = "alice@example.com"
		k8RoleName = "kubemaster"
	)

	kubeAPIMockSvr := startKubeAPIMock(t)
	kubeConfigPath := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvr.URL, localK8SNI))

	username := mustGetCurrentUser(t).Username
	kubeRoleSpec := types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: []string{testImpersonationGroup},
			KubeUsers:  []string{k8User},
		},
	}
	kubeRole, err := types.NewRoleV3(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newProxySuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *service.Config) {
			config.Proxy.Kube.Enabled = true
			config.Version = defaults.TeleportConfigVersionV2
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t), func(config *service.Config) {
			config.Version = defaults.TeleportConfigVersionV2
			config.Proxy.Kube.Enabled = true

			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPath
			config.Kube.ListenAddr = utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
		}),
		withRootClusterRoles(kubeRole),
		withLeafClusterRoles(kubeRole),
		withRootAndLeafTrustedClusterReset(),
		withTrustedCluster(),
	)

	k8Client, _, err := kubeProxyClient(kubeProxyConfig{
		t:                   suite.root,
		username:            kubeRoleSpec.Allow.Logins[0],
		kubeUsers:           kubeRoleSpec.Allow.KubeGroups,
		kubeGroups:          kubeRoleSpec.Allow.KubeUsers,
		customTLSServerName: localK8SNI,
		targetAddress:       suite.root.Config.Proxy.WebAddr,
		routeToCluster:      suite.leaf.Secrets.SiteName,
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
		withLeafConfig(func(config *service.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		withRootConfig(func(config *service.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
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
		t.Run("connect to main cluster via proxy using ping protocol", func(t *testing.T) {
			pingProxy := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolWithPing(alpncommon.ProtocolMySQL))
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
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
		t.Run("connect to main cluster via proxy with ping protocol", func(t *testing.T) {
			pingProxy := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolWithPing(alpncommon.ProtocolPostgres))
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
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
		t.Run("connect to main cluster via proxy with ping protocol", func(t *testing.T) {
			pingProxy := mustStartALPNLocalProxy(t, pack.root.cluster.GetProxyAddr(), alpncommon.ProtocolWithPing(alpncommon.ProtocolMongoDB))
			client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
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
	})

	// Simulate situations where an AWS ALB is between client and the Teleport
	// Proxy service, which drops ALPN along the way. The ALPN local proxy will
	// need to make a connection upgrade first through a web API provided by
	// the Proxy server and then tunnel the original ALPN/TLS routing traffic
	// inside this tunnel.
	t.Run("ALPN connection upgrade", func(t *testing.T) {
		// Make a mock ALB which points to the Teleport Proxy Service. Then
		// ALPN local proxies will point to this ALB instead.
		albProxy := mustStartMockALBProxy(t, pack.root.cluster.GetProxyAddr())

		// Test a protocol in the alpncommon.IsDBTLSProtocol list where
		// the database client will perform a native TLS handshake.
		//
		// Packet layers:
		// - HTTPS served by Teleport web server for connection upgrade
		// - TLS routing with alpncommon.ProtocolMongoDB (no client cert)
		// - TLS with client cert (provided by the database client)
		// - MongoDB
		t.Run("database client native TLS", func(t *testing.T) {
			lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
				RemoteProxyAddr:         albProxy.Addr().String(),
				Protocols:               []alpncommon.Protocol{alpncommon.ProtocolMongoDB},
				ALPNConnUpgradeRequired: true,
				InsecureSkipVerify:      true,
			})
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
			require.NoError(t, client.Disconnect(context.Background()))
		})

		// Test the case where the database client cert is terminated within
		// the database protocol.
		//
		// Packet layers:
		// - HTTPS served by Teleport web server for connection upgrade
		// - TLS routing with alpncommon.ProtocolMySQL (no client cert)
		// - MySQL handshake then upgrade to TLS with Teleport issued client cert
		// - MySQL protocol
		t.Run("MySQL custom TLS", func(t *testing.T) {
			lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
				RemoteProxyAddr:         albProxy.Addr().String(),
				Protocols:               []alpncommon.Protocol{alpncommon.ProtocolMySQL},
				ALPNConnUpgradeRequired: true,
				InsecureSkipVerify:      true,
			})
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
			require.NoError(t, client.Close())
		})

		// Test the case where the client cert is terminated by Teleport and
		// the database client sends data in plain database protocol.
		//
		// Packet layers:
		// - HTTPS served by Teleport web server for connection upgrade
		// - TLS routing with alpncommon.ProtocolMySQL (client cert provided by ALPN local proxy)
		// - MySQL protocol
		t.Run("authenticated tunnel", func(t *testing.T) {
			routeToDatabase := tlsca.RouteToDatabase{
				ServiceName: pack.root.mysqlService.Name,
				Protocol:    pack.root.mysqlService.Protocol,
				Username:    "root",
			}
			clientTLSConfig, err := common.MakeTestClientTLSConfig(common.TestClientConfig{
				AuthClient:      pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer:      pack.root.cluster.Process.GetAuthServer(),
				Cluster:         pack.root.cluster.Secrets.SiteName,
				Username:        pack.root.user.GetName(),
				RouteToDatabase: routeToDatabase,
			})
			require.NoError(t, err)

			lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
				RemoteProxyAddr:         albProxy.Addr().String(),
				Protocols:               []alpncommon.Protocol{alpncommon.ProtocolMySQL},
				ALPNConnUpgradeRequired: true,
				InsecureSkipVerify:      true,
				Certs:                   clientTLSConfig.Certificates,
			})

			client, err := mysql.MakeTestClientWithoutTLS(lp.GetAddr(), routeToDatabase)
			require.NoError(t, err)

			// Execute a query.
			result, err := client.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			// Disconnect.
			require.NoError(t, client.Close())
		})
	})
}

// TestALPNSNIProxyAppAccess tests application access via ALPN SNI proxy service.
func TestALPNSNIProxyAppAccess(t *testing.T) {
	pack := setupWithOptions(t, appTestOptions{
		rootClusterPorts: singleProxyPortSetup(),
		leafClusterPorts: singleProxyPortSetup(),
		rootConfig: func(config *service.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		},
		leafConfig: func(config *service.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		},
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

	client, err := suite.root.NewClient(ClientConfig{
		Login:   username,
		Cluster: suite.root.Hostname,
	})
	require.NoError(t, err)

	ctx := context.Background()
	proxyClient, err := client.ConnectToProxy(context.Background())
	require.NoError(t, err)

	// Dial root auth service.
	rootAuthClient, err := proxyClient.ConnectToAuthServiceThroughALPNSNIProxy(ctx, "root.example.com", "")
	require.NoError(t, err)
	pr, err := rootAuthClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, "root.example.com", pr.ClusterName)
	err = rootAuthClient.Close()
	require.NoError(t, err)

	// Dial leaf auth service.
	leafAuthClient, err := proxyClient.ConnectToAuthServiceThroughALPNSNIProxy(ctx, "leaf.example.com", "")
	require.NoError(t, err)
	pr, err = leafAuthClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, "leaf.example.com", pr.ClusterName)
	err = leafAuthClient.Close()
	require.NoError(t, err)
}

// TestALPNProxyAuthClientConnectWithUserIdentity creates and connects to the Auth service
// using user identity file when teleport is configured with Multiple proxy listener mode.
func TestALPNProxyAuthClientConnectWithUserIdentity(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		Log:         utils.NewLoggerForTests(),
		Ports:       singleProxyPortSetup(),
	})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Version = "v2"
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	username := mustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	identityFilePath := MustCreateUserIdentityFile(t, rc, username, time.Hour)

	identity := client.LoadIdentityFile(identityFilePath)
	require.NoError(t, err)

	tc, err := client.New(context.Background(), client.Config{
		Addrs:                    []string{rc.GetWebAddr()},
		Credentials:              []client.Credentials{identity},
		InsecureAddressDiscovery: true,
	})
	require.NoError(t, err)

	resp, err := tc.Ping(context.Background())
	require.NoError(t, err)
	require.Equal(t, rc.Secrets.SiteName, resp.ClusterName)
}

// TestALPNProxyDialProxySSHWithoutInsecureMode tests dialing to the localhost with teleport-proxy-ssh
// protocol without using insecure mode in order to check if establishing connection to localhost works properly.
func TestALPNProxyDialProxySSHWithoutInsecureMode(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		Priv:        privateKey,
		Pub:         publicKey,
		Log:         utils.NewLoggerForTests(),
		Ports:       standardPortSetup(),
	})
	username := mustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		rc.StopAll()
	})

	// Disable insecure mode to make sure that dialing to localhost works.
	lib.SetInsecureDevMode(false)
	cfg := ClientConfig{
		Login:   username,
		Cluster: rc.Secrets.SiteName,
		Host:    "localhost",
	}

	ctx := context.Background()
	output := &bytes.Buffer{}
	cmd := []string{"echo", "hello world"}
	tc, err := rc.NewClient(cfg)
	require.NoError(t, err)
	tc.Stdout = output

	// Try to connect to the separate proxy SSH listener.
	tc.TLSRoutingEnabled = false
	err = tc.SSH(ctx, cmd, false)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())
	output.Reset()

	// Try to connect to the ALPN SNI Listener.
	tc.TLSRoutingEnabled = true
	err = tc.SSH(ctx, cmd, false)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())
}

// TestALPNProxyHTTPProxyNoProxyDial tests if a node joining to root cluster
// takes into account http_proxy and no_proxy env variables.
func TestALPNProxyHTTPProxyNoProxyDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	addr, err := getLocalIP()
	require.NoError(t, err)

	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    addr,
		Log:         utils.NewLoggerForTests(),
		Ports:       singleProxyPortSetup(),
	})
	username := mustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	// Create and start http_proxy server.
	ph := &helpers.ProxyHandler{}
	ts := httptest.NewServer(ph)
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	t.Setenv("http_proxy", u.Host)
	t.Setenv("no_proxy", addr)

	rcProxyAddr := net.JoinHostPort(addr, rc.GetPortWeb())

	// Start the node, due to no_proxy=127.0.0.1 env variable the connection established
	// to the proxy should not go through the http_proxy server.
	_, err = rc.StartNode(makeNodeConfig("first-root-node", rcProxyAddr))
	require.NoError(t, err)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
	defer cancel()

	err = waitForNodeCount(ctx, rc, "root.example.com", 1)
	require.NoError(t, err)

	require.Zero(t, ph.Count())

	// Unset the no_proxy=127.0.0.1 env variable. After that a new node
	// should take into account the http_proxy address and connection should go through the http_proxy.
	require.NoError(t, os.Unsetenv("no_proxy"))
	_, err = rc.StartNode(makeNodeConfig("second-root-node", rcProxyAddr))
	require.NoError(t, err)
	err = waitForNodeCount(ctx, rc, "root.example.com", 2)
	require.NoError(t, err)

	require.NotZero(t, ph.Count())
}

// TestALPNProxyHTTPProxyBasicAuthDial tests if a node joining to root cluster
// takes into account http_proxy with basic auth credentials in the address
func TestALPNProxyHTTPProxyBasicAuthDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	rcAddr, err := getLocalIP()
	require.NoError(t, err)

	log := utils.NewLoggerForTests()
	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    rcAddr,
		Log:         log,
		Ports:       singleProxyPortSetup(),
	})
	defer rc.StopAll()
	log.Info("Teleport root cluster instance created")

	username := mustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)

	// Create and start http_proxy server.
	ph := &helpers.ProxyHandler{}
	authorizer := helpers.NewProxyAuthorizer(ph, "alice", "rosebud")
	ts := httptest.NewServer(authorizer)
	defer ts.Close()

	proxyURL, err := url.Parse(ts.URL)
	require.NoError(t, err)

	// set http_proxy to user:password@host
	// these credentials will be rejected by the auth proxy (initially).
	user := "aladdin"
	pass := "open sesame"
	t.Setenv("http_proxy", helpers.MakeProxyAddr(user, pass, proxyURL.Host))

	rcProxyAddr := net.JoinHostPort(rcAddr, rc.GetPortWeb())
	nodeCfg := makeNodeConfig("node1", rcProxyAddr)
	nodeCfg.Log = log

	timeout := time.Second * 60
	startErrC := make(chan error)
	// start the node but don't block waiting for it while it attempts to connect to the auth server.
	go func() {
		_, err := rc.StartNode(nodeCfg)
		startErrC <- err
	}()
	require.ErrorIs(t, authorizer.WaitForRequest(timeout), trace.AccessDenied("bad credentials"))
	require.Zero(t, ph.Count())

	// set the auth credentials to match our environment
	authorizer.SetCredentials(user, pass)

	// with env set correctly and authorized, the node should register.
	require.NoError(t, <-startErrC)
	require.NoError(t, waitForNodeCount(context.Background(), rc, rc.Secrets.SiteName, 1))
	require.Greater(t, ph.Count(), 0)
}
