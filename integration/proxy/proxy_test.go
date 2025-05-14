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
	"bytes"
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	apihelpers "github.com/gravitational/teleport/api/testhelpers"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/appaccess"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
		mainClusterPortSetup      helpers.InstanceListenerSetupFunc
		secondClusterPortSetup    helpers.InstanceListenerSetupFunc
		disableALPNListenerOnRoot bool
		disableALPNListenerOnLeaf bool
	}{
		{
			name:                      "StandardAndOnePortSetupMasterALPNDisabled",
			mainClusterPortSetup:      helpers.StandardListenerSetup,
			secondClusterPortSetup:    helpers.SingleProxyPortSetup,
			disableALPNListenerOnRoot: true,
		},
		{
			name:                   "StandardAndOnePortSetup",
			mainClusterPortSetup:   helpers.StandardListenerSetup,
			secondClusterPortSetup: helpers.SingleProxyPortSetup,
		},
		{
			name:                   "TwoClusterOnePortSetup",
			mainClusterPortSetup:   helpers.SingleProxyPortSetup,
			secondClusterPortSetup: helpers.SingleProxyPortSetup,
		},
		{
			name:                      "OnePortAndStandardListenerSetupLeafALPNDisabled",
			mainClusterPortSetup:      helpers.SingleProxyPortSetup,
			secondClusterPortSetup:    helpers.StandardListenerSetup,
			disableALPNListenerOnLeaf: true,
		},
		{
			name:                   "OnePortAndStandardListenerSetup",
			mainClusterPortSetup:   helpers.SingleProxyPortSetup,
			secondClusterPortSetup: helpers.StandardListenerSetup,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			lib.SetInsecureDevMode(true)
			defer lib.SetInsecureDevMode(false)

			username := helpers.MustGetCurrentUser(t).Username

			suite := newSuite(t,
				withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
					config.Proxy.DisableALPNSNIListener = tc.disableALPNListenerOnRoot
				}),
				withLeafClusterConfig(leafClusterStandardConfig(t), func(config *servicecfg.Config) {
					config.Proxy.DisableALPNSNIListener = tc.disableALPNListenerOnLeaf
				}),
				withRootClusterListeners(tc.mainClusterPortSetup),
				withLeafClusterListeners(tc.secondClusterPortSetup),
				withRootAndLeafClusterRoles(createTestRole(username)),
				withStandardRoleMapping(),
			)
			// Run command in root.
			suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
				Login:   username,
				Cluster: suite.root.Secrets.SiteName,
				Host:    helpers.Loopback,
				Port:    helpers.Port(t, suite.root.SSH),
			})
			// Run command in leaf.
			suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
				Login:   username,
				Cluster: suite.leaf.Secrets.SiteName,
				Host:    helpers.Loopback,
				Port:    helpers.Port(t, suite.leaf.SSH),
			})

			t.Run("WebProxyAddr behind ALB", func(t *testing.T) {
				// Make a mock ALB which points to the Teleport Proxy Service.
				albProxy := helpers.MustStartMockALBProxy(t, suite.root.Config.Proxy.WebAddr.Addr)

				// Run command in root through ALB address.
				suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
					Login:   username,
					Cluster: suite.root.Secrets.SiteName,
					Host:    helpers.Loopback,
					Port:    helpers.Port(t, suite.root.SSH),
					ALBAddr: albProxy.Addr().String(),
				})

				// Run command in leaf through ALB address.
				suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
					Login:   username,
					Cluster: suite.leaf.Secrets.SiteName,
					Host:    helpers.Loopback,
					Port:    helpers.Port(t, suite.leaf.SSH),
					ALBAddr: albProxy.Addr().String(),
				})
			})
		})
	}
}

