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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/utils/grpc/stream"
)

type fakeGetClusterDetails func(context.Context, *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error)

type fakeProxySSHServer func(transportv1pb.TransportService_ProxySSHServer) error

type fakeProxyClusterServer func(transportv1pb.TransportService_ProxyClusterServer) error

// fakeTransportService is a [transportv1pb.TransportServiceServer] implementation
// that allows tests to manipulate the server side of various RPCs.
type fakeTransportService struct {
	transportv1pb.UnimplementedTransportServiceServer

	details fakeGetClusterDetails
	ssh     fakeProxySSHServer
	cluster fakeProxyClusterServer
}

func (s fakeTransportService) GetClusterDetails(ctx context.Context, req *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
	if s.details == nil {
		return s.UnimplementedTransportServiceServer.GetClusterDetails(ctx, req)
	}
	return s.details(ctx, req)
}

func (s fakeTransportService) ProxySSH(stream transportv1pb.TransportService_ProxySSHServer) error {
	if s.ssh == nil {
		return s.UnimplementedTransportServiceServer.ProxySSH(stream)
	}
	return s.ssh(stream)
}

func (s fakeTransportService) ProxyCluster(stream transportv1pb.TransportService_ProxyClusterServer) error {
	if s.cluster == nil {
		return s.UnimplementedTransportServiceServer.ProxyCluster(stream)
	}
	return s.cluster(stream)
}

// newGRPCServer creates a [grpc.Server] and registers the
// provided [transportv1pb.TransportServiceServer].
func newGRPCServer(t *testing.T, srv transportv1pb.TransportServiceServer) *fakeGRPCServer {
	// gRPC testPack.
	lis := bufconn.Listen(100)
	t.Cleanup(func() { require.NoError(t, lis.Close()) })

	s := grpc.NewServer()
	t.Cleanup(s.Stop)

	// Register service.
	if srv != nil {
		transportv1pb.RegisterTransportServiceServer(s, srv)
	}

	// Start.
	go func() {
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			panic(fmt.Sprintf("Serve returned err = %v", err))
		}
	}()

	return &fakeGRPCServer{Listener: lis}
}

type fakeGRPCServer struct {
	*bufconn.Listener
}

type fakeAuthServer struct {
	proto.UnimplementedAuthServiceServer
	listener net.Listener
	srv      *grpc.Server
}

func newFakeAuthServer(t *testing.T, conn net.Conn) *fakeAuthServer {
	f := &fakeAuthServer{
		listener: newOneShotListener(conn),
		srv:      grpc.NewServer(),
	}

	t.Cleanup(f.Stop)
	proto.RegisterAuthServiceServer(f.srv, f)
	return f
}

func (f *fakeAuthServer) Ping(context.Context, *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{
		ClusterName:   "test",
		ServerVersion: "1.0.0",
		IsBoring:      true,
	}, nil
}

func (f *fakeAuthServer) Serve() error {
	return f.srv.Serve(f.listener)
}

func (f *fakeAuthServer) Stop() {
	_ = f.listener.Close()
	f.srv.Stop()
}

type oneShotListener struct {
	conn       net.Conn
	closedCh   chan struct{}
	listenedCh chan struct{}
}

func newOneShotListener(conn net.Conn) oneShotListener {
	return oneShotListener{
		conn:       conn,
		closedCh:   make(chan struct{}),
		listenedCh: make(chan struct{}),
	}
}

func (l oneShotListener) Accept() (net.Conn, error) {
	select {
	case <-l.listenedCh:
		<-l.closedCh
		return nil, net.ErrClosed
	default:
		close(l.listenedCh)
		return l.conn, nil
	}
}

func (l oneShotListener) Close() error {
	select {
	case <-l.closedCh:
	default:
		close(l.closedCh)
	}

	return nil
}

func (l oneShotListener) Addr() net.Addr {
	return addr("127.0.0.1")
}

// addr is a [net.Addr] implementation for static tcp addresses.
type addr string

