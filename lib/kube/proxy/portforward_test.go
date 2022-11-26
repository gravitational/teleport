/*
Copyright 2022 Gravitational, Inc.

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

package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestPortForwardKubeService(t *testing.T) {
	const (
		localPort = 9084
	)
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := setupTestContext(
		context.Background(),
		t,
		testConfig{
			clusters: []kubeClusterConfig{{name: kubeCluster, apiEndpoint: kubeMock.URL}},
		},
	)

	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	user, _ := testCtx.createUserAndRole(
		testCtx.ctx,
		t,
		username,
		roleSpec{
			name:       roleName,
			kubeUsers:  roleKubeUsers,
			kubeGroups: roleKubeGroups,
		})

	// generate a kube client with user certs for auth
	_, config := testCtx.genTestKubeClientTLSCert(
		t,
		user.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

	type args struct {
		portforwardBuilder func(*testing.T, portForwardRequestConfig) forwardPorts
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "SPDY protocol",
			args: args{
				portforwardBuilder: spdyPortForward,
			},
		},
		{
			name: "Websocket protocol",
			args: args{
				portforwardBuilder: websocketPortForward,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// readyCh communicate when the port forward is ready to get traffic
			readyCh := make(chan struct{})
			errCh := make(chan error, 1)
			t.Cleanup(func() {
				require.NoError(t, trace.Wrap(<-errCh))
			})
			// stopCh control the port forwarding lifecycle. When it gets closed the
			// port forward will terminate
			stopCh := make(chan struct{}, 1)
			t.Cleanup(
				func() {
					close(stopCh)
				},
			)

			fw := tt.args.portforwardBuilder(t, portForwardRequestConfig{
				podName:      podName,
				podNamespace: podNamespace,
				restConfig:   config,
				localPort:    localPort,
				podPort:      80,
				stopCh:       stopCh,
				readyCh:      readyCh,
			})
			require.NoError(t, err)
			t.Cleanup(fw.Close)
			go func() {
				defer close(errCh)
				errCh <- trace.Wrap(fw.ForwardPorts())
			}()

			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-readyCh:
				conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", localPort))
				require.NoError(t, err)
				t.Cleanup(func() { conn.Close() })
				_, err = conn.Write(stdinContent)
				require.NoError(t, err)
				p := make([]byte, 1024)
				n, err := conn.Read(p)
				require.NoError(t, err)
				require.Equal(t, fmt.Sprintf(testingkubemock.PortForwardPayload, podName, string(stdinContent)), string(p[:n]))
			}
		})
	}
}

func portforwardURL(namespace, podName string, host string, query string) *url.URL {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		namespace, podName)
	// trim https://
	hostIP := strings.TrimLeft(host, "htps:/")
	return &url.URL{Scheme: "https", Path: path, Host: hostIP, RawQuery: query}
}

func spdyPortForward(t *testing.T, req portForwardRequestConfig) forwardPorts {
	transport, upgrader, err := spdy.RoundTripperFor(req.restConfig)
	require.NoError(t, err)
	u := portforwardURL(req.podNamespace, req.podName, req.restConfig.Host, "")
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, u)
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.localPort, req.podPort)}, req.stopCh, req.readyCh, os.Stdout, os.Stdin)
	require.NoError(t, err)
	return fw
}

func websocketPortForward(t *testing.T, req portForwardRequestConfig) forwardPorts {
	// "ports=8080" it's ok to send the mandatory port as anything since the upstream
	// testing mock does not care about the port.
	u := portforwardURL(req.podNamespace, req.podName, req.restConfig.Host, "ports=8080")
	client, err := newWebSocketClient(req.restConfig, "GET", u, withLocalPortforwarding(int32(req.localPort), req.readyCh))
	require.NoError(t, err)
	return client
}

type portForwardRequestConfig struct {
	// restConfig is the Teleport user restConfig.
	restConfig *rest.Config
	// podName is the pod for this port forwarding.
	podName string
	// podNamespace is the pod namespace.
	podNamespace string
	// localPort is the local port that will be selected to expose the PodPort
	localPort int
	// podPort is the target port for the pod.
	podPort int
	// stopCh is the channel used to manage the port forward lifecycle
	stopCh <-chan struct{}
	// readyCh communicates when the tunnel is ready to receive traffic
	readyCh chan struct{}
}

type forwardPorts interface {
	ForwardPorts() error
	Close()
}