// TestALPNSNIProxyTrustedClusterNode tests ssh connection to a trusted cluster node.
func TestALPNSNIProxyTrustedClusterNode(t *testing.T) {
	testCase := []struct {
		name                       string
		mainClusterListenerSetup   helpers.InstanceListenerSetupFunc
		secondClusterListenerSetup helpers.InstanceListenerSetupFunc
		disableALPNListenerOnRoot  bool
		disableALPNListenerOnLeaf  bool
		extraSuiteOptions          []proxySuiteOptionsFunc
	}{
		{
			name:                       "StandardAndOnePortSetupMasterALPNDisabled",
			mainClusterListenerSetup:   helpers.StandardListenerSetup,
			secondClusterListenerSetup: helpers.SingleProxyPortSetup,
			disableALPNListenerOnRoot:  true,
		},
		{
			name:                       "StandardAndOnePortSetup",
			mainClusterListenerSetup:   helpers.StandardListenerSetup,
			secondClusterListenerSetup: helpers.SingleProxyPortSetup,
		},
		{
			name:                       "TwoClusterOnePortSetup",
			mainClusterListenerSetup:   helpers.SingleProxyPortSetup,
			secondClusterListenerSetup: helpers.SingleProxyPortSetup,
		},
		{
			name:                       "OnePortAndStandardListenerSetupLeafALPNDisabled",
			mainClusterListenerSetup:   helpers.SingleProxyPortSetup,
			secondClusterListenerSetup: helpers.StandardListenerSetup,
			disableALPNListenerOnLeaf:  true,
		},
		{
			name:                       "OnePortAndStandardListenerSetup",
			mainClusterListenerSetup:   helpers.SingleProxyPortSetup,
			secondClusterListenerSetup: helpers.StandardListenerSetup,
		},
		{
			name:                       "TrustedClusterBehindALB",
			mainClusterListenerSetup:   helpers.SingleProxyPortSetup,
			secondClusterListenerSetup: helpers.SingleProxyPortSetup,
			extraSuiteOptions:          []proxySuiteOptionsFunc{withTrustedClusterBehindALB()},
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			lib.SetInsecureDevMode(true)
			defer lib.SetInsecureDevMode(false)

			username := helpers.MustGetCurrentUser(t).Username

			opts := []proxySuiteOptionsFunc{
				withRootClusterConfig(rootClusterStandardConfig(t)),
				withLeafClusterConfig(leafClusterStandardConfig(t)),
				withRootClusterListeners(tc.mainClusterListenerSetup),
				withLeafClusterListeners(tc.secondClusterListenerSetup),
				withRootClusterRoles(newRole(t, "maindevs", username)),
				withLeafClusterRoles(newRole(t, "auxdevs", username)),
				withRootAndLeafTrustedClusterReset(),
				withTrustedCluster(),
			}
			suite := newSuite(t, append(opts, tc.extraSuiteOptions...)...)

			nodeHostname := "clusterauxnode"
			suite.addNodeToLeafCluster(t, "clusterauxnode")

			// Try and connect to a node in the Aux cluster from the Root cluster using
			// direct dialing.
			suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
				Login:   username,
				Cluster: suite.leaf.Secrets.SiteName,
				Host:    helpers.Loopback,
				Port:    helpers.Port(t, suite.leaf.SSH),
			})

			// Try and connect to a node in the Aux cluster from the Root cluster using
			// tunnel dialing.
			suite.mustConnectToClusterAndRunSSHCommand(t, helpers.ClientConfig{
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

	username := helpers.MustGetCurrentUser(t).Username

	// We need to use the non-loopback address for our Teleport cluster, as the
	// Go HTTP library will recognize requests to the loopback address and
	// refuse to use the HTTP proxy, which will invalidate the test.
	addr, err := apihelpers.GetLocalIP()
	require.NoError(t, err)

	_ = newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterNodeName(addr),
		withLeafClusterNodeName(addr),
		withRootClusterListeners(helpers.SingleProxyPortSetupOn(addr)),
		withLeafClusterListeners(helpers.SingleProxyPortSetupOn(addr)),
		withRootAndLeafClusterRoles(createTestRole(username)),
		withStandardRoleMapping(),
	)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NotZero(t, ph.Count())
	}, 10*time.Second, time.Second, "http proxy did not intercept any connection")
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

	username := helpers.MustGetCurrentUser(t).Username

	// We need to use the non-loopback address for our Teleport cluster, as the
	// Go HTTP library will recognize requests to the loopback address and
	// refuse to use the HTTP proxy, which will invalidate the test.
	addr, err := apihelpers.GetLocalIP()
	require.NoError(t, err)

	_ = newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterNodeName(addr),
		withLeafClusterNodeName(addr),
		withRootClusterListeners(helpers.SingleProxyPortSetupOn(addr)),
		withLeafClusterListeners(helpers.SingleProxyPortSetupOn(addr)),
		withRootAndLeafClusterRoles(createTestRole(username)),
		withStandardRoleMapping(),
	)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NotZero(t, ph.Count())
	}, 10*time.Second, time.Second, "http proxy did not intercept any connection")
}

// TestAlpnSniProxyKube tests Kubernetes access with custom Kube API mock where traffic is forwarded via
// SNI ALPN proxy service to Kubernetes service based on TLS SNI value.
func TestALPNSNIProxyKube(t *testing.T) {
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
					Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: types.Wildcard,
				},
			},
		},
		Options: types.RoleOptions{
			PinSourceIP: true,
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Proxy.Kube.Enabled = true
			config.Proxy.Kube.KubeconfigPath = kubeConfigPath
			config.Proxy.Kube.LegacyKubeProxy = true
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootAndLeafClusterRoles(kubeRole),
		withStandardRoleMapping(),
	)

	k8Client, k8ClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:                   suite.root,
		Username:            kubeRoleSpec.Allow.Logins[0],
		PinnedIP:            "127.0.0.1",
		KubeUsers:           kubeRoleSpec.Allow.KubeGroups,
		KubeGroups:          kubeRoleSpec.Allow.KubeUsers,
		KubeCluster:         "root.example.com",
		CustomTLSServerName: localK8SNI,
		TargetAddress:       suite.root.Config.Proxy.WebAddr,
	})
	require.NoError(t, err)

	resp, err := k8Client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 3, "pods item length mismatch")

	// Simulate how tsh uses a kube local proxy to send kube requests to
	// Teleport Proxy with a L7 LB in front.
	t.Run("ALPN connection upgrade", func(t *testing.T) {
		teleportCluster := suite.root.Config.Auth.ClusterName.GetClusterName()
		kubeCluster := "gke_project_europecentral2a_cluster1"

		k8sClient := createALPNLocalKubeClient(t,
			suite.root.Config.Proxy.WebAddr,
			teleportCluster,
			kubeCluster,
			k8ClientConfig)

		mustGetKubePod(t, k8sClient)
	})
}

// TestALPNSNIProxyKubeV2Leaf tests remove cluster kubernetes configuration where root and leaf proxies
// are using V2 configuration with Multiplex proxy listener.
func TestALPNSNIProxyKubeV2Leaf(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

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
					Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: types.Wildcard,
				},
			},
		},
		Options: types.RoleOptions{
			PinSourceIP: true,
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Proxy.Kube.Enabled = true
			config.Version = defaults.TeleportConfigVersionV2
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

	k8Client, _, err := kube.ProxyClient(kube.ProxyConfig{
		T:                   suite.root,
		Username:            kubeRoleSpec.Allow.Logins[0],
		PinnedIP:            "127.0.0.1",
		KubeUsers:           kubeRoleSpec.Allow.KubeGroups,
		KubeGroups:          kubeRoleSpec.Allow.KubeUsers,
		KubeCluster:         "gke_project_europecentral2a_cluster1",
		CustomTLSServerName: localK8SNI,
		TargetAddress:       suite.root.Config.Proxy.WebAddr,
		RouteToCluster:      suite.leaf.Secrets.SiteName,
	})
	require.NoError(t, err)

	mustGetKubePod(t, k8Client)
}