func (a addr) Network() string {
	return "tcp"
}

func (a addr) String() string {
	return string(a)
}

type fakeProxy struct {
	*fakeGRPCServer
}

func newFakeProxy(t *testing.T, transportService transportv1pb.TransportServiceServer) *fakeProxy {
	grpcSrv := newGRPCServer(t, transportService)

	return &fakeProxy{
		fakeGRPCServer: grpcSrv,
	}
}

func (f *fakeProxy) clientConfig(t *testing.T) ClientConfig {
	return ClientConfig{
		ProxyAddress: "127.0.0.1",
		SSHConfig:    &ssh.ClientConfig{},
		DialOpts: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return f.fakeGRPCServer.DialContext(ctx)
		})},
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name      string
		srv       transportv1pb.TransportServiceServer
		assertion func(t *testing.T, clt *Client, err error)
	}{
		{
			name: "does not implement transport",
			assertion: func(t *testing.T, clt *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, clt)

				details, err := clt.transport.ClusterDetails(context.Background())
				require.Error(t, err)
				require.Nil(t, details)
			},
		},
		{
			name: "compliant grpc server",
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				},
			},
			assertion: func(t *testing.T, clt *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, clt)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.srv)
			cfg := proxy.clientConfig(t)

			clt, err := NewClient(ctx, cfg)
			if clt != nil {
				t.Cleanup(func() { require.NoError(t, clt.Close()) })
			}
			test.assertion(t, clt, err)
		})
	}
}

func TestClient_ClusterDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name      string
		srv       transportv1pb.TransportServiceServer
		assertion func(t *testing.T, details ClusterDetails, err error)
	}{
		{
			name: "cluster details",
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: false}}, nil
				},
			},
			assertion: func(t *testing.T, details ClusterDetails, err error) {
				require.NoError(t, err)
				require.False(t, details.FIPS)
			},
		},
		{
			name: "cluster details fails",
			srv: fakeTransportService{
				details: func() func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
						return nil, trace.ConnectionProblem(nil, "connection closed")
					}
				}(),
			},
			assertion: func(t *testing.T, details ClusterDetails, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.srv)
			cfg := proxy.clientConfig(t)

			cfg.DialOpts = append(cfg.DialOpts, grpc.WithDisableRetry())

			clt, err := NewClient(ctx, cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, clt.Close()) })

			details, err := clt.ClusterDetails(ctx)
			test.assertion(t, details, err)
		})
	}
}

func TestClient_DialHost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name      string
		srv       transportv1pb.TransportServiceServer
		keyring   agent.ExtendedAgent
		assertion func(t *testing.T, conn net.Conn, details ClusterDetails, err error)
	}{
		{
			name: "grpc connection fails",
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				},
				ssh: func(server transportv1pb.TransportService_ProxySSHServer) error {
					_, err := server.Recv()
					if err != nil {
						return trail.ToGRPC(trace.Wrap(err))
					}

					return trail.ToGRPC(trace.ConnectionProblem(nil, "connection closed"))
				},
			},
			assertion: func(t *testing.T, conn net.Conn, details ClusterDetails, err error) {
				require.ErrorIs(t, err, trace.ConnectionProblem(nil, "connection closed"))
				require.Nil(t, conn)
				require.False(t, details.FIPS)
			},
		},
		{
			name: "grpc connection established",
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				},
				ssh: func(server transportv1pb.TransportService_ProxySSHServer) error {
					_, err := server.Recv()
					if err != nil {
						return trail.ToGRPC(trace.Wrap(err))
					}

					if err := server.Send(&transportv1pb.ProxySSHResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}); err != nil {
						return trail.ToGRPC(err)
					}

					req, err := server.Recv()
					if err != nil {
						return trail.ToGRPC(trace.Wrap(err))
					}

					switch f := req.Frame.(type) {
					case *transportv1pb.ProxySSHRequest_Ssh:
						if err := server.Send(&transportv1pb.ProxySSHResponse{
							Details: nil,
							Frame:   &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: f.Ssh.Payload}},
						}); err != nil {
							return trail.ToGRPC(trace.Wrap(err))
						}
					default:
						return trace.BadParameter("unexpected frame type received")
					}

					return nil
				},
			},
			assertion: func(t *testing.T, conn net.Conn, details ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
				require.True(t, details.FIPS)

				// test that the server echos data back over the connection
				msg := []byte("hello123")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Len(t, msg, n)

				out := make([]byte, len(msg))
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Len(t, msg, n)
				require.Equal(t, msg, out)

				require.NoError(t, conn.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.srv)
			cfg := proxy.clientConfig(t)

			clt, err := NewClient(ctx, cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, clt.Close()) })

			conn, details, err := clt.DialHost(ctx, "test", "cluster", test.keyring)
			test.assertion(t, conn, details, err)
		})
	}
}

