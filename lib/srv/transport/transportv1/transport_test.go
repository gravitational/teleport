// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transportv1

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	proxyv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/proxy/v1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

	os.Exit(m.Run())
}

// echoConn is a [net.Conn] echos the data received
// back to the other side of the connection.
type echoConn struct {
	net.Conn

	mu     sync.Mutex
	buffer bytes.Buffer
	data   chan struct{}
	closed bool
	sync.Once
}

func (e *echoConn) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed == true {
		return 0, io.EOF
	}

	n, err := e.buffer.Write(p)

	e.data <- struct{}{}

	return n, err
}

func (e *echoConn) Read(p []byte) (int, error) {
	<-e.data

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed == true {
		return 0, io.EOF
	}

	return e.buffer.Read(p)
}

func (e *echoConn) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.buffer.Reset()
	e.Once.Do(func() {
		close(e.data)
		e.closed = true
	})

	return nil
}

// fakeDialer implements [Dialer] with a static map of
// site and host to connections.
type fakeDialer struct {
	siteConns map[string]net.Conn
	hostConns map[string]net.Conn
}

func (f fakeDialer) DialSite(ctx context.Context, clusterName string) (net.Conn, error) {
	conn, ok := f.siteConns[clusterName]
	if !ok {
		return nil, trace.NotFound(clusterName)
	}

	return conn, nil
}

func (f fakeDialer) DialHost(ctx context.Context, from net.Addr, host, port, clusterName string, accessChecker services.AccessChecker, agentGetter teleagent.Getter) (net.Conn, error) {
	key := fmt.Sprintf("%s.%s.%s", host, port, clusterName)
	conn, ok := f.hostConns[key]
	if !ok {
		return nil, trace.NotFound(key)
	}

	return conn, nil
}

// testPack used to test a [Service].
type testPack struct {
	Client proxyv1.ProxyServiceClient
	Server *Service
}

// newServer creates a [Service] with the provided config and
// an authenticated client to exercise various RPCs on the [Service].
func newServer(t *testing.T, cfg ServerConfig) testPack {
	// gRPC testPack.
	const bufSize = 100 // arbitrary
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})

	s := grpc.NewServer(
		grpc.StreamInterceptor(utils.GRPCServerStreamErrorInterceptor),
		grpc.UnaryInterceptor(utils.GRPCServerUnaryErrorInterceptor),
	)
	t.Cleanup(func() {
		s.GracefulStop()
		s.Stop()
	})

	srv, err := NewService(cfg)
	require.NoError(t, err)

	// Register service.
	proxyv1.RegisterProxyServiceServer(s, srv)

	// Start.
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(fmt.Sprintf("Serve returned err = %v", err))
		}
	}()

	// gRPC client.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, "unused",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(utils.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(utils.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return testPack{
		Client: proxyv1.NewProxyServiceClient(cc),
		Server: srv,
	}
}

// TestServer_GetClusterDetails validates that a [Service] returns
// the expected cluster details.
func TestServer_GetClusterDetails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		FIPS bool
	}{
		{
			name: "FIPS disabled",
		},
		{
			name: "FIPS enabled",
			FIPS: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, ServerConfig{
				Dialer: fakeDialer{},
				FIPS:   test.FIPS,
			})

			resp, err := srv.Client.GetClusterDetails(context.Background(), &proxyv1.GetClusterDetailsRequest{})
			require.NoError(t, err)
			require.Equal(t, test.FIPS, resp.Details.FipsEnabled)
		})
	}
}

// TestServer_ProxyCluster validates that a [Service] proxies data to
// and from a target cluster.
func TestServer_ProxyCluster(t *testing.T) {
	t.Parallel()
	const cluster = "test"

	tests := []struct {
		name string
		fn   func(t *testing.T, stream proxyv1.ProxyService_ProxyClusterClient, conn *echoConn)
	}{
		{
			name: "transport established to cluster",
			fn: func(t *testing.T, stream proxyv1.ProxyService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&proxyv1.ProxyClusterRequest{Cluster: cluster}))

				var msg = []byte("hello")
				require.NoError(t, stream.Send(&proxyv1.ProxyClusterRequest{Frame: &proxyv1.Frame{Payload: msg}}))

				resp, err := stream.Recv()
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Frame)
				require.Equal(t, msg, resp.Frame.Payload)

				require.NoError(t, stream.CloseSend())
			},
		},
		{
			name: "terminated connection ends stream",
			fn: func(t *testing.T, stream proxyv1.ProxyService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&proxyv1.ProxyClusterRequest{Cluster: cluster}))

				require.NoError(t, conn.Close())
				var msg = []byte("hello")
				require.NoError(t, stream.Send(&proxyv1.ProxyClusterRequest{Frame: &proxyv1.Frame{Payload: msg}}))

				resp, err := stream.Recv()
				require.Error(t, err)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, resp)

				require.NoError(t, stream.CloseSend())
			},
		},
		{
			name: "unknown cluster",
			fn: func(t *testing.T, stream proxyv1.ProxyService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&proxyv1.ProxyClusterRequest{Cluster: uuid.NewString()}))
				resp, err := stream.Recv()
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, resp)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			conn := &echoConn{
				data: make(chan struct{}, 1),
			}

			srv := newServer(t, ServerConfig{
				Dialer: fakeDialer{
					siteConns: map[string]net.Conn{
						cluster: conn,
					},
				},
			})

			stream, err := srv.Client.ProxyCluster(context.Background())
			require.NoError(t, err)

			test.fn(t, stream, conn)
		})
	}
}