// TestKubePROXYProtocol tests correct behavior of Proxy Kube listener regarding PROXY protocol usage.
func TestKubePROXYProtocol(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	const (
		kubeCluster = constants.KubeTeleportProxyALPNPrefix + "teleport.cluster.local"
		k8User      = "alice@example.com"
		k8RoleName  = "kubemaster"
	)

	testCases := []struct {
		desc              string
		proxyListenerMode types.ProxyListenerMode
		proxyProtocolMode multiplexer.PROXYProtocolMode
		useALPNUpgrade    bool
	}{
		{
			desc:              "PROXY protocol on, separate Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolOn,
			proxyListenerMode: types.ProxyListenerMode_Separate,
		},
		{
			desc:              "PROXY protocol off, separate Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolOff,
			proxyListenerMode: types.ProxyListenerMode_Separate,
		},
		{
			desc:              "PROXY protocol unspecified, separate Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolUnspecified,
			proxyListenerMode: types.ProxyListenerMode_Separate,
		},
		{
			desc:              "PROXY protocol on, multiplexed Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolOn,
			proxyListenerMode: types.ProxyListenerMode_Multiplex,
		},
		{
			desc:              "PROXY protocol off, multiplexed Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolOff,
			proxyListenerMode: types.ProxyListenerMode_Multiplex,
		},
		{
			desc:              "PROXY protocol unspecified, multiplexed Proxy listeners",
			proxyProtocolMode: multiplexer.PROXYProtocolUnspecified,
			proxyListenerMode: types.ProxyListenerMode_Multiplex,
		},
		{
			desc:              "PROXY protocol on, multiplexed Proxy listeners with ALPN upgrade",
			proxyProtocolMode: multiplexer.PROXYProtocolOn,
			proxyListenerMode: types.ProxyListenerMode_Multiplex,
			useALPNUpgrade:    true,
		},
		{
			desc:              "PROXY protocol off, multiplexed Proxy listeners with ALPN upgrade",
			proxyProtocolMode: multiplexer.PROXYProtocolOff,
			proxyListenerMode: types.ProxyListenerMode_Multiplex,
			useALPNUpgrade:    true,
		},
	}

	username := helpers.MustGetCurrentUser(t).Username
	kubeRoleSpec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:           []string{username},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubeGroups:       []string{kube.TestImpersonationGroup},
			KubeUsers:        []string{k8User},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: types.Wildcard,
				},
			},
		},
	}

	// Create mock kube server to test connection against
	kubeAPIMockSvrRoot := startKubeAPIMock(t)
	kubeConfigPathRoot := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvrRoot.URL, kubeCluster))

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cfg := helpers.InstanceConfig{
				ClusterName: "root.example.com",
				HostID:      uuid.New().String(),
				NodeName:    helpers.Loopback,
				Logger:      utils.NewSlogLoggerForTests(),
			}
			tconf := servicecfg.MakeDefaultConfig()
			tconf.Proxy.Kube.ListenAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerProxyKube, &cfg.Fds))
			if tt.proxyListenerMode == types.ProxyListenerMode_Multiplex {
				cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
				tconf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			} else {
				cfg.Listeners = helpers.StandardListenerSetup(t, &cfg.Fds)
				tconf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				tconf.Proxy.DisableALPNSNIListener = true
			}

			testCluster := helpers.NewInstance(t, cfg)

			tconf.Version = defaults.TeleportConfigVersionV3
			tconf.DataDir = t.TempDir()
			tconf.Auth.Enabled = true
			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebInterface = true
			tconf.SSH.Enabled = false
			tconf.Proxy.Kube.Enabled = true

			tconf.Proxy.PROXYProtocolMode = tt.proxyProtocolMode

			tconf.Kube.Enabled = true
			tconf.Kube.KubeconfigPath = kubeConfigPathRoot
			tconf.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &tconf.FileDescriptors))

			// Force Proxy kube server multiplexer to check required PROXY lines on all connections
			tconf.Testing.KubeMultiplexerIgnoreSelfConnections = true

			kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
			require.NoError(t, err)

			testCluster.AddUserWithRole(username, kubeRole)

			require.NoError(t, testCluster.CreateEx(t, nil, tconf))
			require.NoError(t, testCluster.Start())
			t.Cleanup(func() {
				require.NoError(t, testCluster.StopAll())
			})

			checkForTargetAddr := func(targetAddr utils.NetAddr) {
				// If PROXY protocol is required, create load balancer in front of Teleport cluster
				if tt.proxyProtocolMode == multiplexer.PROXYProtocolOn {
					frontend := *utils.MustParseAddr("127.0.0.1:0")
					lb, err := utils.NewLoadBalancer(context.Background(), frontend)
					require.NoError(t, err)
					lb.PROXYHeader = []byte("PROXY TCP4 127.0.0.1 127.0.0.2 12345 42\r\n") // Send fake PROXY header
					lb.AddBackend(targetAddr)
					err = lb.Listen()
					require.NoError(t, err)

					go lb.Serve()
					t.Cleanup(func() { require.NoError(t, lb.Close()) })
					targetAddr = *utils.MustParseAddr(lb.Addr().String())
				}

				// Create kube client that we'll use to test that connection is working correctly.
				k8Client, kubeConfig, err := kube.ProxyClient(kube.ProxyConfig{
					T:                   testCluster,
					Username:            kubeRoleSpec.Allow.Logins[0],
					KubeUsers:           kubeRoleSpec.Allow.KubeGroups,
					KubeGroups:          kubeRoleSpec.Allow.KubeUsers,
					KubeCluster:         kubeClusterName,
					CustomTLSServerName: kubeCluster,
					TargetAddress:       targetAddr,
					RouteToCluster:      testCluster.Secrets.SiteName,
				})
				require.NoError(t, err)

				if tt.useALPNUpgrade {
					k8Client = createALPNLocalKubeClient(t,
						targetAddr,
						testCluster.Secrets.SiteName,
						kubeCluster,
						kubeConfig)
				}

				resp, err := k8Client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.Len(t, resp.Items, 3, "pods item length mismatch")
			}

			// kube listener does not support ALPN upgrade
			if !tt.useALPNUpgrade {
				checkForTargetAddr(testCluster.Config.Proxy.Kube.ListenAddr)
			}
			if tt.proxyListenerMode == types.ProxyListenerMode_Multiplex {
				checkForTargetAddr(testCluster.Config.Proxy.WebAddr)
			}
		})
	}

}

