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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/utils"
)

func TestPortForwardKubeService(t *testing.T) {
	t.Parallel()

	missingPermissions := metav1.Status{
		Status:  metav1.StatusFailure,
		Message: "missing permissions",
		Reason:  metav1.StatusReasonForbidden,
		Code:    http.StatusForbidden,
	}

	type args struct {
		portforwardClientBuilder func(*testing.T, portForwardRequestConfig) portForwarder
		opts                     []testingkubemock.Option
	}
	tests := []struct {
		name    string
		args    args
		wantErr *metav1.Status
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
		{
			name: "SPDY protocol error",
			args: args{
				portforwardClientBuilder: spdyPortForwardClientBuilder,
				opts: []testingkubemock.Option{
					testingkubemock.WithPortForwardError(missingPermissions),
				},
			},
			wantErr: &missingPermissions,
		},
		{
			name: "Websocket protocol error",
			args: args{
				portforwardClientBuilder: websocketPortForwardClientBuilder,
				opts: []testingkubemock.Option{
					testingkubemock.WithPortForwardError(missingPermissions),
				},
			},
			wantErr: &missingPermissions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kubeMock, err := testingkubemock.NewKubeAPIMock(tt.args.opts...)
			require.NoError(t, err)
			t.Cleanup(func() { kubeMock.Close() })

			// creates a Kubernetes service with a configured cluster pointing to mock api server
			testCtx := SetupTestContext(
				context.Background(),
				t,
				TestConfig{
					Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
				},
			)

			t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

			// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
			user, _ := testCtx.CreateUserAndRole(
				testCtx.Context,
				t,
				username,
				RoleSpec{
					Name:       roleName,
					KubeUsers:  roleKubeUsers,
					KubeGroups: roleKubeGroups,
				})

			// generate a kube client with user certs for auth
			_, config := testCtx.GenTestKubeClientTLSCert(
				t,
				user.GetName(),
				kubeCluster,
			)
			require.NoError(t, err)

			// readyCh communicate when the port forward is ready to get traffic
			readyCh := make(chan struct{})
			// errCh receives a single error from ForwardPorts goroutine.
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
				if tt.wantErr != nil {
					require.ErrorContains(t, err, (&kubeerrors.StatusError{
						ErrStatus: *tt.wantErr,
					}).Error())
					return
				}
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
				ports, err := fw.GetPorts()
				require.NoError(t, err)
				require.Len(t, ports, 1)

				conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", ports[0].Local))
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
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", 0, req.podPort)}, req.stopCh, req.readyCh, os.Stdout, os.Stdin)
	require.NoError(t, err)
	return fw
}

func websocketPortForwardClientBuilder(t *testing.T, req portForwardRequestConfig) portForwarder {
	// "ports=8080" it's ok to send the mandatory port as anything since the upstream
	// testing mock does not care about the port.
	u, err := portforwardURL(req.podNamespace, req.podName, req.restConfig.Host, "ports=8080")
	require.NoError(t, err)
	client, err := newWebSocketClient(req.restConfig, "GET", u, withLocalPortforwarding(req.readyCh))
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
	// podPort is the target port for the pod.
	podPort int
	// stopCh is the channel used to manage the port forward lifecycle
	stopCh <-chan struct{}
	// readyCh communicates when the tunnel is ready to receive traffic
	readyCh chan struct{}
}

type portForwarder interface {
	ForwardPorts() error
	GetPorts() ([]portforward.ForwardedPort, error)
	Close()
}

// TestPortForwardProxy_run_connsClosed tests that the port forward proxy cleans up the
// spdy stream when it is closed. This is important because the spdy connection
// holds a reference to the stream and if the stream is not removed from the
// connection, it will leak memory.
func TestPortForwardProxy_run_connsClosed(t *testing.T) {
	t.Parallel()
	const (
		reqID = "reqID"
		// portHeaderValue is the value of the port header in the stream.
		// This value is not used to listen for requests, but it is used to identify the stream
		// destination.
		portHeaderValue = "8080"
	)

	sourceConn := newfakeSPDYConnection()
	targetConn := newfakeSPDYConnection()
	h := &portForwardProxy{
		portForwardRequest: portForwardRequest{
			context:       context.Background(),
			onPortForward: func(addr string, success bool) {},
		},
		logger:                utils.NewSlogLoggerForTests(),
		sourceConn:            sourceConn,
		targetConn:            targetConn,
		streamChan:            make(chan httpstream.Stream),
		streamPairs:           map[string]*httpStreamPair{},
		streamCreationTimeout: 5 * time.Second,
	}

	go func() {
		dataStream, err := sourceConn.CreateStream(http.Header{
			PortForwardRequestIDHeader: []string{reqID},
			PortHeader:                 []string{portHeaderValue},
			StreamType:                 []string{StreamTypeError},
		})
		assert.NoError(t, err)
		h.streamChan <- dataStream
		errStream, err := sourceConn.CreateStream(http.Header{
			PortForwardRequestIDHeader: []string{reqID},
			PortHeader:                 []string{portHeaderValue},
			StreamType:                 []string{StreamTypeData},
		})
		assert.NoError(t, err)
		h.streamChan <- errStream
		// Close the source after the streams are processed to unblock the call.
		sourceConn.Close()
	}()
	// run the port forward proxy. it will read the h.streamChan and
	// process the streams synchronously. Once the streams are processed,
	// the sourceConn will be closed and the proxy will exit the run loop.
	h.run()
	// targetConn is closed once all streams are removed. It is an hack to
	// unblock the targetConn.waitForClose() call otherwise it will block
	// forever.
	require.Eventually(t, func() bool {
		select {
		case <-targetConn.closed:
			return true
		default:
			return false
		}
	}, 5*time.Second, 100*time.Millisecond, "streams werent properly removed from targetConn")

	require.True(t, sourceConn.streamsClosed(), "sourceConn streams not closed")
	require.True(t, targetConn.streamsClosed(), "targetConn streams not closed")
}

type fakeSPDYStream struct {
	closed     bool
	headers    http.Header
	identifier uint32
	mu         sync.Mutex
}

func (f *fakeSPDYStream) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (f *fakeSPDYStream) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (f *fakeSPDYStream) Headers() http.Header {
	return f.headers
}

func (f *fakeSPDYStream) Reset() error {
	return nil
}

func (f *fakeSPDYStream) Identifier() uint32 {
	return f.identifier
}

func (f *fakeSPDYStream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeSPDYStream) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

type fakeSPDYConnection struct {
	count        int
	streams      map[uint32]*fakeSPDYStream
	streamsSlice []*fakeSPDYStream
	closed       chan bool
	closedOnce   sync.Once
	mu           sync.Mutex
}

func newfakeSPDYConnection() *fakeSPDYConnection {
	return &fakeSPDYConnection{
		streams: make(map[uint32]*fakeSPDYStream),
		closed:  make(chan bool),
	}
}

// CreateStream creates a new Stream with the supplied headers.
func (f *fakeSPDYConnection) CreateStream(headers http.Header) (httpstream.Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	newHeader := http.Header{}
	for k, v := range headers {
		newHeader.Set(k, v[0])
	}
	f.count++
	identifier := uint32(f.count)
	stream := &fakeSPDYStream{identifier: identifier, headers: newHeader}
	f.streamsSlice = append(f.streamsSlice, stream)
	f.streams[identifier] = stream
	return stream, nil
}

// Close resets all streams and closes the connection.
func (f *fakeSPDYConnection) Close() error {
	f.closedOnce.Do(func() {
		close(f.closed)
	})
	return nil
}

// CloseChan returns a channel that is closed when the underlying connection is closed.
func (f *fakeSPDYConnection) CloseChan() <-chan bool {
	return f.closed
}

// SetIdleTimeout sets the amount of time the connection may remain idle before
// it is automatically closed.
func (f *fakeSPDYConnection) SetIdleTimeout(_ time.Duration) {}

// RemoveStreams can be used to remove a set of streams from the Connection.
func (f *fakeSPDYConnection) RemoveStreams(streams ...httpstream.Stream) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, stream := range streams {
		if stream == nil {
			continue
		}
		delete(f.streams, stream.Identifier())
	}
	// if there are no streams left, close the connection so the test can exit
	if len(f.streams) == 0 {
		f.Close()
	}
}

func (f *fakeSPDYConnection) streamsClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.streams) != 0 {
		return false
	}
	for _, stream := range f.streamsSlice {
		if !stream.isClosed() {
			return false
		}
	}
	return true
}