func TestClient_DialCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name      string
		authCfg   func(config *client.Config)
		srv       transportv1pb.TransportServiceServer
		keyring   agent.ExtendedAgent
		assertion func(t *testing.T, clt *client.Client, err error)
	}{
		{
			name: "grpc connection fails",
			authCfg: func(config *client.Config) {
				config.DialTimeout = 500 * time.Millisecond // speed up dial failure
			},
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				},
				cluster: func(server transportv1pb.TransportService_ProxyClusterServer) error {
					_, err := server.Recv()
					if err != nil {
						return trace.Wrap(err)
					}

					return trace.ConnectionProblem(nil, "connection closed")
				},
			},
			assertion: func(t *testing.T, clt *client.Client, err error) {
				require.Error(t, err)
				require.Nil(t, clt)
			},
		},
		{
			name:    "grpc connection established",
			authCfg: func(config *client.Config) {},
			srv: fakeTransportService{
				details: func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				},
				cluster: func(server transportv1pb.TransportService_ProxyClusterServer) error {
					_, err := server.Recv()
					if err != nil {
						return trace.Wrap(err)
					}

					rw, err := stream.NewReadWriter(clusterStream{stream: server})
					if err != nil {
						return trace.Wrap(err)
					}

					auth := newFakeAuthServer(t, stream.NewConn(rw, nil, nil))
					err = auth.Serve()
					return trace.Wrap(err)
				},
			},
			assertion: func(t *testing.T, clt *client.Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, clt)

				expected := &proto.PingResponse{
					ClusterName:   "test",
					ServerVersion: "1.0.0",
					IsBoring:      true,
				}

				resp, err := clt.Ping(ctx)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expected, resp, protocmp.Transform()))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.srv)
			cfg := proxy.clientConfig(t)

			clt, err := NewClient(ctx, cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, clt.Close()) })

			authCfg, err := clt.ClientConfig(ctx, "cluster")
			require.NoError(t, err)

			authCfg.DialOpts = []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithReturnConnectionError(),
				grpc.WithDisableRetry(),
				grpc.FailOnNonTempDialError(true),
			}
			authCfg.Credentials = []client.Credentials{insecureCredentials{}}
			authCfg.DialTimeout = 3 * time.Second

			test.authCfg(&authCfg)

			authClt, err := client.New(ctx, authCfg)
			if authClt != nil {
				t.Cleanup(func() {
					require.NoError(t, authClt.Close())
				})
			}
			test.assertion(t, authClt, err)
		})
	}
}

// clusterStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxyClusterServer].
type clusterStream struct {
	stream transportv1pb.TransportService_ProxyClusterServer
}

func (c clusterStream) Recv() ([]byte, error) {
	req, err := c.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Frame == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return req.Frame.Payload, nil
}

func (c clusterStream) Send(frame []byte) error {
	return trace.Wrap(c.stream.Send(&transportv1pb.ProxyClusterResponse{Frame: &transportv1pb.Frame{Payload: frame}}))
}

