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
	"crypto/ecdsa"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

func (p *kubeTestPack) testProxyKube(t *testing.T) {
	// Set default kubeconfig to a non-exist file to avoid loading other things.
	t.Setenv("KUBECONFIG", filepath.Join(os.Getenv(types.HomeEnvVar), uuid.NewString()))

	// Test "tsh proxy kube root-cluster1".
	t.Run("with kube cluster arg", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(cancel)

		validateCmd := func(cmd *exec.Cmd) error {
			config := kubeConfigFromCmdEnv(t, cmd)
			checkKubeLocalProxyConfig(t, config, p.rootClusterName, p.rootKubeCluster1)
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
			checkKubeLocalProxyConfig(t, config, p.rootClusterName, p.rootKubeCluster2)
			checkKubeLocalProxyConfig(t, config, p.leafClusterName, p.leafKubeCluster)
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

func checkKubeLocalProxyConfig(t *testing.T, config *clientcmdapi.Config, teleportCluster, kubeCluster string) {
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

	// Sanity check we're using an ECDSA client key.
	key, err := keys.ParsePrivateKey(config.AuthInfos[contextName].ClientKeyData)
	require.NoError(t, err)
	require.IsType(t, (*ecdsa.PrivateKey)(nil), key.Signer)

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     config.Clusters[contextName].CertificateAuthorityData,
		CertData:   config.AuthInfos[contextName].ClientCertificateData,
		KeyData:    config.AuthInfos[contextName].ClientKeyData,
		ServerName: common.KubeLocalProxySNI(teleportCluster, kubeCluster),
	}
	restConfig := &rest.Config{
		Host:            "https://" + teleportCluster,
		TLSClientConfig: tlsClientConfig,
		Proxy:           http.ProxyURL(proxyURL),
	}
	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	resp, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	runKubectlExec(t, restConfig)
}

// runKubectlExec runs a kubectl exec command in a dummy pod.
// The mock Kubernetes API server will return the pod name and the stdin data
// written to the pod.
func runKubectlExec(t *testing.T, config *rest.Config) {
	var (
		stdinWrite               = &bytes.Buffer{}
		stdout                   = &bytes.Buffer{}
		stderr                   = &bytes.Buffer{}
		podName                  = "teleport"
		podNamespace             = "default"
		podContainerName         = "teleportContainer"
		containerCommmandExecute = []string{"sh"}
		stdinContent             = []byte("stdin_data")
	)

	_, err := stdinWrite.Write(stdinContent)
	require.NoError(t, err)

	streamOpts := remotecommand.StreamOptions{
		Stdin:  io.NopCloser(stdinWrite),
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	}

	req, err := generateExecRequest(
		generateExecRequestConfig{
			config:        config,
			podName:       podName,
			podNamespace:  podNamespace,
			containerName: podContainerName,
			cmd:           containerCommmandExecute, // placeholder for commands to execute in the dummy pod
			options:       streamOpts,
		},
	)
	require.NoError(t, err)

	exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	require.NoError(t, err)

	err = exec.StreamWithContext(context.Background(), streamOpts)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s\n%s", podContainerName, string(stdinContent)), stdout.String())
}

// generateExecRequestConfig is the config for generating a Kube API url for
// executing commands in pods.
type generateExecRequestConfig struct {
	// config is the rest config for the cluster.
	config *rest.Config
	// podName is the name of the pod to execute the command in.
	podName string
	// podNamespace is the namespace of the pod to execute the command in.
	podNamespace string
	// containerName is the name of the container to execute the command in.
	containerName string
	// cmd is the command to execute in the container.
	cmd []string
	// options are the options for the command execution.
	options remotecommand.StreamOptions
}

// generateExecRequest generates a Kube API url for executing commands in pods.
// The url format is the following:
// "/api/v1/namespaces/{podNamespace}/pods/{podName}/exec?stderr={stdout}&stdout={stdout}&tty={tty}&reason={reason}&container={containerName}&command={command}"
func generateExecRequest(config generateExecRequestConfig) (*rest.Request, error) {
	restClient, err := rest.RESTClientFor(
		&rest.Config{
			Host:    config.config.Host,
			APIPath: "/api",
			ContentConfig: rest.ContentConfig{
				GroupVersion:         &corev1.SchemeGroupVersion,
				NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
			},
			TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		},
	)
	if err != nil {
		return nil, err
	}

	req := restClient.Post().
		Resource("pods").
		Name(config.podName).
		Namespace(config.podNamespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: config.containerName,
			Command:   config.cmd,
			Stdin:     config.options.Stdin != nil,
			Stdout:    config.options.Stdout != nil,
			Stderr:    config.options.Stderr != nil,
			TTY:       config.options.Tty,
		}, scheme.ParameterCodec)

	return req, nil
}