func createALPNLocalKubeClient(t *testing.T, targetAddr utils.NetAddr, teleportCluster, kubeCluster string, k8ClientConfig *rest.Config) *kubernetes.Clientset {
	// Generate a self-signed CA for kube local proxy.
	localCAKey, localCACert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{alpncommon.KubeLocalProxyWildcardDomain(teleportCluster)}, defaults.CATTL)
	require.NoError(t, err)

	// Make a mock ALB which points to the Teleport Proxy Service. Then
	// ALPN local proxies will point to this ALB instead.
	albProxy := helpers.MustStartMockALBProxy(t, targetAddr.String())

	// Create the kube local proxy.
	lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
		Listener:                mustCreateKubeLocalProxyListener(t, teleportCluster, localCACert, localCAKey),
		RemoteProxyAddr:         albProxy.Addr().String(),
		ALPNConnUpgradeRequired: true,
		InsecureSkipVerify:      true,
		SNI:                     constants.KubeTeleportProxyALPNPrefix + "teleport.cluster.local",
		HTTPMiddleware:          mustCreateKubeLocalProxyMiddleware(t, teleportCluster, kubeCluster, k8ClientConfig.CertData, k8ClientConfig.KeyData),
		Protocols:               []alpncommon.Protocol{alpncommon.ProtocolHTTP},
	})
	require.NoError(t, err)

	// Create a proxy-url for kube clients.
	fp := mustStartKubeForwardProxy(t, lp.GetAddr())

	k8Client, err := kubernetes.NewForConfig(&rest.Config{
		Host:  "https://" + teleportCluster,
		Proxy: http.ProxyURL(mustParseURL(t, "http://"+fp.GetAddr())),
		TLSClientConfig: rest.TLSClientConfig{
			CAData:     localCACert,
			CertData:   localCACert, // Client uses same cert as local proxy server.
			KeyData:    localCAKey,
			ServerName: alpncommon.KubeLocalProxySNI(teleportCluster, kubeCluster),
		},
	})
	require.NoError(t, err)
	return k8Client
}

func TestKubeIPPinning(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	modules.SetInsecureTestMode(true)

	const (
		kubeCluster = constants.KubeTeleportProxyALPNPrefix + "teleport.cluster.local"
		k8User      = "alice@example.com"
		k8RoleName  = "kubemaster"
	)

	kubeAPIMockSvrRoot := startKubeAPIMock(t)
	kubeAPIMockSvrLeaf := startKubeAPIMock(t)
	kubeConfigPathRoot := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvrRoot.URL, kubeCluster))
	kubeConfigPathLeaf := mustCreateKubeConfigFile(t, k8ClientConfig(kubeAPIMockSvrLeaf.URL, kubeCluster))

	username := helpers.MustGetCurrentUser(t).Username
	kubeRoleSpec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:           []string{username, username + "2", username + "3"},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubeGroups:       []string{kube.TestImpersonationGroup},
			KubeUsers:        []string{k8User},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: types.Wildcard,
				},
			},
		},
		Options: types.RoleOptions{
			PinSourceIP: true,
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Proxy.Kube.Enabled = true
			config.Version = defaults.TeleportConfigVersionV3

			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPathRoot
			config.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &config.FileDescriptors))
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Version = defaults.TeleportConfigVersionV3
			config.Proxy.Kube.Enabled = true

			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPathLeaf
			config.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &config.FileDescriptors))
		}),
		withRootClusterRoles(kubeRole),
		withLeafClusterRoles(kubeRole),
		withRootAndLeafTrustedClusterReset(),
		withTrustedCluster(),
	)

	testCases := []struct {
		desc           string
		pinnedIP       string
		routeToCluster string
		wantClientErr  string
	}{
		{
			desc:           "root cluster missing pinned IP",
			routeToCluster: suite.root.Secrets.SiteName,
			wantClientErr:  "pinned IP is required for the user, but is not present on identity",
		},
		{
			desc:           "root cluster wrong pinned IP",
			pinnedIP:       "127.0.0.2",
			routeToCluster: suite.root.Secrets.SiteName,
			wantClientErr:  "pinned IP doesn't match observed client IP",
		},
		{
			desc:           "root cluster pinned IP",
			pinnedIP:       "127.0.0.1",
			routeToCluster: suite.root.Secrets.SiteName,
		},
		{
			desc:           "leaf cluster missing pinned IP",
			routeToCluster: suite.leaf.Secrets.SiteName,
			wantClientErr:  "pinned IP is required for the user, but is not present on identity",
		},
		{
			desc:           "leaf cluster wrong pinned IP",
			pinnedIP:       "127.0.0.2",
			routeToCluster: suite.leaf.Secrets.SiteName,
			wantClientErr:  "pinned IP doesn't match observed client IP",
		},
		{
			desc:           "leaf cluster pinned IP",
			pinnedIP:       "127.0.0.1",
			routeToCluster: suite.leaf.Secrets.SiteName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			k8Client, _, err := kube.ProxyClient(kube.ProxyConfig{
				T:                   suite.root,
				Username:            kubeRoleSpec.Allow.Logins[0],
				PinnedIP:            tc.pinnedIP,
				KubeUsers:           kubeRoleSpec.Allow.KubeGroups,
				KubeGroups:          kubeRoleSpec.Allow.KubeUsers,
				KubeCluster:         kubeClusterName,
				CustomTLSServerName: kubeCluster,
				TargetAddress:       suite.root.Config.Proxy.WebAddr,
				RouteToCluster:      tc.routeToCluster,
			})
			require.NoError(t, err)

			resp, err := k8Client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
			if tc.wantClientErr != "" {
				require.ErrorContains(t, err, tc.wantClientErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, resp.Items, 3, "pods item length mismatch")
		})
	}
}

