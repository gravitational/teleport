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
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestProxyKube(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	rootKubeCluster1 := "root-kube1"
	rootKubeCluster2 := "root-kube2"
	leafKubeCluster := "leaf-kube"

	ctx := context.Background()
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, rootKubeCluster1, rootKubeCluster2)
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, leafKubeCluster)
		}),
		withValidationFunc(func(s *suite) bool {
			rootClusters, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			leafClusters, err := s.leaf.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			return len(rootClusters) >= 2 && len(leafClusters) >= 1
		}),
	)

	mustLoginSetEnv(t, s)

	// Set default kubeconfig to a non-exist file to avoid loading other things.
	t.Setenv("KUBECONFIG", path.Join(os.Getenv(types.HomeEnvVar), uuid.NewString()))

	rootClusterName := s.root.Config.Auth.ClusterName.GetClusterName()
	leafClusterName := s.leaf.Config.Auth.ClusterName.GetClusterName()

	// Test "tsh proxy kube root-cluster1".
	t.Run("with kube cluster arg", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		validateCmd := func(cmd *exec.Cmd) error {
			config := kubeConfigFromCmdEnv(t, cmd)
			checkKubeLocalProxyConfig(t, s, config, rootClusterName, rootKubeCluster1)
			return nil
		}
		err := Run(
			ctx,
			[]string{"proxy", "kube", rootKubeCluster1},
			setCmdRunner(validateCmd),
		)
		require.NoError(t, err)
	})

	// Test "tsh proxy kube" after "tsh login"s.
	t.Run("without kube cluster arg", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		require.NoError(t, Run(ctx, []string{"kube", "login", rootKubeCluster2, "--insecure"}))
		require.NoError(t, Run(ctx, []string{"kube", "login", leafKubeCluster, "-c", leafClusterName, "--insecure"}))

		validateCmd := func(cmd *exec.Cmd) error {
			config := kubeConfigFromCmdEnv(t, cmd)
			checkKubeLocalProxyConfig(t, s, config, rootClusterName, rootKubeCluster2)
			checkKubeLocalProxyConfig(t, s, config, leafClusterName, leafKubeCluster)
			return nil
		}
		err := Run(
			ctx,
			[]string{"proxy", "kube"},
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

	checkKubeLocalProxyConfigPaths(t, s, config, teleportCluster, kubeCluster)
	sendKubeLocalProxyRequest(t, config, teleportCluster, kubeCluster)
}

// checkKubeLocalProxyConfigPaths check some basic values.
func checkKubeLocalProxyConfigPaths(t *testing.T, s *suite, config *clientcmdapi.Config, teleportCluster, kubeCluster string) {
	t.Helper()

	tshHome := os.Getenv(types.HomeEnvVar)
	proxy := s.root.Config.Auth.ClusterName.GetClusterName()
	user := s.user.GetName()
	wantCAPath := keypaths.KubeLocalCAPath(tshHome, proxy, user, teleportCluster)
	wantKeyPath := keypaths.UserKeyPath(tshHome, proxy, user)

	contextName := kubeconfig.ContextName(teleportCluster, kubeCluster)
	authInfo := config.AuthInfos[contextName]
	require.NotNil(t, authInfo)
	clusterInfo := config.Clusters[contextName]
	require.NotNil(t, clusterInfo)

	require.Equal(t, wantCAPath, authInfo.ClientCertificate)
	require.Equal(t, wantKeyPath, authInfo.ClientKey)
	require.Equal(t, wantCAPath, clusterInfo.CertificateAuthority)
}

// sendKubeLocalProxyRequest makes a request with a bad SNI and looks for the
// "no client cert found" error by local proxy's KubeMiddleware. If found, it
// means the request has successfully went through forward proxy "CONNECT"
// upgrade and ALPN local proxy TLS handshake. We want the request to stop here
// to avoid reaching Proxy further.
func sendKubeLocalProxyRequest(t *testing.T, config *clientcmdapi.Config, teleportCluster, kubeCluster string) {
	t.Helper()

	request, err := http.NewRequest("Get", "https://localhost", nil)
	require.NoError(t, err)

	client, badServerName := clientForKubeLocalProxy(t, config, teleportCluster, kubeCluster)

	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()

	require.Equal(t, http.StatusNotFound, response.StatusCode)

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "no client cert found for "+badServerName)
}

func clientForKubeLocalProxy(t *testing.T, config *clientcmdapi.Config, teleportCluster, kubeCluster string) (http.Client, string) {
	t.Helper()

	contextName := kubeconfig.ContextName(teleportCluster, kubeCluster)
	authInfo := config.AuthInfos[contextName]
	clusterInfo := config.Clusters[contextName]

	require.True(t, strings.HasPrefix(clusterInfo.ProxyURL, "http://127.0.0.1:"))
	proxyURL, err := url.Parse(clusterInfo.ProxyURL)
	require.NoError(t, err)
	clientCert, err := tls.LoadX509KeyPair(authInfo.ClientCertificate, authInfo.ClientKey)
	require.NoError(t, err)
	serverCAs, err := utils.NewCertPoolFromPath(clusterInfo.CertificateAuthority)
	require.NoError(t, err)

	badServerName := fmt.Sprintf("%s.%s", uuid.NewString(), teleportCluster)
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      serverCAs,
				ServerName:   badServerName,
			},
		},
	}
	return client, badServerName
}