func TestPortForwardUnderlyingProtocol(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		version      *version.Info
		validateFunc func(*testing.T, *testingkubemock.KubeMockServer)
	}{
		{
			name: "SPDY protocol, version < 1.31",
			version: &version.Info{
				GitVersion: "v1.30.0",
			},
			validateFunc: func(t *testing.T, kms *testingkubemock.KubeMockServer) {
				// forward used SPDY to kubernetes API
				require.EqualValues(t, 1, kms.KubePortforward.SPDY.Load())
				require.EqualValues(t, 0, kms.KubePortforward.Websocket.Load())
			},
		},
		{
			name: "Websocket protocol for clusters >=1.31",
			version: &version.Info{
				GitVersion: "v1.31.0",
			},
			validateFunc: func(t *testing.T, kms *testingkubemock.KubeMockServer) {
				// forward used SPDY over websocket to kubernetes API
				require.EqualValues(t, 0, kms.KubePortforward.SPDY.Load())
				require.EqualValues(t, 1, kms.KubePortforward.Websocket.Load())
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kubeMock, err := testingkubemock.NewKubeAPIMock(
				testingkubemock.WithVersion(tt.version),
			)
			require.NoError(t, err)
			t.Cleanup(func() { kubeMock.Close() })
			t.Cleanup(func() {
				tt.validateFunc(t, kubeMock)
			})
			// creates a Kubernetes service with a configured cluster pointing to mock api server
			testCtx := SetupTestContext(
				context.Background(),
				t,
				TestConfig{
					Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
				},
			)

			t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

			// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
			user, _ := testCtx.CreateUserAndRole(
				testCtx.Context,
				t,
				username,
				RoleSpec{
					Name:       roleName,
					KubeUsers:  roleKubeUsers,
					KubeGroups: roleKubeGroups,
				})

			// generate a kube client with user certs for auth
			_, config := testCtx.GenTestKubeClientTLSCert(
				t,
				user.GetName(),
				kubeCluster,
			)
			require.NoError(t, err)
			// readyCh communicate when the port forward is ready to get traffic
			readyCh := make(chan struct{})
			// errCh receives a single error from ForwardPorts goroutine.
			errCh := make(chan error)
			t.Cleanup(func() { require.NoError(t, <-errCh) })
			// stopCh control the port forwarding lifecycle. When it gets closed the
			// port forward will terminate.
			stopCh := make(chan struct{})
			t.Cleanup(func() { close(stopCh) })

			fw := spdyPortForwardClientBuilder(t, portForwardRequestConfig{
				podName:      podName,
				podNamespace: podNamespace,
				restConfig:   config,
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

				ports, err := fw.GetPorts()
				require.NoError(t, err)
				require.Len(t, ports, 1)

				conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", ports[0].Local))
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
