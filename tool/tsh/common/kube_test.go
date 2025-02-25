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

package common

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	kubeserver "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestKube(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	pack := setupKubeTestPack(t, true)
	t.Run("list kube", pack.testListKube)
	t.Run("proxy kube", pack.testProxyKube)
}

func TestKubeLogin(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	testKubeLogin := func(t *testing.T, kubeCluster string, expectedAddr string) {
		// Set default kubeconfig to a non-exist file to avoid loading other things.
		t.Setenv("KUBECONFIG", filepath.Join(os.Getenv(types.HomeEnvVar), uuid.NewString()))

		// Test "tsh proxy kube root-cluster1".

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		err := Run(
			ctx,
			[]string{"kube", "login", kubeCluster, "--insecure"},
		)
		require.NoError(t, err)

		k, err := kubeconfig.Load(os.Getenv("KUBECONFIG"))
		require.NoError(t, err)
		require.NotNil(t, k)

		require.Equal(t, "https://"+expectedAddr, k.Clusters[k.Contexts[k.CurrentContext].Cluster].Server)
	}

	t.Run("kube login with multiplex mode", func(t *testing.T) {
		pack := setupKubeTestPack(t, true /* withMultiplexMode */)
		webProxyAddr, err := pack.root.ProxyWebAddr()
		require.NoError(t, err)
		testKubeLogin(t, pack.rootKubeCluster1, webProxyAddr.String())
	})

	t.Run("kube login without multiplex mode", func(t *testing.T) {
		pack := setupKubeTestPack(t, false /* withMultiplexMode */)
		proxyAddr, err := pack.root.ProxyKubeAddr()
		require.NoError(t, err)
		addr := net.JoinHostPort("localhost", fmt.Sprintf("%d", proxyAddr.Port(defaults.KubeListenPort)))
		testKubeLogin(t, pack.rootKubeCluster1, addr)
	})
}

type kubeTestPack struct {
	*suite

	rootClusterName  string
	leafClusterName  string
	rootKubeCluster1 string
	rootKubeCluster2 string
	leafKubeCluster  string
}

func setupKubeTestPack(t *testing.T, withMultiplexMode bool) *kubeTestPack {
	t.Helper()

	ctx := context.Background()
	rootKubeCluster1 := "root-cluster"
	rootKubeCluster2 := "first-cluster"
	// mock a discovered kube cluster name in the leaf Teleport cluster.
	leafKubeCluster := "leaf-cluster-some-suffix-added-by-discovery-service"
	rootLabels := map[string]string{
		"label1": "val1",
		"ultra_long_label_for_teleport_kubernetes_service_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_service_list_kube_clusters_method",
	}
	leafLabels := map[string]string{
		"label1": "val1",
		"ultra_long_label_for_teleport_kubernetes_service_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_service_list_kube_clusters_method",
		// mock a discovered kube cluster in the leaf Teleport cluster.
		types.DiscoveredNameLabel: "leaf-cluster",
	}

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			if withMultiplexMode {
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			}
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, rootKubeCluster1, rootKubeCluster2)
			cfg.Kube.StaticLabels = rootLabels
			cfg.Proxy.Kube.Enabled = true
			cfg.Proxy.Kube.ListenAddr = *utils.MustParseAddr(localListenerAddr())
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
				if withMultiplexMode {
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}
				cfg.Kube.Enabled = true
				cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
				cfg.Kube.KubeconfigPath = newKubeConfigFile(t, leafKubeCluster)
				cfg.Kube.StaticLabels = leafLabels
			},
		),
		withValidationFunc(func(s *suite) bool {
			rootClusters, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			leafClusters, err := s.leaf.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			return len(rootClusters) >= 2 && len(leafClusters) >= 1
		}),
	)

	mustLoginSetEnvLegacy(t, s)
	return &kubeTestPack{
		suite:            s,
		rootClusterName:  s.root.Config.Auth.ClusterName.GetClusterName(),
		leafClusterName:  s.leaf.Config.Auth.ClusterName.GetClusterName(),
		rootKubeCluster1: rootKubeCluster1,
		rootKubeCluster2: rootKubeCluster2,
		leafKubeCluster:  leafKubeCluster,
	}
}