// TestALPNSNIProxyDatabaseAccess test DB connection forwarded through local SNI ALPN proxy where
// DB protocol is wrapped into TLS and forwarded to proxy ALPN SNI service and routed to appropriate db service.
func TestALPNSNIProxyDatabaseAccess(t *testing.T) {
	pack := dbhelpers.SetupDatabaseTest(t,
		dbhelpers.WithListenerSetupDatabaseTest(helpers.SingleProxyPortSetup),
		dbhelpers.WithLeafConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		dbhelpers.WithRootConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)
	pack.WaitForLeaf(t)

	t.Run("mysql", func(t *testing.T) {
		lp := mustStartALPNLocalProxy(t, pack.Root.Cluster.SSHProxy, alpncommon.ProtocolMySQL)
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MysqlService.Name,
					Protocol:    pack.Root.MysqlService.Protocol,
					Username:    "root",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			result, err := client.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			require.Equal(t, mysql.DefaultServerVersion, client.GetServerVersion())

			// Disconnect.
			err = client.Close()
			require.NoError(t, err)
		})

		t.Run("connect to leaf cluster via proxy", func(t *testing.T) {
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Leaf.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Leaf.MysqlService.Name,
					Protocol:    pack.Leaf.MysqlService.Protocol,
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
			pingProxy := mustStartALPNLocalProxy(t, pack.Root.Cluster.SSHProxy, alpncommon.ProtocolWithPing(alpncommon.ProtocolMySQL))
			client, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MysqlService.Name,
					Protocol:    pack.Root.MysqlService.Protocol,
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
		lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
			RemoteProxyAddr:    pack.Root.Cluster.SSHProxy,
			Protocols:          []alpncommon.Protocol{alpncommon.ProtocolPostgres},
			InsecureSkipVerify: true,
			// Since this a non-tunnel local proxy, we should check certs are needed
			// for postgres.
			// (this is how a local proxy would actually be configured for postgres).
			CheckCertNeeded: true,
		})
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.PostgresService.Name,
					Protocol:    pack.Root.PostgresService.Protocol,
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
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Leaf.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Leaf.PostgresService.Name,
					Protocol:    pack.Leaf.PostgresService.Protocol,
					Username:    "postgres",
					Database:    "test",
				},
			})
			require.NoError(t, err)
			mustRunPostgresQuery(t, client)
			mustClosePostgresClient(t, client)
		})
		t.Run("connect to main cluster via proxy with ping protocol", func(t *testing.T) {
			pingProxy := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
				RemoteProxyAddr:    pack.Root.Cluster.SSHProxy,
				Protocols:          []alpncommon.Protocol{alpncommon.ProtocolWithPing(alpncommon.ProtocolPostgres)},
				InsecureSkipVerify: true,
				// Since this a non-tunnel local proxy, we should check certs are needed
				// for postgres.
				// (this is how a local proxy would actually be configured for postgres).
				CheckCertNeeded: true,
			})
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.PostgresService.Name,
					Protocol:    pack.Root.PostgresService.Protocol,
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
		lp := mustStartALPNLocalProxy(t, pack.Root.Cluster.SSHProxy, alpncommon.ProtocolMongoDB)
		t.Run("connect to main cluster via proxy", func(t *testing.T) {
			client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MongoService.Name,
					Protocol:    pack.Root.MongoService.Protocol,
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
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Leaf.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Leaf.MongoService.Name,
					Protocol:    pack.Leaf.MongoService.Protocol,
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
			pingProxy := mustStartALPNLocalProxy(t, pack.Root.Cluster.SSHProxy, alpncommon.ProtocolWithPing(alpncommon.ProtocolMongoDB))
			client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    pingProxy.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MongoService.Name,
					Protocol:    pack.Root.MongoService.Protocol,
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
		albProxy := helpers.MustStartMockALBProxy(t, pack.Root.Cluster.Web)

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
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MongoService.Name,
					Protocol:    pack.Root.MongoService.Protocol,
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
				AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
				Address:    lp.GetAddr(),
				Cluster:    pack.Root.Cluster.Secrets.SiteName,
				Username:   pack.Root.User.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.Root.MysqlService.Name,
					Protocol:    pack.Root.MysqlService.Protocol,
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
				ServiceName: pack.Root.MysqlService.Name,
				Protocol:    pack.Root.MysqlService.Protocol,
				Username:    "root",
			}
			clientTLSConfig, err := common.MakeTestClientTLSConfig(common.TestClientConfig{
				AuthClient:      pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
				AuthServer:      pack.Root.Cluster.Process.GetAuthServer(),
				Cluster:         pack.Root.Cluster.Secrets.SiteName,
				Username:        pack.Root.User.GetName(),
				RouteToDatabase: routeToDatabase,
			})
			require.NoError(t, err)

			lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
				RemoteProxyAddr:         albProxy.Addr().String(),
				Protocols:               []alpncommon.Protocol{alpncommon.ProtocolMySQL},
				ALPNConnUpgradeRequired: true,
				InsecureSkipVerify:      true,
				Cert:                    clientTLSConfig.Certificates[0],
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

	t.Run("authenticated tunnel with cert renewal", func(t *testing.T) {
		// get a teleport client
		tc, err := pack.Root.Cluster.NewClient(helpers.ClientConfig{
			Login:   pack.Root.User.GetName(),
			Cluster: pack.Root.Cluster.Secrets.SiteName,
		})
		require.NoError(t, err)
		routeToDatabase := tlsca.RouteToDatabase{
			ServiceName: pack.Root.MysqlService.Name,
			Protocol:    pack.Root.MysqlService.Protocol,
			Username:    "root",
		}
		// inject a fake clock into the middleware so we can control when it thinks certs have expired
		fakeClock := clockwork.NewFakeClockAt(time.Now())

		// configure local proxy without certs but with cert checking/reissuing middleware
		// local proxy middleware should fetch a DB cert when the local proxy starts
		lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
			RemoteProxyAddr:    pack.Root.Cluster.SSHProxy,
			Protocols:          []alpncommon.Protocol{alpncommon.ProtocolMySQL},
			InsecureSkipVerify: true,
			Middleware:         libclient.NewDBCertChecker(tc, routeToDatabase, fakeClock),
			Clock:              fakeClock,
		})

		client, err := mysql.MakeTestClientWithoutTLS(lp.GetAddr(), routeToDatabase)
		require.NoError(t, err)

		// Execute a query.
		result, err := client.Execute("select 1")
		require.NoError(t, err)
		require.Equal(t, mysql.TestQueryResponse, result)

		// Disconnect.
		require.NoError(t, client.Close())

		// advance the fake clock and verify that the local proxy thinks its cert expired.
		fakeClock.Advance(time.Hour * 48)
		err = lp.CheckDBCert(context.Background(), routeToDatabase)
		require.Error(t, err)
		var x509Err x509.CertificateInvalidError
		require.ErrorAs(t, err, &x509Err)
		require.Equal(t, x509.Expired, x509Err.Reason)
		require.Contains(t, x509Err.Detail, "is after")

		// Open a new connection
		client, err = mysql.MakeTestClientWithoutTLS(lp.GetAddr(), routeToDatabase)
		require.NoError(t, err)

		// Execute a query.
		result, err = client.Execute("select 1")
		require.NoError(t, err)
		require.Equal(t, mysql.TestQueryResponse, result)

		// Disconnect.
		require.NoError(t, client.Close())
	})

	t.Run("teleterm db gateways cert renewal", func(t *testing.T) {
		testTeletermDbGatewaysCertRenewal(t, pack)
	})
}

