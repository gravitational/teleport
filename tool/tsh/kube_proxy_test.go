/*
Copyright 2023 Gravitational, Inc.

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

package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func (p *kubeTestPack) testProxyKube(t *testing.T) {
	// Set default kubeconfig to a non-exist file to avoid loading other things.
	t.Setenv("KUBECONFIG", path.Join(os.Getenv(types.HomeEnvVar), uuid.NewString()))

	// Test "tsh proxy kube root-cluster1".
	t.Run("with kube cluster arg", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		validateCmd := func(cmd *exec.Cmd) error {
			config := kubeConfigFromCmdEnv(t, cmd)
			checkKubeLocalProxyConfig(t, p.suite, config, p.rootClusterName, p.rootKubeCluster1)
			return nil
		}
		err := Run(
			ctx,
			[]string{"proxy", "kube", p.rootKubeCluster1, "--insecure"},
			setCmdRunner(validateCmd),
		)
		require.NoError(t, err)
	})

	// Test "tsh proxy kube" after "tsh login"s.
	t.Run("without kube cluster arg", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		require.NoError(t, Run(ctx, []string{"kube", "login", p.rootKubeCluster2, "--insecure"}))
		require.NoError(t, Run(ctx, []string{"kube", "login", p.leafKubeCluster, "-c", p.leafClusterName, "--insecure"}))

		validateCmd := func(cmd *exec.Cmd) error {
			config := kubeConfigFromCmdEnv(t, cmd)
			checkKubeLocalProxyConfig(t, p.suite, config, p.rootClusterName, p.rootKubeCluster2)
			checkKubeLocalProxyConfig(t, p.suite, config, p.leafClusterName, p.leafKubeCluster)
			return nil
		}
		err := Run(
			ctx,
			[]string{"proxy", "kube", "--insecure"},
			setCmdRunner(validateCmd),
		)
		require.NoError(t, err)
	})
}

func TestProxyKubeComplexSelectors(t *testing.T) {
	testenv.WithInsecureDevMode(t, true)
	testenv.WithResyncInterval(t, 0)
	kubeFoo := "foo"
	kubeFooBar := "foo-bar"
	kubeBaz := "baz-qux"
	kubeBazEKS := "baz-eks-us-west-1-123456789012"
	kubeFooLeaf := "foo"
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.SSH.Enabled = false
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, kubeFoo, kubeFooBar, kubeBaz)
			cfg.Kube.StaticLabels = map[string]string{"env": "root"}
			cfg.Kube.ResourceMatchers = []services.ResourceMatcher{{
				Labels: map[string]apiutils.Strings{"*": {"*"}},
			}}
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				cfg.SSH.Enabled = false
				cfg.Kube.Enabled = true
				cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
				cfg.Kube.KubeconfigPath = newKubeConfigFile(t, kubeFooLeaf)
				cfg.Kube.StaticLabels = map[string]string{"env": "leaf"}
			},
		),
		withValidationFunc(func(s *suite) bool {
			rootClusters, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			leafClusters, err := s.leaf.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			return len(rootClusters) == 3 && len(leafClusters) == 1
		}),
	)
	// setup a fake "discovered" kube cluster by adding a discovered name label
	// to a dynamic kube cluster.
	kc, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: kubeBazEKS,
			Labels: map[string]string{
				types.DiscoveredNameLabel: "baz",
				types.OriginLabel:         types.OriginDynamic,
			},
		},
		types.KubernetesClusterSpecV3{
			Kubeconfig: newKubeConfig(t, kubeBazEKS),
		},
	)
	require.NoError(t, err)
	err = s.root.GetAuthServer().CreateKubernetesCluster(ctx, kc)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		servers, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
		assert.NoError(c, err)
		for _, ks := range servers {
			if ks.GetName() == kubeBazEKS {
				return
			}
		}
		assert.Fail(c, "kube server not found")
	}, time.Second*10, time.Millisecond*500, "failed to find dynamically created kube cluster %v", kubeBazEKS)

	rootClusterName := s.root.Config.Auth.ClusterName.GetClusterName()
	leafClusterName := s.leaf.Config.Auth.ClusterName.GetClusterName()

	tests := []struct {
		desc              string
		makeValidateCmdFn func(*testing.T) func(*exec.Cmd) error
		args              []string
		wantErr           string
	}{
		{
			desc: "with full name",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFoo)
					return nil
				}
			},
			args: []string{kubeFoo, "--insecure"},
		},
		{
			desc: "with discovered name",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeBazEKS)
					return nil
				}
			},
			args: []string{"baz", "--insecure"},
		},
		{
			desc: "with prefix name",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFooBar)
					return nil
				}
			},
			args: []string{"foo-b", "--insecure"},
		},
		{
			desc: "with labels",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFoo)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFooBar)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeBaz)
					return nil
				}
			},
			args: []string{"--labels", "env=root", "--insecure"},
		},
		{
			desc: "with query",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFoo)
					return nil
				}
			},
			args: []string{"--query", `labels["env"]=="root"`, "--insecure"},
		},
		{
			desc: "with labels, query, and prefix",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, rootClusterName, kubeFoo)
					return nil
				}
			},
			args: []string{
				"--labels", "env=root",
				"--query", `name == "foo"`,
				"f", // prefix of "foo".
				"--insecure",
			},
		},
		{
			desc: "in leaf cluster with prefix name",
			makeValidateCmdFn: func(t *testing.T) func(*exec.Cmd) error {
				return func(cmd *exec.Cmd) error {
					config := kubeConfigFromCmdEnv(t, cmd)
					checkKubeLocalProxyConfig(t, s, config, leafClusterName, kubeFooLeaf)
					return nil
				}
			},
			args: []string{
				"--cluster", leafClusterName,
				"--insecure",
				"f", // prefix of "foo" kube cluster in leaf teleport cluster.
			},
		},
		{
			desc: "ambiguous name prefix is an error",
			args: []string{
				"f", // prefix of foo, foo-bar in root cluster.
				"--insecure",
			},
			wantErr: `kubernetes cluster "f" matches multiple`,
		},
		{
			desc: "zero name matches is an error",
			args: []string{
				"xxx",
				"--insecure",
			},
			wantErr: `kubernetes cluster "xxx" not found`,
		},
		{
			desc: "zero label matches is an error",
			args: []string{
				"--labels", "env=nonexistent",
				"--insecure",
			},
			wantErr: `kubernetes cluster with labels "env=nonexistent" not found`,
		},
		{
			desc: "zero query matches is an error",
			args: []string{
				"--query", `labels["env"]=="nonexistent"`,
				"--insecure",
			},
			wantErr: `kubernetes cluster with query (labels["env"]=="nonexistent") not found`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			// login for each parallel test to avoid races when multiple tsh
			// clients work in the same profile dir.
			tshHome, _ := mustLogin(t, s)
			// Set kubeconfig to a non-exist file to avoid loading other things.
			kubeConfigPath := path.Join(tshHome, "kube-config")
			var cmdRunner func(*exec.Cmd) error
			if test.makeValidateCmdFn != nil {
				cmdRunner = test.makeValidateCmdFn(t)
			}
			err := Run(ctx, append([]string{"proxy", "kube", "--port", ports.Pop()}, test.args...),
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
}

func kubeConfigFromCmdEnv(t *testing.T, cmd *exec.Cmd) *clientcmdapi.Config {
	t.Helper()

	for _, env := range cmd.Env {
		if !strings.HasPrefix(env, "KUBECONFIG=") {
			continue
		}
		path := strings.TrimPrefix(env, "KUBECONFIG=")
		isProfilePath, err := keypaths.IsProfileKubeConfigPath(path)
		require.NoError(t, err)
		require.True(t, isProfilePath)

		config, err := kubeconfig.Load(path)
		require.NoError(t, err)
		return config
	}

	require.Fail(t, "no KUBECONFIG found")
	return nil
}

func checkKubeLocalProxyConfig(t *testing.T, s *suite, config *clientcmdapi.Config, teleportCluster, kubeCluster string) {
	t.Helper()

	sendRequestToKubeLocalProxy(t, config, teleportCluster, kubeCluster)
}

func sendRequestToKubeLocalProxy(t *testing.T, config *clientcmdapi.Config, teleportCluster, kubeCluster string) {
	t.Helper()

	contextName := kubeconfig.ContextName(teleportCluster, kubeCluster)

	require.NotNil(t, config)
	require.NotNil(t, config.Clusters)
	require.Contains(t, config.Clusters, contextName)
	proxyURL, err := url.Parse(config.Clusters[contextName].ProxyURL)
	require.NoError(t, err)

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     config.Clusters[contextName].CertificateAuthorityData,
		CertData:   config.AuthInfos[contextName].ClientCertificateData,
		KeyData:    config.AuthInfos[contextName].ClientKeyData,
		ServerName: common.KubeLocalProxySNI(teleportCluster, kubeCluster),
	}

	client, err := kubernetes.NewForConfig(&rest.Config{
		Host:            "https://" + teleportCluster,
		TLSClientConfig: tlsClientConfig,
		Proxy:           http.ProxyURL(proxyURL),
	})
	require.NoError(t, err)

	resp, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.Nil(t, err)
	require.GreaterOrEqual(t, len(resp.Items), 1)
}