func TestClient_SSHConfig(t *testing.T) {
	t.Parallel()

	proxy := newFakeProxy(t, fakeTransportService{})
	cfg := proxy.clientConfig(t)

	clt, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	const user = "test-user"
	sshConfig := clt.SSHConfig(user)
	require.NotNil(t, sshConfig)
	require.Equal(t, user, sshConfig.User)
	require.Empty(t, cmp.Diff(cfg.SSHConfig, sshConfig, cmpopts.IgnoreFields(ssh.ClientConfig{}, "User", "Auth", "HostKeyCallback")))
}

type fakeTransportCredentials struct {
	credentials.TransportCredentials
	info credentials.AuthInfo
	err  error
}

type fakeAuthInfo struct{}

func (f fakeAuthInfo) AuthType() string {
	return "test"
}

func (t fakeTransportCredentials) ClientHandshake(ctx context.Context, addr string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, t.info, t.err
}

func TestClusterCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		expectedClusterName string
		credentials         fakeTransportCredentials
		errAssertion        require.ErrorAssertionFunc
	}{
		{
			name:         "handshake error",
			credentials:  fakeTransportCredentials{err: context.Canceled},
			errAssertion: require.Error,
		},
		{
			name:         "no tls auth info",
			credentials:  fakeTransportCredentials{info: fakeAuthInfo{}},
			errAssertion: require.NoError,
		},
		{
			name:         "no server cert",
			credentials:  fakeTransportCredentials{info: credentials.TLSInfo{}},
			errAssertion: require.NoError,
		},
		{
			name: "no cluster oid set",
			credentials: fakeTransportCredentials{info: credentials.TLSInfo{
				State: tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								Names: []pkix.AttributeTypeAndValue{
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 0, 1},
									},
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 2, 1},
									},
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 0, 2},
									},
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 2, 2},
									},
								},
							},
						},
					},
				},
			}},
			errAssertion: require.NoError,
		}, {
			name:                "cluster name presented",
			expectedClusterName: "test-cluster",
			credentials: fakeTransportCredentials{info: credentials.TLSInfo{
				State: tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								Names: []pkix.AttributeTypeAndValue{
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 2, 1},
									},
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 0, 2},
									},
									{
										Type: asn1.ObjectIdentifier{1, 3, 9999, 2, 2},
									},
									{
										Type:  teleportClusterASN1ExtensionOID,
										Value: "test-cluster",
									},
								},
							},
						},
					},
				},
			}},
			errAssertion: require.NoError,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			c := &clusterName{}
			creds := clusterCredentials{TransportCredentials: test.credentials, clusterName: c}
			_, _, err := creds.ClientHandshake(context.Background(), "127.0.0.1", nil)
			test.errAssertion(t, err)
			require.Equal(t, test.expectedClusterName, c.get())
		})
	}
}

func TestNewDialerForGRPCClient(t *testing.T) {
	t.Run("Check that PROXYHeaderGetter if present sends PROXY header as first bytes on the connection", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, listener.Close())
		})

		prefix := []byte("FAKEPROXY")
		proxyHeaderGetter := func() ([]byte, error) {
			return prefix, nil
		}

		ctx := context.Background()
		cfg := &ClientConfig{
			PROXYHeaderGetter: proxyHeaderGetter,
		}
		dialer := newDialerForGRPCClient(ctx, cfg)

		resultChan := make(chan bool)
		// Start listening, emulating receiving end of connection
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				assert.Fail(t, err.Error())
				return
			}

			buf := make([]byte, len(prefix))
			_, err = conn.Read(buf)
			assert.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, conn.Close())
			})

			// On the received connection first bytes should be our PROXY prefix
			resultChan <- slices.Equal(buf, prefix)
		}()

		conn, err := dialer(ctx, listener.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, conn.Close())
		})

		select {
		case res := <-resultChan:
			require.True(t, res, "Didn't receive required prefix as first bytes on the connection")
		case <-time.After(time.Second):
			require.Fail(t, "Timed out waiting for connection")
		}
	})
}
