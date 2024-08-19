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

package benchmark

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/gravitational/teleport/lib/client"
)

// KubeListBenchmark is a benchmark suite that runs successive kubectl get pods
// against a Teleport Kubernetes proxy for a given duration and rate.
type KubeListBenchmark struct {
	// Namespace is the Kubernetes namespace to run the command against.
	// If empty, it will include pods from all namespaces.
	Namespace string
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (k KubeListBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	restCfg, err := newKubernetesRestConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	restCfg.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(500000, 500000)
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		// List all pods in all namespaces.
		_, err := clientset.CoreV1().Pods(k.Namespace).List(ctx, metav1.ListOptions{})
		return trace.Wrap(err)
	}, nil
}

// newKubernetesRestConfig returns a new rest.Config for the kubernetes cluster
// that the client wants to connected to.
func newKubernetesRestConfig(ctx context.Context, tc *client.TeleportClient) (*rest.Config, error) {
	tlsClientConfig, err := getKubeTLSClientConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	restConfig := &rest.Config{
		Host:            tc.KubeClusterAddr(),
		TLSClientConfig: tlsClientConfig,
		APIPath:         "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: scheme.Codecs,
		},
	}
	return restConfig, nil
}

// getKubeTLSClientConfig returns a TLS client config for the kubernetes cluster
// that the client wants to connected to.
func getKubeTLSClientConfig(ctx context.Context, tc *client.TeleportClient) (rest.TLSClientConfig, error) {
	var k *client.KeyRing
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		k, err = tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		})
		return err
	})
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	cred := k.KubeTLSCredentials[tc.KubernetesCluster]

	keyPEM, err := cred.PrivateKey.SoftwarePrivateKeyPEM()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	credentials, err := tc.LocalAgent().GetCoreKeyRing()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	var clusterCAs [][]byte
	if tc.LoadAllCAs {
		clusterCAs = credentials.TLSCAs()
	} else {
		clusterCAs, err = credentials.RootClusterCAs()
		if err != nil {
			return rest.TLSClientConfig{}, trace.Wrap(err)
		}
	}
	if len(clusterCAs) == 0 {
		return rest.TLSClientConfig{}, trace.BadParameter("no trusted CAs found")
	}

	tlsServerName := ""
	if tc.TLSRoutingEnabled {
		k8host, _ := tc.KubeProxyHostPort()
		tlsServerName = client.GetKubeTLSServerName(k8host)
	}

	return rest.TLSClientConfig{
		CAData:     bytes.Join(clusterCAs, []byte("\n")),
		CertData:   cred.Cert,
		KeyData:    keyPEM,
		ServerName: tlsServerName,
	}, nil
}

// KubeListBenchmark is a benchmark suite that runs successive kubectl exec
// against a Teleport Kubernetes proxy for a given duration and rate.
type KubeExecBenchmark struct {
	// Namespace is the Kubernetes namespace to run the command against.
	Namespace string
	// PodName is the name of the pod to run the command against.
	PodName string
	// ContainerName is the name of the container to run the command against.
	ContainerName string
	// Command is the command to run.
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (k KubeExecBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	restCfg, err := newKubernetesRestConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stdin := tc.Stdin
	stderr := tc.Stderr
	if k.Interactive {
		// If interactive, we need to set up a pty and we cannot use the
		// stderr stream because the server will hang.
		stderr = nil
	} else {
		// If not interactive, we need to set up stdin to be nil so that
		// the server wont wait for input.
		stdin = nil
	}
	exec, err := k.kubeExecOnPod(ctx, restCfg, stdin != nil, tc.Stdout != nil, stderr != nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if k.Interactive {
			stdin = bytes.NewBuffer([]byte(strings.Join(k.Command, " ") + "\r\nexit\r\n"))
		}
		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: tc.Stdout,
			Stderr: stderr,
			Tty:    k.Interactive,
		})
		return trace.Wrap(err)
	}, nil
}

func (k KubeExecBenchmark) kubeExecOnPod(ctx context.Context, restConfig *rest.Config, hasStdin, hasStdout, hasStderr bool) (remotecommand.Executor, error) {
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := restClient.Post().
		Resource("pods").
		Name(k.PodName).
		Namespace(k.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: k.ContainerName,
		Command:   k.Command,
		Stdin:     hasStdin,
		Stdout:    hasStdout,
		Stderr:    hasStderr,
		TTY:       k.Interactive,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	return exec, trace.Wrap(err)
}
