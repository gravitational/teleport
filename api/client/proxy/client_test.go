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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
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
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/api/utils/sshutils"
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

type fakeSSHServer struct {
	listener net.Listener
	cfg      fakeSSHServerConfig
}

func (s *fakeSSHServer) run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go func() {
			sconn, chans, reqs, err := ssh.NewServerConn(conn, s.cfg.config)
			if err != nil {
				return
			}
			s.cfg.handler(sconn, chans, reqs)
		}()
	}
}

func (s *fakeSSHServer) Stop() error {
	return s.listener.Close()
}

func generateSigner(t *testing.T) ssh.Signer {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	}

	privatePEM := pem.EncodeToMemory(block)
	signer, err := ssh.ParsePrivateKey(privatePEM)
	require.NoError(t, err)
	return signer
}

func (s *fakeSSHServer) clientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.cfg.cSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.cfg.hSigner.PublicKey()),
	}
}

func (s *fakeSSHServer) newClientConn() (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sconn, nc, r, err := ssh.NewClientConn(conn, "", s.clientConfig())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return sconn, nc, r, nil
}

type sshHandler func(*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request)

type fakeSSHServerConfig struct {
	config  *ssh.ServerConfig
	handler sshHandler
	cSigner ssh.Signer
	hSigner ssh.Signer
}

func discardHandler(conn *ssh.ServerConn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	defer func() { _ = conn.Close() }()

	go ssh.DiscardRequests(reqs)

	for ch := range chans {
		_ = ch.Reject(ssh.Prohibited, "discard")
	}
}

func proxySubsystemHandler(details sshDetails, handleConn func(conn *ssh.ServerConn, ch ssh.Channel)) sshHandler {
	return func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
		defer func() { _ = conn.Close() }()

		go func() {
			for req := range requests {
				if req.Type == clusterDetailsRequest {
					_ = req.Reply(true, ssh.Marshal(details))
				}
			}
		}()

		for nch := range channels {
			if nch.ChannelType() != "session" {
				_ = nch.Reject(ssh.UnknownChannelType, "unknown channel")
				continue
			}

			ch, reqs, err := nch.Accept()
			if err != nil {
				return
			}

			go func() {
				defer func() { _ = ch.Close() }()

				for req := range reqs {
					ok := req.Type == "subsystem"

					if req.WantReply {
						_ = req.Reply(ok, nil)
					}

					if !ok {
						continue
					}

					handleConn(conn, ch)
				}
			}()
		}
	}
}

func echoHandler(details sshDetails) sshHandler {
	return proxySubsystemHandler(details, func(conn *ssh.ServerConn, ch ssh.Channel) {
		_, _ = io.Copy(ch, ch)
	})
}

func authHandler(t *testing.T) sshHandler {
	return proxySubsystemHandler(sshDetails{}, func(conn *ssh.ServerConn, ch ssh.Channel) {
		auth := newFakeAuthServer(t, sshutils.NewChConn(conn, ch))
		t.Cleanup(auth.Stop)
		_ = auth.Serve()
	})
}

type fakeAuthServer struct {
	*proto.UnimplementedAuthServiceServer
	listener net.Listener
	srv      *grpc.Server
}

func newFakeAuthServer(t *testing.T, conn net.Conn) *fakeAuthServer {
	f := &fakeAuthServer{
		listener:                       newOneShotListener(conn),
		UnimplementedAuthServiceServer: &proto.UnimplementedAuthServiceServer{},
		srv:                            grpc.NewServer(),
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

func newSSHServer(t *testing.T, cfg fakeSSHServerConfig) *fakeSSHServer {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	srv := &fakeSSHServer{
		listener: listener,
		cfg:      cfg,
	}

	go srv.run()

	t.Cleanup(func() { require.NoError(t, srv.Stop()) })
	return srv
}

type fakeProxy struct {
	*fakeGRPCServer
	*fakeSSHServer
}

func newFakeProxy(t *testing.T, sshHandler sshHandler, transportService transportv1pb.TransportServiceServer) *fakeProxy {
	cSigner := generateSigner(t)
	hSigner := generateSigner(t)

	sshConfig := &ssh.ServerConfig{
		NoClientAuth:  true,
		ServerVersion: "SSH-2.0-Teleport",
	}
	sshConfig.AddHostKey(hSigner)

	sshSrv := newSSHServer(t, fakeSSHServerConfig{
		config:  sshConfig,
		handler: sshHandler,
		cSigner: cSigner,
		hSigner: hSigner,
	})
	grpcSrv := newGRPCServer(t, transportService)

	return &fakeProxy{
		fakeGRPCServer: grpcSrv,
		fakeSSHServer:  sshSrv,
	}
}

func (f *fakeProxy) clientConfig(t *testing.T) ClientConfig {
	return ClientConfig{
		ProxyAddress: "127.0.0.1",
		SSHDialer: SSHDialerFunc(func(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error) {
			conn, chans, reqs, err := f.fakeSSHServer.newClientConn()
			if err != nil {
				return nil, err
			}

			clt := &tracessh.Client{Client: ssh.NewClient(conn, chans, reqs)}
			t.Cleanup(func() { _ = clt.Close() })
			return clt, err
		}),
		SSHConfig: f.fakeSSHServer.clientConfig(),
		DialOpts: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return f.fakeGRPCServer.DialContext(ctx)
		})},
	}
}