func (p *kubeTestPack) testListKube(t *testing.T) {
	staticRootLabels := p.suite.root.Config.Kube.StaticLabels
	formattedRootLabels := common.FormatLabels(staticRootLabels, false)
	formattedRootLabelsVerbose := common.FormatLabels(staticRootLabels, true)

	staticLeafLabels := p.suite.leaf.Config.Kube.StaticLabels
	formattedLeafLabels := common.FormatLabels(staticLeafLabels, false)
	formattedLeafLabelsVerbose := common.FormatLabels(staticLeafLabels, true)

	tests := []struct {
		name      string
		args      []string
		wantTable func() string
	}{
		{
			name: "default mode with truncated table",
			args: nil,
			wantTable: func() string {
				// p.rootKubeCluster2 ("first-cluster") should appear before
				// p.rootKubeCluster1 ("root-cluster") after sorting.
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Kube Cluster Name", "Labels", "Selected"},
					[][]string{
						{p.rootKubeCluster2, formattedRootLabels, ""},
						{p.rootKubeCluster1, formattedRootLabels, ""},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "show complete list of labels",
			args: []string{"--verbose"},
			wantTable: func() string {
				table := asciitable.MakeTable(
					[]string{"Kube Cluster Name", "Labels", "Selected"},
					[]string{p.rootKubeCluster2, formattedRootLabelsVerbose, ""},
					[]string{p.rootKubeCluster1, formattedRootLabelsVerbose, ""})
				return table.AsBuffer().String()
			},
		},
		{
			name: "show headless table",
			args: []string{"--quiet"},
			wantTable: func() string {
				table := asciitable.MakeHeadlessTable(2)
				table.AddRow([]string{p.rootKubeCluster2, formattedRootLabels, ""})
				table.AddRow([]string{p.rootKubeCluster1, formattedRootLabels, ""})

				return table.AsBuffer().String()
			},
		},
		{
			name: "list all clusters including leaf clusters",
			args: []string{"--all"},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Proxy", "Cluster", "Kube Cluster Name", "Labels"},
					[][]string{
						// "leaf-cluster" should be displayed instead of the
						// full leaf cluster name, since it is mocked as a
						// discovered resource and the discovered resource name
						// is displayed in non-verbose mode.
						{p.root.Config.Proxy.WebAddr.String(), "leaf1", "leaf-cluster", formattedLeafLabels},
						{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster2, formattedRootLabels},
						{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster1, formattedRootLabels},
					},
					"Labels",
				)
				return table.AsBuffer().String()
			},
		},
		{
			name: "list all clusters including leaf clusters with complete list of labels",
			args: []string{"--all", "--verbose"},
			wantTable: func() string {
				table := asciitable.MakeTable(
					[]string{"Proxy", "Cluster", "Kube Cluster Name", "Labels"},
					[]string{p.root.Config.Proxy.WebAddr.String(), "leaf1", p.leafKubeCluster, formattedLeafLabelsVerbose},
					[]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster2, formattedRootLabelsVerbose},
					[]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster1, formattedRootLabelsVerbose},
				)
				return table.AsBuffer().String()
			},
		},
		{
			name: "list all clusters including leaf clusters in headless table",
			args: []string{"--all", "--quiet"},
			wantTable: func() string {
				table := asciitable.MakeHeadlessTable(4)
				table.AddRow([]string{p.root.Config.Proxy.WebAddr.String(), "leaf1", "leaf-cluster", formattedLeafLabels})
				table.AddRow([]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster2, formattedRootLabels})
				table.AddRow([]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster1, formattedRootLabels})
				return table.AsBuffer().String()
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			captureStdout := new(bytes.Buffer)
			err := Run(
				context.Background(),
				append([]string{
					"--insecure",
					"kube",
					"ls",
				},
					tc.args...,
				),
				setCopyStdout(captureStdout),

				// set a custom empty kube config for each test, as we do
				// not want parallel (or even shuffled sequential) tests
				// potentially racing on the same config
				setKubeConfigPath(filepath.Join(t.TempDir(), "kubeconfig")),
			)
			require.NoError(t, err)
			got := strings.TrimSpace(captureStdout.String())
			want := strings.TrimSpace(tc.wantTable())
			diff := cmp.Diff(want, got)
			require.Empty(t, diff)
		})
	}
}

