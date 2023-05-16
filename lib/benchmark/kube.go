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
	var k *client.Key
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		k, err = tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		}, nil /*applyOpts*/)
		return err
	})
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	certPem := k.KubeTLSCerts[tc.KubernetesCluster]

	rsaKeyPEM, err := k.PrivateKey.RSAPrivateKeyPEM()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	credentials, err := tc.LocalAgent().GetCoreKey()
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
		CertData:   certPem,
		KeyData:    rsaKeyPEM,
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
	if k.Interactive {
		// If interactive, we need to set up a pty and we cannot use the
		// stderr stream because the server will hang.
		tc.Stderr = nil
	} else {
		// If not interactive, we need to set up stdin to be nil so that
		// the server wont wait for input.
		tc.Stdin = nil
	}
	exec, err := k.kubeExecOnPod(ctx, tc, restCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		stdin := tc.Stdin
		if k.Interactive {
			stdin = bytes.NewBuffer([]byte(strings.Join(k.Command, " ") + "\r\nexit\r\n"))
		}
		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: tc.Stdout,
			Stderr: tc.Stderr,
			Tty:    k.Interactive,
		})
		return trace.Wrap(err)
	}, nil
}

func (k KubeExecBenchmark) kubeExecOnPod(ctx context.Context, tc *client.TeleportClient, restConfig *rest.Config) (remotecommand.Executor, error) {
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
		Stdin:     tc.Stdin != nil,
		Stdout:    tc.Stdout != nil,
		Stderr:    tc.Stderr != nil,
		TTY:       k.Interactive,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	return exec, trace.Wrap(err)
}