func withoutSSH(cfg *ClientConfig) {
	cfg.SSHDialer = SSHDialerFunc(func(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error) {
		return nil, trace.NotImplemented("not implemented")
	})
}

func withoutGRPC(cfg *ClientConfig) {
	cfg.DialOpts = []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return nil, trace.NotImplemented("not implemented")
	})}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name       string
		cfg        func(cfg *ClientConfig)
		srv        transportv1pb.TransportServiceServer
		sshHandler sshHandler
		assertion  func(t *testing.T, clt *Client, err error)
	}{
		{
			name: "no ssh server and grpc that does not implement transport",
			cfg:  withoutSSH,
			assertion: func(t *testing.T, clt *Client, err error) {
				require.Error(t, err)
				require.Nil(t, clt)
			},
		},
		{
			name: "no ssh server and grpc that fails to retrieve cluster details",
			cfg:  withoutSSH,
			srv:  fakeTransportService{},
			assertion: func(t *testing.T, clt *Client, err error) {
				require.Error(t, err)
				require.Nil(t, clt)
			},
		},
		{
			name: "no ssh server but compliant grpc server",
			cfg:  withoutSSH,
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
		{
			name:       "no grpc server and ssh server",
			cfg:        withoutGRPC,
			sshHandler: discardHandler,
			assertion: func(t *testing.T, clt *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, clt)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.sshHandler, test.srv)
			cfg := proxy.clientConfig(t)
			test.cfg(&cfg)

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
		name       string
		cfg        func(*ClientConfig)
		srv        transportv1pb.TransportServiceServer
		sshHandler sshHandler
		assertion  func(t *testing.T, details ClusterDetails, err error)
	}{
		{
			name: "cluster details via ssh",
			cfg:  withoutGRPC,
			sshHandler: echoHandler(sshDetails{
				RecordingProxy: true,
				FIPSEnabled:    true,
			}),
			assertion: func(t *testing.T, details ClusterDetails, err error) {
				require.NoError(t, err)
				require.True(t, details.FIPS)
			},
		},
		{
			name:       "cluster details via ssh fails",
			cfg:        withoutGRPC,
			sshHandler: discardHandler,
			assertion: func(t *testing.T, details ClusterDetails, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "cluster details via grpc",
			cfg:  withoutSSH,
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
			name: "cluster details via grpc fails",
			cfg:  withoutSSH,
			srv: fakeTransportService{
				details: func() func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
					// NewClient will try to get cluster details to validate that the gRPC
					// connection is established, so we need to send details on the first request.
					// The second request will be from the test case that is expecting an error.
					sent := &atomic.Bool{}
					return func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
						if sent.Load() {
							return nil, trace.ConnectionProblem(nil, "connection closed")
						}

						sent.Store(true)
						return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: false}}, nil
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
			proxy := newFakeProxy(t, test.sshHandler, test.srv)
			cfg := proxy.clientConfig(t)
			test.cfg(&cfg)

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
		name       string
		cfg        func(*ClientConfig)
		srv        transportv1pb.TransportServiceServer
		sshHandler sshHandler
		keyring    agent.ExtendedAgent
		assertion  func(t *testing.T, conn net.Conn, details ClusterDetails, err error)
	}{
		{
			name:       "ssh connection fails",
			cfg:        withoutGRPC,
			sshHandler: discardHandler,
			assertion: func(t *testing.T, conn net.Conn, details ClusterDetails, err error) {
				require.Error(t, err)
				require.Nil(t, conn)
				require.False(t, details.FIPS)
			},
		},
		{
			name:       "ssh connection established",
			cfg:        withoutGRPC,
			sshHandler: echoHandler(sshDetails{RecordingProxy: false, FIPSEnabled: true}),
			assertion: func(t *testing.T, conn net.Conn, details ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
				require.True(t, details.FIPS)

				// test that the server echos data back over the connection
				msg := []byte("hello123")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Equal(t, len(msg), n)

				out := make([]byte, len(msg))
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Equal(t, len(msg), n)
				require.Equal(t, msg, out)

				require.NoError(t, conn.Close())
			},
		},
		{
			name: "grpc connection fails",
			cfg:  withoutSSH,
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
			cfg:  withoutSSH,
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
				require.Equal(t, len(msg), n)

				out := make([]byte, len(msg))
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Equal(t, len(msg), n)
				require.Equal(t, msg, out)

				require.NoError(t, conn.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxy := newFakeProxy(t, test.sshHandler, test.srv)
			cfg := proxy.clientConfig(t)
			test.cfg(&cfg)

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
		name       string
		cfg        func(*ClientConfig)
		authCfg    func(config *client.Config)
		srv        transportv1pb.TransportServiceServer
		sshHandler sshHandler
		keyring    agent.ExtendedAgent
		assertion  func(t *testing.T, clt *client.Client, err error)
	}{
		{
			name: "ssh connection fails",
			cfg:  withoutGRPC,
			authCfg: func(config *client.Config) {
				config.DialTimeout = 500 * time.Millisecond // speed up dial failure
			},
			sshHandler: discardHandler,
			assertion: func(t *testing.T, clt *client.Client, err error) {
				require.Error(t, err)
				require.Nil(t, clt)
			},
		},
		{
			name:       "ssh connection established",
			cfg:        withoutGRPC,
			authCfg:    func(config *client.Config) {},
			sshHandler: authHandler(t),
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
		{
			name: "grpc connection fails",
			cfg:  withoutSSH,
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
			cfg:     withoutSSH,
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
			proxy := newFakeProxy(t, test.sshHandler, test.srv)
			cfg := proxy.clientConfig(t)
			test.cfg(&cfg)

			clt, err := NewClient(ctx, cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, clt.Close()) })

			authCfg := clt.ClientConfig(ctx, "cluster")
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

	proxy := newFakeProxy(t, discardHandler, fakeTransportService{})
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

type fakePublicKey struct{}

func (f fakePublicKey) Type() string {
	return "test"
}

func (f fakePublicKey) Marshal() []byte {
	return nil
}

func (f fakePublicKey) Verify(data []byte, sig *ssh.Signature) error {
	return trace.NotImplemented("")
}

func TestClusterCallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		hostKeyCB           ssh.HostKeyCallback
		publicKey           ssh.PublicKey
		expectedClusterName string
		errAssertion        require.ErrorAssertionFunc
	}{
		{
			name: "handshake failure",
			hostKeyCB: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return context.Canceled
			},
			errAssertion: require.Error,
		},
		{
			name:      "invalid certificate",
			publicKey: fakePublicKey{},
			hostKeyCB: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			errAssertion: require.NoError,
		},
		{
			name: "no authority present",
			publicKey: &ssh.Certificate{
				Permissions: ssh.Permissions{
					Extensions: map[string]string{},
				},
			},
			hostKeyCB: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			errAssertion: require.NoError,
		},

		{
			name:                "cluster name presented",
			expectedClusterName: "test-cluster",
			publicKey: &ssh.Certificate{
				Permissions: ssh.Permissions{
					Extensions: map[string]string{
						teleportAuthority: "test-cluster",
					},
				},
			},
			hostKeyCB: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			errAssertion: require.NoError,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			c := &clusterName{}
			err := clusterCallback(c, test.hostKeyCB)("test", addr("127.0.0.1"), test.publicKey)
			test.errAssertion(t, err)
			require.Equal(t, test.expectedClusterName, c.get())

		})
	}
}