// Tests `tsh kube login`, `tsh proxy kube`.
func TestKubeSelection(t *testing.T) {
	modules.SetTestModules(t,
		&modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.K8s: {Enabled: true},
				},
			},
		},
	)
	testenv.WithInsecureDevMode(t, true)
	testenv.WithResyncInterval(t, 0)

	// Create a role that allows the user to request access to a restricted
	// cluster but not to access it directly.
	user, err := user.Current()
	require.NoError(t, err)

	const roleName = "restricted"
	role, err := types.NewRole(
		roleName,
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups: []string{user.Username},
				KubernetesLabels: types.Labels{
					"env": []string{"dev", "prod"},
				},
				Request: &types.AccessRequestConditions{
					SearchAsRoles: []string{"access"},
				},
			},
		},
	)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			// reconfig the user to use the new role instead of the default ones
			// User is the second bootstrap resource.
			user, ok := cfg.Auth.BootstrapResources[1].(types.User)
			require.True(t, ok)
			user.SetRoles([]string{roleName})
			cfg.Auth.BootstrapResources = append(cfg.Auth.BootstrapResources, role)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.SSH.Enabled = false
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.ResourceMatchers = []services.ResourceMatcher{{
				Labels: map[string]apiutils.Strings{"*": {"*"}},
			}}
			// Do not use a fake clock to better imitate real-world behavior.
		}),
	)
	kubeBarEKS := "bar-eks-us-west-1-123456789012"
	kubeBazEKS1 := "baz-eks-us-west-1-123456789012"
	kubeBazEKS2 := "baz-eks-us-west-2-123456789012"
	kubeRootEKS := "root-cluster-eks-us-east-1-123456789012"
	mustRegisterKubeClusters(t, ctx, s.root.GetAuthServer(),
		mustMakeDynamicKubeCluster(t, kubeBarEKS, "bar", map[string]string{types.DiscoveryLabelRegion: "us-west-1", "env": "dev"}),
		mustMakeDynamicKubeCluster(t, kubeBazEKS1, "baz", map[string]string{types.DiscoveryLabelRegion: "us-west-1", "env": "prod"}),
		mustMakeDynamicKubeCluster(t, kubeBazEKS2, "baz", map[string]string{types.DiscoveryLabelRegion: "us-west-2", "env": "prod"}),
		mustMakeDynamicKubeCluster(t, kubeRootEKS, "root-cluster", map[string]string{types.DiscoveryLabelRegion: "us-east-2", "env": "restricted"}),
	)
	allKubes := []string{kubeBarEKS, kubeBazEKS1, kubeBazEKS2}

	rootClusterName := s.root.Config.Auth.ClusterName.GetClusterName()

	tests := []struct {
		desc                    string
		wantLoginCurrentContext string
		wantLoggedIn            []string
		wantProxied             []string
		args                    []string
		// indicate if a test case is only for one of the `tsh kube login`
		// or `tsh proxy kube` test runners.
		// A lot of the test cases can be shared to test `tsh kube login` and
		// `tsh proxy kube`, but some are specific.
		loginTestOnly bool
		proxyTestOnly bool
		wantErr       string
	}{
		{
			desc:                    "with full name",
			wantLoginCurrentContext: kubeBazEKS1,
			wantLoggedIn:            []string{kubeBazEKS1},
			wantProxied:             []string{kubeBazEKS1},
			args:                    []string{kubeBazEKS1},
		},
		{
			desc:                    "with discovered name",
			wantLoginCurrentContext: kubeBarEKS,
			wantLoggedIn:            []string{kubeBarEKS},
			wantProxied:             []string{kubeBarEKS},
			args:                    []string{"bar"},
		},
		{
			desc:         "with labels",
			wantLoggedIn: []string{kubeBazEKS1, kubeBazEKS2},
			wantProxied:  []string{kubeBazEKS1, kubeBazEKS2},
			args:         []string{"--labels", "env=prod"},
		},
		{
			desc:         "with query",
			wantLoggedIn: []string{kubeBazEKS1},
			wantProxied:  []string{kubeBazEKS1},
			args:         []string{"--query", `labels["env"]=="prod" && labels["region"] == "us-west-1"`},
		},
		{
			desc: "with labels and discovered name",
			// both these match the labels, only one of them matches the discovered name to select the context.
			wantLoginCurrentContext: kubeBazEKS1,
			wantLoggedIn:            []string{kubeBarEKS, kubeBazEKS1},
			wantProxied:             []string{kubeBazEKS1},
			args: []string{
				"--labels", "region=us-west-1",
				"baz",
			},
		},
		{
			desc:                    "with query and discovered name",
			wantLoginCurrentContext: kubeBazEKS2,
			wantLoggedIn:            []string{kubeBazEKS2},
			wantProxied:             []string{kubeBazEKS2},
			args: []string{
				"--query", `labels["region"] == "us-west-2"`,
				"baz",
			},
		},
		{
			desc: "ambiguous discovered name is an error",
			args: []string{
				"baz",
			},
			wantErr: `kubernetes cluster "baz" matches multiple`,
		},
		{
			desc: "zero name matches is an error",
			args: []string{
				"xxx",
			},
			wantErr: `kubernetes cluster "xxx" not found`,
		},
		{
			desc: "zero label matches is an error",
			args: []string{
				"--labels", "env=nonexistent",
			},
			wantErr: `kubernetes cluster with labels "env=nonexistent" not found`,
		},
		{
			desc: "zero query matches is an error",
			args: []string{
				"--query", `labels["env"]=="nonexistent"`,
			},
			wantErr: `kubernetes cluster with query (labels["env"]=="nonexistent") not found`,
		},
		// cases specific to `tsh kube login` testing
		{
			desc:                    "login to all and set current context by full name",
			args:                    []string{kubeBazEKS1, "--all"},
			wantLoginCurrentContext: kubeBazEKS1,
			wantLoggedIn:            allKubes,
			loginTestOnly:           true,
		},
		{
			desc:                    "login to all and set current context by discovered name",
			args:                    []string{kubeBarEKS, "--all"},
			wantLoginCurrentContext: kubeBarEKS,
			wantLoggedIn:            allKubes,
			loginTestOnly:           true,
		},
		{
			desc:          "login to all and set current context by ambiguous discovered name is an error",
			args:          []string{"baz", "--all"},
			loginTestOnly: true,
			wantErr:       `kubernetes cluster "baz" matches multiple`,
		},
		{
			desc:          "login with all",
			args:          []string{"--all"},
			wantLoggedIn:  allKubes,
			loginTestOnly: true,
		},
		{
			desc:          "all with labels is an error",
			args:          []string{"xxx", "--all", "--labels", `env=root`},
			loginTestOnly: true,
			wantErr:       "cannot use",
		},
		{
			desc:          "all with query is an error",
			args:          []string{"xxx", "--all", "--query", `name == "foo-bar" || name == "foo"`},
			loginTestOnly: true,
			wantErr:       "cannot use",
		},
		{
			desc:          "missing required args is an error",
			args:          []string{},
			loginTestOnly: true,
			wantErr:       "required",
		},
		// cases specific to `tsh proxy kube` testing
		{
			desc:          "proxy multiple",
			wantProxied:   []string{kubeBazEKS1, kubeBazEKS2, kubeBarEKS},
			args:          []string{kubeBazEKS1, kubeBazEKS2, kubeBarEKS},
			proxyTestOnly: true,
		},
		{
			desc:          "proxy multiple with one ambiguous discovered name",
			args:          []string{kubeBarEKS, "baz"},
			wantErr:       "matches multiple",
			proxyTestOnly: true,
		},
		{
			desc:          "proxy multiple with query resolving ambiguity",
			wantProxied:   []string{kubeBarEKS, kubeBazEKS2},
			args:          []string{kubeBarEKS, "baz", "--query", `labels.region == "us-west-2" || labels.env == "dev"`},
			proxyTestOnly: true,
		},
	}

	t.Run("proxy", func(t *testing.T) {
		t.Parallel()
		for _, test := range tests {
			if test.loginTestOnly {
				// skip test cases specific to `tsh kube login`.
				continue
			}
			test := test
			t.Run(test.desc, func(t *testing.T) {
				t.Parallel()
				// login for each parallel test to avoid races when multiple tsh
				// clients work in the same profile dir.
				tshHome, _ := mustLoginLegacy(t, s)
				// Set kubeconfig to a non-exist file to avoid loading other things.
				kubeConfigPath := filepath.Join(tshHome, "kube-config")
				var cmdRunner func(*exec.Cmd) error
				if len(test.wantProxied) > 0 {
					cmdRunner = func(cmd *exec.Cmd) error {
						config := kubeConfigFromCmdEnv(t, cmd)
						for _, kube := range test.wantProxied {
							checkKubeLocalProxyConfig(t, config, rootClusterName, kube)
						}
						return nil
					}
				}
				err := Run(ctx, append([]string{"proxy", "kube", "--insecure", "--port", ports.Pop()}, test.args...),
					setCmdRunner(cmdRunner),
					setHomePath(tshHome),
					setKubeConfigPath(kubeConfigPath),
				)
				if test.wantErr != "" {
					require.ErrorContains(t, err, test.wantErr)
					return
				}
				require.NoError(t, err)
			})
		}
	})

	t.Run("login", func(t *testing.T) {
		t.Parallel()
		webProxyAddr, err := utils.ParseAddr(s.root.Config.Proxy.WebAddr.String())
		require.NoError(t, err)
		// profile kube config path depends on web proxy host
		webProxyHost := webProxyAddr.Host()
		for _, test := range tests {
			if test.proxyTestOnly {
				continue
			}
			test := test
			t.Run(test.desc, func(t *testing.T) {
				t.Parallel()
				tshHome, kubeConfigPath := mustLoginLegacy(t, s)
				err := Run(
					context.Background(),
					append([]string{"kube", "login", "--insecure"},
						test.args...,
					),
					setHomePath(tshHome),
					// set a custom empty kube config for each test, as we do
					// not want parallel (or even shuffled sequential) tests
					// potentially racing on the same config
					setKubeConfigPath(kubeConfigPath),
				)
				if test.wantErr != "" {
					require.ErrorContains(t, err, test.wantErr)
					return
				}
				require.NoError(t, err)

				// load the global kube config.
				config, err := kubeconfig.Load(kubeConfigPath)
				require.NoError(t, err)

				// check that the kube config context is set to what we expect.
				if test.wantLoginCurrentContext == "" {
					require.Empty(t, config.CurrentContext)
				} else {
					require.Equal(t,
						kubeconfig.ContextName("root", test.wantLoginCurrentContext),
						config.CurrentContext,
					)
				}

				// check which kube clusters were added to the global kubeconfig.
				for _, name := range allKubes {
					contextName := kubeconfig.ContextName("root", name)
					if !slices.Contains(test.wantLoggedIn, name) {
						require.NotContains(t, config.AuthInfos, contextName, "unexpected kube cluster %v in config update", name)
						continue
					}
					require.Contains(t, config.AuthInfos, contextName, "kube cluster %v not in config update", name)
					authInfo := config.AuthInfos[contextName]
					require.NotNil(t, authInfo)
					require.Contains(t, authInfo.Exec.Args, fmt.Sprintf("--kube-cluster=%v", name))
				}

				// ensure the profile config only contains one kube cluster.
				profileKubeConfigPath := keypaths.KubeConfigPath(
					profile.FullProfilePath(tshHome),
					webProxyHost,
					s.user.GetName(),
					s.root.Config.Auth.ClusterName.GetClusterName(),
					test.wantLoginCurrentContext,
				)

				// load the profile kube config
				profileConfig, err := kubeconfig.Load(profileKubeConfigPath)
				require.NoError(t, err)

				// check that the kube config context is set to what we expect.
				if test.wantLoginCurrentContext == "" {
					require.Empty(t, profileConfig.CurrentContext)
				} else {
					require.Equal(t,
						kubeconfig.ContextName("root", test.wantLoginCurrentContext),
						profileConfig.CurrentContext,
					)
				}
				for _, name := range allKubes {
					contextName := kubeconfig.ContextName("root", name)
					if name != test.wantLoginCurrentContext {
						require.NotContains(t, profileConfig.AuthInfos, contextName, "unexpected kube cluster %v in profile config update", name)
						continue
					}
					require.Contains(t, profileConfig.AuthInfos, contextName, "kube cluster %v not in profile config update", name)
					authInfo := profileConfig.AuthInfos[contextName]
					require.NotNil(t, authInfo)
					require.Contains(t, authInfo.Exec.Args, fmt.Sprintf("--kube-cluster=%v", name))
				}
			})
		}
	})

	t.Run("access request", func(t *testing.T) {
		t.Parallel()
		// login as the user.
		tshHome, kubeConfig := mustLoginLegacy(t, s)

		// Run the login command in a goroutine so we can check if the access
		// request was created and approved.
		// The goroutine will exit when the access request is approved.
		wg := &errgroup.Group{}
		wg.Go(func() error {
			err := Run(
				context.Background(),
				[]string{
					"--insecure",
					"kube",
					"login",
					// by discovered name
					"root-cluster",
					"--request-reason",
					"test",
				},
				setHomePath(tshHome),
				setKubeConfigPath(kubeConfig),
			)
			// assert no error for more useful error message when access request is
			// never created. assert instead of require because it's in a goroutine.
			assert.NoError(t, err)
			return trace.Wrap(err)
		})
		// Wait for the access request to be created and finally approve it.
		var accessRequestID string
		require.Eventually(t, func() bool {
			accessRequests, err := s.root.GetAuthServer().
				GetAccessRequests(
					context.Background(),
					types.AccessRequestFilter{State: types.RequestState_PENDING},
				)
			if err != nil || len(accessRequests) != 1 {
				return false
			}

			equal := reflect.DeepEqual(
				accessRequests[0].GetRequestedResourceIDs(),
				[]types.ResourceID{
					{
						ClusterName: s.root.Config.Auth.ClusterName.GetClusterName(),
						Kind:        types.KindKubernetesCluster,
						Name:        kubeRootEKS,
					},
				},
			)
			accessRequestID = accessRequests[0].GetName()

			return equal
		}, 10*time.Second, 500*time.Millisecond, "waiting for access request to be created")
		// Approve the access request to release the login command lock.
		err := s.root.GetAuthServer().SetAccessRequestState(context.Background(), types.AccessRequestUpdate{
			RequestID: accessRequestID,
			State:     types.RequestState_APPROVED,
		})
		require.NoError(t, err)
		// Wait for the login command to exit after the request is approved
		require.NoError(t, wg.Wait())
	})
}

