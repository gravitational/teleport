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
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
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

func (p *kubeTestPack) testProxyKubeWithExecCmd(t *testing.T) {
	// Set KUBECONFIG to non-existent file
	t.Setenv("KUBECONFIG", filepath.Join(os.Getenv(types.HomeEnvVar), uuid.NewString()))

	tests := []struct {
		name          string
		args          []string
		expectedCmd   []string
		expectError   bool
		errorContains string
	}{
		{
			name:        "with exec-cmd",
			args:        []string{"proxy", "kube", p.rootKubeCluster1, "--insecure", "--exec", "--exec-cmd", "whoami"},
			expectedCmd: []string{"whoami"},
		},
		{
			name:        "with exec-cmd and args",
			args:        []string{"proxy", "kube", p.rootKubeCluster1, "--insecure", "--exec", "--exec-cmd", "echo", "--exec-arg", "hello", "--exec-arg", "world"},
			expectedCmd: []string{"echo", "hello", "world"},
		},
		{
			name:        "backward compatibility - no exec-cmd",
			args:        []string{"proxy", "kube", p.rootKubeCluster1, "--insecure", "--exec"},
			expectedCmd: []string{getExecCommand("")},
		},
		{
			name:        "exec-cmd without exec flag",
			args:        []string{"proxy", "kube", p.rootKubeCluster1, "--insecure", "--exec-cmd", "date"},
			expectedCmd: []string{"date"},
		},
		{
			name:          "exec-arg without exec-cmd should error",
			args:          []string{"proxy", "kube", p.rootKubeCluster1, "--insecure", "--exec-arg", "test"},
			expectError:   true,
			errorContains: "cannot use --exec-arg without --exec-cmd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectError {
				err := Run(t.Context(), tt.args)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
			} else {
				validateCmd := func(cmd *exec.Cmd) error {
					require.Equal(t, tt.expectedCmd, cmd.Args)
					return nil
				}
				err := Run(t.Context(), tt.args, setCmdRunner(validateCmd))
				require.NoError(t, err)
			}
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
		ServerName: teleportCluster,
	}
	restConfig := &rest.Config{
		Host:            "https://" + teleportCluster + common.KubeLocalProxyPathPrefix(teleportCluster, kubeCluster),
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

// TestKubeProxyCertReissuerRestoresKubeconfig verifies that the middleware cert reissuer
// recreates the ephemeral kubeconfig deleted by a relogin before it attempts the issuance,
// so a failed issuance does not leave the running proxy without its kubeconfig and the next reissue can load it again.
func TestKubeProxyCertReissuerRestoresKubeconfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kubeconfig")
	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "test-context"
	require.NoError(t, kubeconfig.Save(path, *cfg))

	cc := &fakeKubeCertClient{mfaRequired: true}
	cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
		return nil, trace.AccessDenied("issuance failed after the relogin")
	}
	issuer := newTestKubeCertIssuer(cc)
	issuer.conn = &clusterConn{dialer: reloginClusterDialer{path: path, cc: cc}}

	kubeProxy := &kubeLocalProxy{
		kubeConfigPath: path,
		kubeconfig:     cfg,
		certIssuer:     issuer,
	}

	_, err := kubeProxy.getCertReissuer()(t.Context(), "root", "kube-a")
	require.Error(t, err, "the issuance must fail in this scenario")

	restored, err := kubeconfig.Load(path)
	require.NoError(t, err, "the kubeconfig deleted by the relogin must be recreated even when the issuance fails")
	require.Equal(t, "test-context", restored.CurrentContext)
}

// reloginClusterDialer deletes the ephemeral kubeconfig when it dials, mimicking a relogin during the dial.
type reloginClusterDialer struct {
	path string
	cc   *fakeKubeCertClient
}

func (d reloginClusterDialer) DialCluster(ctx context.Context) (kubeCertClient, error) {
	if err := os.Remove(d.path); err != nil && !os.IsNotExist(err) {
		return nil, trace.Wrap(err)
	}
	return d.cc, nil
}

// TestKubeProxyCertReissuerReloginOverCachedConn verifies that
// the middleware cert reissuer recovers when the cached cluster connection is dead.
func TestKubeProxyCertReissuerReloginOverCachedConn(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kubeconfig")
	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "test-context"
	require.NoError(t, kubeconfig.Save(path, *cfg))

	clusters := newTestKubeClusters(1)
	keyRing := newTestKubeKeyRing(t, clusters)

	deadCC := &fakeKubeCertClient{mfaRequired: true}
	deadCC.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
		return nil, trace.Wrap(&interceptors.RemoteError{Err: apiclient.ErrClientCredentialsHaveExpired})
	}

	freshCC := &fakeKubeCertClient{mfaRequired: true}
	freshCC.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
		return &client.IssueUserCertsWithMFAResult{
			KeyRing:     keyRing,
			MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
		}, nil
	}

	issuer := newTestKubeCertIssuer(freshCC)
	issuer.conn = &clusterConn{dialer: reloginClusterDialer{path: path, cc: freshCC}, conn: deadCC}

	kubeProxy := &kubeLocalProxy{
		kubeConfigPath: path,
		kubeconfig:     cfg,
		certIssuer:     issuer,
	}
	reissue := kubeProxy.getCertReissuer()

	// The reissue over the dead connection fails and kubectl gets an error for this request,
	// but the issuer detects that a relogin can resolve the error and drops the connection
	// instead of leaving it lingering for the next request.
	_, err := reissue(t.Context(), clusters[0].TeleportCluster, clusters[0].KubeCluster)
	require.ErrorIs(t, err, apiclient.ErrClientCredentialsHaveExpired)
	require.Equal(t, 1, deadCC.closes, "the dead connection must be dropped so the next reissue dials afresh")

	// Steady traffic delivers the next request.
	cert, err := reissue(t.Context(), clusters[0].TeleportCluster, clusters[0].KubeCluster)
	require.NoError(t, err, "the reissue must recover once the relogin ran on the fresh dial")
	require.NotNil(t, cert.PrivateKey)

	// The ephemeral kubeconfig deleted by the relogin was recreated before the issuance.
	restored, err := kubeconfig.Load(path)
	require.NoError(t, err)
	require.Equal(t, "test-context", restored.CurrentContext)
}