// TestALPNSNIProxyAppAccess tests application access via ALPN SNI proxy service.
func TestALPNSNIProxyAppAccess(t *testing.T) {
	ctx := context.Background()
	pack := appaccess.SetupWithOptions(t, appaccess.AppTestOptions{
		RootClusterListeners: helpers.SingleProxyPortSetup,
		LeafClusterListeners: helpers.SingleProxyPortSetup,
		RootConfig: func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		},
		LeafConfig: func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		},
	})

	t.Run("root cluster", func(t *testing.T) {
		cookies := pack.CreateAppSessionCookies(t, pack.RootAppPublicAddr(), pack.RootAppClusterName())
		status, _, err := pack.MakeRequest(cookies, http.MethodGet, "/")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
	})

	t.Run("leaf cluster", func(t *testing.T) {
		cookies := pack.CreateAppSessionCookies(t, pack.LeafAppPublicAddr(), pack.LeafAppClusterName())
		status, _, err := pack.MakeRequest(cookies, http.MethodGet, "/")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
	})

	t.Run("ALPN connection upgrade", func(t *testing.T) {
		// Get client cert for app request.
		clientCerts := pack.CreateAppSessionWithClientCert(t)

		// Make a mock ALB which points to the Teleport Proxy Service. Then
		// ALPN local proxies will point to this ALB instead.
		albProxy := helpers.MustStartMockALBProxy(t, pack.RootWebAddr())

		lp := mustStartALPNLocalProxyWithConfig(t, alpnproxy.LocalProxyConfig{
			RemoteProxyAddr:         albProxy.Addr().String(),
			Protocols:               []alpncommon.Protocol{alpncommon.ProtocolHTTP},
			ALPNConnUpgradeRequired: true,
			InsecureSkipVerify:      true,
			Cert:                    clientCerts[0],
		})

		// Send the request to local proxy.
		req, err := http.NewRequest(http.MethodGet, "http://"+lp.GetAddr(), nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("teleterm app gateways cert renewal", func(t *testing.T) {
		t.Run("without per-session MFA", func(t *testing.T) {
			makeTC := func(t *testing.T) (*libclient.TeleportClient, mfa.WebauthnLoginFunc) {
				user, _ := pack.CreateUser(t)
				tc := pack.MakeTeleportClient(t, user.GetName())
				return tc, nil
			}
			testTeletermAppGateway(t, pack, makeTC)
			testTeletermAppGatewayTargetPortValidation(t, pack, makeTC)
		})

		t.Run("per-session MFA", func(t *testing.T) {
			// They update clusters authentication to Webauthn so they must run after tests which do not use MFA.
			requireSessionMFAAuthPref(ctx, t, pack.RootAuthServer(), "127.0.0.1")
			requireSessionMFAAuthPref(ctx, t, pack.LeafAuthServer(), "127.0.0.1")
			makeTCAndWebauthnLogin := func(t *testing.T) (*libclient.TeleportClient, mfa.WebauthnLoginFunc) {
				// Create a separate user for each tests to enable parallel tests that use per-session MFA.
				// See the comment for webauthnLogin in setupUserMFA for more details.
				user, _ := pack.CreateUser(t)
				tc := pack.MakeTeleportClient(t, user.GetName())
				webauthnLogin := setupUserMFA(ctx, t, pack.RootAuthServer(), user.GetName(), "127.0.0.1")
				return tc, webauthnLogin
			}
			testTeletermAppGateway(t, pack, makeTCAndWebauthnLogin)
		})
	})
}

// TestALPNProxyRootLeafAuthDial tests dialing local/remote auth service based on ALPN
// teleport-auth protocol and ServerName as encoded cluster name.
func TestALPNProxyRootLeafAuthDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	username := helpers.MustGetCurrentUser(t).Username

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t)),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootClusterListeners(helpers.SingleProxyPortSetup),
		withLeafClusterListeners(helpers.SingleProxyPortSetup),
		withRootClusterRoles(newRole(t, "rootdevs", username)),
		withLeafClusterRoles(newRole(t, "leafdevs", username)),
		withRootAndLeafTrustedClusterReset(),
		withTrustedCluster(),
	)

	client, err := suite.root.NewClient(helpers.ClientConfig{
		Login:   username,
		Cluster: suite.root.Hostname,
	})
	require.NoError(t, err)

	ctx := context.Background()

	clusterClient, err := client.ConnectToCluster(ctx)
	require.NoError(t, err)
	defer clusterClient.Close()

	// Dial root auth service.
	rootAuthClient, err := clusterClient.ConnectToCluster(ctx, "root.example.com")
	require.NoError(t, err)
	defer rootAuthClient.Close()

	pr, err := rootAuthClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, "root.example.com", pr.ClusterName)
	err = rootAuthClient.Close()
	require.NoError(t, err)

	// Dial leaf auth service.
	leafAuthClient, err := clusterClient.ConnectToCluster(ctx, "leaf.example.com")
	require.NoError(t, err)
	defer leafAuthClient.Close()

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

	cfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Version = "v2"
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	username := helpers.MustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	identityFilePath := helpers.MustCreateUserIdentityFile(t, rc, username, time.Hour)

	identity := client.LoadIdentityFile(identityFilePath)
	require.NoError(t, err)

	// Make a mock ALB which points to the Teleport Proxy Service. Then
	// client can point to this ALB instead.
	albProxy := helpers.MustStartMockALBProxy(t, rc.Web)

	tests := []struct {
		name         string
		clientConfig client.Config
	}{
		{
			name: "sync connect to Proxy",
			clientConfig: client.Config{
				Addrs:                    []string{rc.Web},
				Credentials:              []client.Credentials{identity},
				InsecureAddressDiscovery: true,
			},
		},
		{
			name: "sync connect to Proxy behind ALB",
			clientConfig: client.Config{
				Addrs:                    []string{albProxy.Addr().String()},
				Credentials:              []client.Credentials{identity},
				InsecureAddressDiscovery: true,
			},
		},
		{
			name: "background connect to Proxy",
			clientConfig: client.Config{
				Addrs:                      []string{rc.Web},
				Credentials:                []client.Credentials{identity},
				InsecureAddressDiscovery:   true,
				DialInBackground:           true,
				ALPNSNIAuthDialClusterName: cfg.ClusterName,
			},
		},
		{
			name: "background connect to Proxy behind ALB",
			clientConfig: client.Config{
				Addrs:                      []string{albProxy.Addr().String()},
				Credentials:                []client.Credentials{identity},
				InsecureAddressDiscovery:   true,
				DialInBackground:           true,
				ALPNSNIAuthDialClusterName: cfg.ClusterName,
				ALPNConnUpgradeRequired:    true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tc, err := client.New(context.Background(), test.clientConfig)
			require.NoError(t, err)

			resp, err := tc.Ping(context.Background())
			require.NoError(t, err)
			require.Equal(t, rc.Secrets.SiteName, resp.ClusterName)
		})
	}
}