func newKubeConfigFile(t *testing.T, clusterNames ...string) string {
	tmpDir := t.TempDir()

	kubeConf := clientcmdapi.NewConfig()
	for _, name := range clusterNames {
		kubeConf.Clusters[name] = &clientcmdapi.Cluster{
			Server:                newKubeSelfSubjectServer(t),
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[name] = &clientcmdapi.Context{
			Cluster:  name,
			AuthInfo: name,
		}
	}
	kubeConfigLocation := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*kubeConf, kubeConfigLocation)
	require.NoError(t, err)
	return kubeConfigLocation
}

func newKubeConfig(t *testing.T, name string) []byte {
	kubeConf := clientcmdapi.NewConfig()

	kubeConf.Clusters[name] = &clientcmdapi.Cluster{
		Server:                newKubeSelfSubjectServer(t),
		InsecureSkipTLSVerify: true,
	}
	kubeConf.AuthInfos[name] = &clientcmdapi.AuthInfo{}

	kubeConf.Contexts[name] = &clientcmdapi.Context{
		Cluster:  name,
		AuthInfo: name,
	}

	buf, err := clientcmd.Write(*kubeConf)
	require.NoError(t, err)
	return buf
}

func newKubeSelfSubjectServer(t *testing.T) string {
	srv, err := kubeserver.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	return srv.URL
}
