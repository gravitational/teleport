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
		portforwardClientBuilder func(*testing.T, portForwardRequestConfig) portForwarder
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "SPDY protocol",
			args: args{
				portforwardClientBuilder: spdyPortForwardClientBuilder,
			},
		},
		{
			name: "Websocket protocol",
			args: args{
				portforwardClientBuilder: websocketPortForwardClientBuilder,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// readyCh communicate when the port forward is ready to get traffic
			readyCh := make(chan struct{})
			// errCh receives a single error from ForwardPorts go routine.
			errCh := make(chan error)
			t.Cleanup(func() { require.NoError(t, <-errCh) })
			// stopCh control the port forwarding lifecycle. When it gets closed the
			// port forward will terminate.
			stopCh := make(chan struct{})
			t.Cleanup(func() { close(stopCh) })

			fw := tt.args.portforwardClientBuilder(t, portForwardRequestConfig{
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
				// When we receive an error instead of a ready signal, it means that
				// fw.ForwardPorts() setup failed.
				// fw.ForwardPorts() setup creates a listener, a connection to the
				// Teleport Kubernetes Service and upgrades the connection to SPDY
				// or WebSocket. Either of these cases can return an error.
				// After the setup finishes readyCh is notified, and fw.ForwardPorts()
				// runs until the upstream server reports any error or fw.Close executes.
				// fw.ForwardPorts() only returns err=nil if properly closed using
				// fw.Close, otherwise err!=nil.
				t.Fatalf("Received error on errCh instead of a ready signal: %v", err)
			case <-readyCh:
				// portforward creates a listener at localPort.
				// Once client dials to localPort, portforward client will connect to
				// the upstream (Teleport) and copy the data from the local connection
				// into the upstream and from the upstream into the local connection.
				// The connection is closed if the upstream reports any error and
				// ForwardPorts returns it.
				// Dial a connection to localPort.
				conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", localPort))
				require.NoError(t, err)
				t.Cleanup(func() { conn.Close() })
				_, err = conn.Write(stdinContent)
				require.NoError(t, err)
				p := make([]byte, 1024)
				n, err := conn.Read(p)
				require.NoError(t, err)
				// Make sure we hit the upstream server and that the upstream received
				// the contents written into the connection.
				// Expected payload: testingkubemock.PortForwardPayload podName stdinContent
				expected := fmt.Sprint(testingkubemock.PortForwardPayload, podName, string(stdinContent))
				require.Equal(t, expected, string(p[:n]))
			}
		})
	}
}

func portforwardURL(namespace, podName string, host string, query string) (*url.URL, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.Scheme = "https"
	u.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		namespace, podName)
	u.RawQuery = query

	return u, nil
}

func spdyPortForwardClientBuilder(t *testing.T, req portForwardRequestConfig) portForwarder {
	transport, upgrader, err := spdy.RoundTripperFor(req.restConfig)
	require.NoError(t, err)
	u, err := portforwardURL(req.podNamespace, req.podName, req.restConfig.Host, "")
	require.NoError(t, err)
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, u)
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.localPort, req.podPort)}, req.stopCh, req.readyCh, os.Stdout, os.Stdin)
	require.NoError(t, err)
	return fw
}

func websocketPortForwardClientBuilder(t *testing.T, req portForwardRequestConfig) portForwarder {
	// "ports=8080" it's ok to send the mandatory port as anything since the upstream
	// testing mock does not care about the port.
	u, err := portforwardURL(req.podNamespace, req.podName, req.restConfig.Host, "ports=8080")
	require.NoError(t, err)
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

type portForwarder interface {
	ForwardPorts() error
	Close()
}