// TestALPNProxyDialProxySSHWithoutInsecureMode tests dialing to the localhost with teleport-proxy-ssh
// protocol without using insecure mode in order to check if establishing connection to localhost works properly.
func TestALPNProxyDialProxySSHWithoutInsecureMode(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	rootCfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      utils.NewSlogLoggerForTests(),
	}
	rootCfg.Listeners = helpers.StandardListenerSetup(t, &rootCfg.Fds)
	rc := helpers.NewInstance(t, rootCfg)
	username := helpers.MustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	// Make root cluster config.
	rcConf := servicecfg.MakeDefaultConfig()
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
	cfg := helpers.ClientConfig{
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
	err = tc.SSH(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())
	output.Reset()

	// Try to connect to the ALPN SNI Listener.
	tc.TLSRoutingEnabled = true
	err = tc.SSH(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())
}

// TestALPNProxyHTTPProxyNoProxyDial tests if a node joining to root cluster
// takes into account http_proxy and no_proxy env variables.
func TestALPNProxyHTTPProxyNoProxyDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// We need to use the non-loopback address for our Teleport cluster, as the
	// Go HTTP library will recognize requests to the loopback address and
	// refuse to use the HTTP proxy, which will invalidate the test.
	addr, err := apihelpers.GetLocalIP()
	require.NoError(t, err)

	instanceCfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    addr,
		Logger:      utils.NewSlogLoggerForTests(),
	}
	instanceCfg.Listeners = helpers.SingleProxyPortSetupOn(addr)(t, &instanceCfg.Fds)
	rc := helpers.NewInstance(t, instanceCfg)
	username := helpers.MustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	rcConf := servicecfg.MakeDefaultConfig()
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

	rcProxyAddr := rc.Web

	// Start the node, due to no_proxy=127.0.0.1 env variable the connection established
	// to the proxy should not go through the http_proxy server.
	_, err = rc.StartNode(makeNodeConfig("first-root-node", rcProxyAddr))
	require.NoError(t, err)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
	defer cancel()

	err = rc.WaitForNodeCount(ctx, "root.example.com", 1)
	require.NoError(t, err)

	require.Zero(t, ph.Count())

	// Unset the no_proxy=127.0.0.1 env variable. After that a new node
	// should take into account the http_proxy address and connection should go through the http_proxy.
	require.NoError(t, os.Unsetenv("no_proxy"))
	_, err = rc.StartNode(makeNodeConfig("second-root-node", rcProxyAddr))
	require.NoError(t, err)
	err = rc.WaitForNodeCount(ctx, "root.example.com", 2)
	require.NoError(t, err)

	require.NotZero(t, ph.Count())
}

