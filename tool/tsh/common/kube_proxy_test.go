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

package common

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
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