// TestALPNProxyHTTPProxyBasicAuthDial tests if a node joining to root cluster
// takes into account http_proxy with basic auth credentials in the address
func TestALPNProxyHTTPProxyBasicAuthDial(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	log := utils.NewSlogLoggerForTests()

	// We need to use the non-loopback address for our Teleport cluster, as the
	// Go HTTP library will recognize requests to the loopback address and
	// refuse to use the HTTP proxy, which will invalidate the test.
	rcAddr, err := apihelpers.GetLocalIP()
	require.NoError(t, err)

	cfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    rcAddr,
		Logger:      log,
	}
	cfg.Listeners = helpers.SingleProxyPortSetupOn(rcAddr)(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)
	defer rc.StopAll()

	username := helpers.MustGetCurrentUser(t).Username
	rc.AddUser(username, []string{username})

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	rcConf.Logger = log

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

	rcProxyAddr := net.JoinHostPort(rcAddr, helpers.PortStr(t, rc.Web))
	nodeCfg := makeNodeConfig("node1", rcProxyAddr)
	nodeCfg.Logger = log

	timeout := time.Second * 60
	startErrC := make(chan error)
	// start the node but don't block waiting for it while it attempts to connect to the auth server.
	go func() {
		_, err := rc.StartNode(nodeCfg)
		startErrC <- err
	}()
	require.ErrorIs(t, authorizer.WaitForRequest(timeout), trace.AccessDenied("bad credentials"))
	require.Zero(t, ph.Count())
	// stop the node so it doesn't keep trying to join the cluster with bad credentials.
	require.NoError(t, rc.StopNodes())
	require.Error(t, <-startErrC)

	// set the auth credentials to match our environment
	authorizer.SetCredentials(user, pass)

	// with env set correctly and authorized, the node should be able to register.
	go func() {
		_, err := rc.StartNode(nodeCfg)
		startErrC <- err
	}()
	require.NoError(t, <-startErrC)
	require.NoError(t, rc.WaitForNodeCount(context.Background(), rc.Secrets.SiteName, 1))
	require.Greater(t, ph.Count(), 0)
}

// TestALPNSNIProxyGRPCInsecure tests ALPN protocol ProtocolProxyGRPCInsecure
// by registering a node with IAM join method.
func TestALPNSNIProxyGRPCInsecure(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	nodeAccount := "123456789012"
	nodeRoleARN := "arn:aws:iam::123456789012:role/test"
	nodeCredentials := credentials.NewStaticCredentialsProvider("FAKE_ID", "FAKE_KEY", "FAKE_TOKEN")
	provisionToken := mustCreateIAMJoinProvisionToken(t, "iam-join-token", nodeAccount, nodeRoleARN)

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Auth.BootstrapResources = []types.Resource{provisionToken}
			config.Auth.HTTPClientForAWSSTS = fakeSTSClient{
				accountID:   nodeAccount,
				arn:         nodeRoleARN,
				credentials: nodeCredentials,
			}
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
	)

	// Test register through Proxy.
	mustRegisterUsingIAMMethod(t, suite.root.Config.Proxy.WebAddr, provisionToken.GetName(), nodeCredentials)

	// Test register through Proxy behind a L7 load balancer.
	t.Run("ALPN conn upgrade", func(t *testing.T) {
		albProxy := helpers.MustStartMockALBProxy(t, suite.root.Config.Proxy.WebAddr.Addr)
		albAddr, err := utils.ParseAddr(albProxy.Addr().String())
		require.NoError(t, err)

		mustRegisterUsingIAMMethod(t, *albAddr, provisionToken.GetName(), nodeCredentials)
	})
}

// TestALPNSNIProxyGRPCSecure tests ALPN protocol ProtocolProxyGRPCSecure
// by creating a KubeServiceClient for pod search.
func TestALPNSNIProxyGRPCSecure(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

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
					Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard}, APIGroup: types.Wildcard,
				},
			},
		},
	}
	kubeRole, err := types.NewRole(k8RoleName, kubeRoleSpec)
	require.NoError(t, err)

	suite := newSuite(t,
		withRootClusterConfig(rootClusterStandardConfig(t), func(config *servicecfg.Config) {
			config.Proxy.Kube.Enabled = true
			config.Version = defaults.TeleportConfigVersionV3
			config.Kube.Enabled = true
			config.Kube.KubeconfigPath = kubeConfigPath
			config.Kube.ListenAddr = utils.MustParseAddr(
				helpers.NewListener(t, service.ListenerKube, &config.FileDescriptors))
		}),
		withLeafClusterConfig(leafClusterStandardConfig(t)),
		withRootAndLeafClusterRoles(kubeRole),
		withStandardRoleMapping(),
	)

	t.Run("root", func(t *testing.T) {
		tc, err := suite.root.NewClient(helpers.ClientConfig{
			Login:   username,
			Cluster: suite.root.Secrets.SiteName,
			Host:    helpers.Loopback,
			Port:    helpers.Port(t, suite.root.SSH),
		})
		require.NoError(t, err)
		mustFindKubePod(t, tc)
	})
	t.Run("ALPN conn upgrade", func(t *testing.T) {
		// Make a mock ALB which points to the Teleport Proxy Service.
		albProxy := helpers.MustStartMockALBProxy(t, suite.root.Config.Proxy.WebAddr.Addr)

		tc, err := suite.root.NewClient(helpers.ClientConfig{
			Login:   username,
			Cluster: suite.root.Secrets.SiteName,
			Host:    helpers.Loopback,
			Port:    helpers.Port(t, suite.root.SSH),
			ALBAddr: albProxy.Addr().String(),
		})
		require.NoError(t, err)
		mustFindKubePod(t, tc)
	})
}
