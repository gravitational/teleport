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

package transportv1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
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

	pr *io.PipeReader
	pw *io.PipeWriter
}

func newEchoConn() *echoConn {
	pr, pw := io.Pipe()

	return &echoConn{
		pr: pr,
		pw: pw,
	}
}

func (c *echoConn) Write(p []byte) (int, error) {
	n, err := c.pw.Write(p)
	if errors.Is(err, io.ErrClosedPipe) {
		return n, io.EOF
	}

	return n, err
}

func (c *echoConn) Read(p []byte) (int, error) {
	n, err := c.pr.Read(p)
	if errors.Is(err, io.ErrClosedPipe) {
		return n, io.EOF
	}

	return n, err
}

func (c *echoConn) Close() error {
	return trace.NewAggregate(c.pr.Close(), c.pw.Close())
}

// fakeDialer implements [Dialer] with a static map of
// site and host to connections.
type fakeDialer struct {
	siteConns map[string]net.Conn
	hostConns map[string]net.Conn
}

func (f fakeDialer) DialSite(ctx context.Context, clusterName string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	conn, ok := f.siteConns[clusterName]
	if !ok {
		return nil, trace.NotFound("%s", clusterName)
	}

	return conn, nil
}

func (f fakeDialer) DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, cluster, loginName string, checker services.AccessChecker, agentGetter teleagent.Getter, singer agentless.SignerCreator) (_ net.Conn, err error) {
	key := fmt.Sprintf("%s.%s.%s", host, port, cluster)
	conn, ok := f.hostConns[key]
	if !ok {
		return nil, trace.NotFound("%s", key)
	}

	return conn, nil
}

// testPack used to test a [Service].
type testPack struct {
	Client transportv1pb.TransportServiceClient
	Server *Service
}

type listenerWithAddr struct {
	*bufconn.Listener
	localAddr net.Addr
}

func (l *listenerWithAddr) Addr() net.Addr {
	return l.localAddr
}

func (l *listenerWithAddr) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	return &connWithAddr{
		Conn: conn,
		addr: l.localAddr,
	}, err
}

type connWithAddr struct {
	net.Conn
	addr net.Addr
}

func (c *connWithAddr) RemoteAddr() net.Addr {
	return c.addr
}

func (c *connWithAddr) LocalAddr() net.Addr {
	return c.addr
}

// newServer creates a [Service] with the provided config and
// an authenticated client to exercise various RPCs on the [Service].
func newServer(t *testing.T, cfg ServerConfig) testPack {
	// gRPC testPack.
	const bufSize = 100 // arbitrary
	var lisWithAddr net.Listener
	lis := bufconn.Listen(bufSize)
	lisWithAddr = &listenerWithAddr{
		Listener:  lis,
		localAddr: utils.MustParseAddr("127.0.0.1:4242"),
	}
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})

	s := grpc.NewServer(
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	t.Cleanup(func() {
		s.GracefulStop()
		s.Stop()
	})

	srv, err := NewService(cfg)
	require.NoError(t, err)

	// Register service.
	transportv1pb.RegisterTransportServiceServer(s, srv)

	// Start.
	go func() {
		if err := s.Serve(lisWithAddr); err != nil {
			panic(fmt.Sprintf("Serve returned err = %v", err))
		}
	}()

	// gRPC client.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, "unused",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			conn, err := lis.DialContext(ctx)
			return &connWithAddr{
				Conn: conn,
				addr: utils.MustParseAddr("127.0.0.1:8484"),
			}, err
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return testPack{
		Client: transportv1pb.NewTransportServiceClient(cc),
		Server: srv,
	}
}

func fakeSigner(authzCtx *authz.Context, clusterName string) agentless.SignerCreator {
	return func(_ context.Context, _ agentless.LocalAccessPoint, _ agentless.CertGenerator) (ssh.Signer, error) {
		return nil, nil
	}
}

type fakeMonitor struct{}

func (f fakeMonitor) MonitorConn(ctx context.Context, authCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error) {
	return ctx, conn, nil
}

// TestService_GetClusterDetails validates that a [Service] returns
// the expected cluster details.
func TestService_GetClusterDetails(t *testing.T) {
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
				Dialer:            fakeDialer{},
				Logger:            utils.NewSlogLoggerForTests(),
				FIPS:              test.FIPS,
				SignerFn:          fakeSigner,
				ConnectionMonitor: fakeMonitor{},
				LocalAddr:         utils.MustParseAddr("127.0.0.1:4242"),
			})

			resp, err := srv.Client.GetClusterDetails(context.Background(), &transportv1pb.GetClusterDetailsRequest{})
			require.NoError(t, err)
			require.Equal(t, test.FIPS, resp.Details.FipsEnabled)
		})
	}
}

// TestService_ProxyCluster validates that a [Service] proxies data to
// and from a target cluster.
func TestService_ProxyCluster(t *testing.T) {
	t.Parallel()
	const cluster = "test"

	tests := []struct {
		name string
		fn   func(t *testing.T, stream transportv1pb.TransportService_ProxyClusterClient, conn *echoConn)
	}{
		{
			name: "transport established to cluster",
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxyClusterRequest{Cluster: cluster}))

				msg := []byte("hello")
				require.NoError(t, stream.Send(&transportv1pb.ProxyClusterRequest{Frame: &transportv1pb.Frame{Payload: msg}}))

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
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxyClusterRequest{Cluster: cluster}))

				require.NoError(t, conn.Close())
				msg := []byte("hello")
				require.NoError(t, stream.Send(&transportv1pb.ProxyClusterRequest{Frame: &transportv1pb.Frame{Payload: msg}}))

				resp, err := stream.Recv()
				require.Error(t, err)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, resp)

				require.NoError(t, stream.CloseSend())
			},
		},
		{
			name: "unknown cluster",
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxyClusterClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxyClusterRequest{Cluster: uuid.NewString()}))
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

			conn := newEchoConn()

			srv := newServer(t, ServerConfig{
				Dialer: fakeDialer{
					siteConns: map[string]net.Conn{
						cluster: conn,
					},
				},
				Logger:            utils.NewSlogLoggerForTests(),
				SignerFn:          fakeSigner,
				ConnectionMonitor: fakeMonitor{},
				LocalAddr:         utils.MustParseAddr("127.0.0.1:4242"),
			})

			stream, err := srv.Client.ProxyCluster(context.Background())
			require.NoError(t, err)

			test.fn(t, stream, conn)
		})
	}
}

type fakeChecker struct {
	services.AccessChecker
}

// TestService_ProxySSH_Errors validates that a [Service] terminates a
// ProxySSH stream if various error conditions occur
func TestService_ProxySSH_Errors(t *testing.T) {
	t.Parallel()
	const fakeHost = "test.0.test"

	tests := []struct {
		name      string
		checkerFn func(info credentials.AuthInfo) (services.AccessChecker, error)
		fn        func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn)
	}{
		{
			name: "missing dial target terminates stream",
			checkerFn: func(info credentials.AuthInfo) (services.AccessChecker, error) {
				return fakeChecker{}, nil
			},
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{}))

				resp, err := stream.Recv()
				require.True(t, trace.IsBadParameter(err))
				require.Nil(t, resp)
			},
		},
		{
			name: "invalid hostpost terminates stream",
			checkerFn: func(info credentials.AuthInfo) (services.AccessChecker, error) {
				return fakeChecker{}, nil
			},
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{DialTarget: &transportv1pb.TargetHost{
					HostPort: "1234",
					Cluster:  "test",
				}}))

				resp, err := stream.Recv()
				require.True(t, trace.IsBadParameter(err))
				require.Nil(t, resp)
			},
		},
		{
			name: "no access checker terminates stream",
			checkerFn: func(info credentials.AuthInfo) (services.AccessChecker, error) {
				return nil, trace.AccessDenied("no access checker")
			},
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn) {
				err := stream.Send(&transportv1pb.ProxySSHRequest{DialTarget: &transportv1pb.TargetHost{
					HostPort: "1234",
					Cluster:  "test",
				}})
				switch {
				// The server will attempt to get the authz context prior to receiving the first
				// message from the client which may terminate the stream and result in an EOF.
				case errors.Is(err, io.EOF):
					return
				default:
					require.NoError(t, err)
				}

				resp, err := stream.Recv()
				require.Nil(t, resp)
				switch {
				// The server will attempt to get the authz context prior to receiving the first
				// message from the client which may terminate the stream and result in an EOF.
				case errors.Is(err, io.EOF):
				// The client send may be completed prior to the server getting the authz context
				// which will result in the client actually receiving the error from getting the
				// authz context instead of an EOF.
				case errors.Is(err, trace.AccessDenied("no access checker")):
				// All other errors indicate that something went wrong
				default:
					t.Fatalf("expected either EOF or Access Denied, got %v", err)
				}
			},
		},
		{
			name: "terminated connection ends stream",
			checkerFn: func(info credentials.AuthInfo) (services.AccessChecker, error) {
				return fakeChecker{}, nil
			},
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{DialTarget: &transportv1pb.TargetHost{
					HostPort: "test:0",
					Cluster:  "test",
				}}))

				// get cluster details
				resp, err := stream.Recv()
				require.NoError(t, err)
				require.NotNil(t, resp.Details)
				require.Nil(t, resp.Frame)

				require.NoError(t, conn.Close())
				msg := []byte("hello")
				require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{Frame: &transportv1pb.ProxySSHRequest_Ssh{Ssh: &transportv1pb.Frame{Payload: msg}}}))

				resp, err = stream.Recv()
				require.Error(t, err)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, resp)

				require.NoError(t, stream.CloseSend())
			},
		},
		{
			name: "unknown host terminates stream",
			checkerFn: func(info credentials.AuthInfo) (services.AccessChecker, error) {
				return fakeChecker{}, nil
			},
			fn: func(t *testing.T, stream transportv1pb.TransportService_ProxySSHClient, conn *echoConn) {
				require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{DialTarget: &transportv1pb.TargetHost{
					HostPort: "test:100",
					Cluster:  "test",
				}}))
				resp, err := stream.Recv()
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, resp)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newEchoConn()

			srv := newServer(t, ServerConfig{
				Dialer: fakeDialer{
					hostConns: map[string]net.Conn{
						fakeHost: conn,
					},
				},
				SignerFn:          fakeSigner,
				ConnectionMonitor: fakeMonitor{},
				Logger:            utils.NewSlogLoggerForTests(),
				LocalAddr:         utils.MustParseAddr("127.0.0.1:4242"),
				authzContextFn: func(info credentials.AuthInfo) (*authz.Context, error) {
					checker, err := test.checkerFn(info)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					return &authz.Context{Checker: checker}, nil
				},
			})

			stream, err := srv.Client.ProxySSH(context.Background())
			require.NoError(t, err)

			test.fn(t, stream, conn)
		})
	}
}

// TestService_ProxySSH validates that a [Service] proxies SSH and
// SSH Agent protocol over the stream.
func TestService_ProxySSH(t *testing.T) {
	t.Parallel()

	// create an ssh server that will be dialed by ProxySSH. Global requests
	// of type "echo" will respond with the received payload. All other requests
	// and channels are rejected.
	sshSrv := newSSHServer(t, func(sconn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
		for {
			select {
			case req := <-requests:
				switch {
				case req == nil:
					return
				case req.Type != "echo":
					if !req.WantReply {
						continue
					}

					req.Reply(false, nil)
				case !req.WantReply:
					continue
				}

				req.Reply(true, req.Payload)

			case nch := <-channels:
				if nch == nil {
					return
				}

				nch.Reject(ssh.UnknownChannelType, "only test channel is supported")
			}
		}
	})
	go sshSrv.Run()

	// create a server that will open a new connection to the
	// ssh server created above on each dial request
	srv := newServer(t, ServerConfig{
		Dialer:            sshSrv,
		SignerFn:          fakeSigner,
		Logger:            utils.NewSlogLoggerForTests(),
		LocalAddr:         utils.MustParseAddr("127.0.0.1:4242"),
		ConnectionMonitor: fakeMonitor{},
		agentGetterFn: func(rw io.ReadWriter) teleagent.Getter {
			return func() (teleagent.Agent, error) {
				srw, ok := rw.(*streamutils.ReadWriter)
				if !ok {
					return nil, trace.BadParameter("rw must be a streamutils.ReadWriter")
				}

				return testAgent{
					ReadWriteCloser: srw,
				}, nil
			}
		},
		authzContextFn: func(info credentials.AuthInfo) (*authz.Context, error) {
			return &authz.Context{Checker: fakeChecker{}}, nil
		},
	})

	// create the stream
	stream, err := srv.Client.ProxySSH(context.Background())
	require.NoError(t, err)

	// send a fictitious dial target - it does not matter since
	// each connection will be to the same server. this test
	// solely cares that a connection is made and protocols are
	// multiplexed, not that we are dialing our target.
	require.NoError(t, stream.Send(&transportv1pb.ProxySSHRequest{
		DialTarget: &transportv1pb.TargetHost{
			HostPort: "test:0",
			Cluster:  "test",
		},
	}))

	// wait for the response indicating that the connection
	// was established
	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.Details)
	require.Nil(t, resp.Frame)

	// create a stream for agent protocol
	agentStream := newClientStream(stream, func(payload []byte) *transportv1pb.ProxySSHRequest {
		return &transportv1pb.ProxySSHRequest{Frame: &transportv1pb.ProxySSHRequest_Agent{Agent: &transportv1pb.Frame{Payload: payload}}}
	})

	// create a stream for ssh protocol
	sshStream := newClientStream(stream, func(payload []byte) *transportv1pb.ProxySSHRequest {
		return &transportv1pb.ProxySSHRequest{Frame: &transportv1pb.ProxySSHRequest_Ssh{Ssh: &transportv1pb.Frame{Payload: payload}}}
	})

	// multiplex the frames to the correct handlers
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}

				return
			}

			switch frame := req.Frame.(type) {
			case *transportv1pb.ProxySSHResponse_Ssh:
				sshStream.incomingC <- frame.Ssh.Payload
			case *transportv1pb.ProxySSHResponse_Agent:
				agentStream.incomingC <- frame.Agent.Payload
			default:
				continue
			}
		}
	}()

	// create a reader writer for SSH protocol
	sshRW, err := streamutils.NewReadWriter(sshStream)
	require.NoError(t, err)

	// create reader writer for SSH Agent protocol
	agentRW, err := streamutils.NewReadWriter(agentStream)
	require.NoError(t, err)

	// create a new ssh client connection over a stream conn
	addr := &utils.NetAddr{Addr: "127.0.0.1", AddrNetwork: "tcp"}
	sshconn, chans, reqs, err := ssh.NewClientConn(
		streamutils.NewConn(sshRW, addr, sshSrv.listener.Addr()),
		addr.String(),
		sshSrv.clientConfig())
	require.NoError(t, err)

	// create the ssh client
	client := ssh.NewClient(sshconn, chans, reqs)

	// send an ssh request to our server which will echo the payload
	// back in the response.
	msg := []byte("hello")
	ok, response, err := client.SendRequest("echo", true, msg)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, msg, response)

	// create an ssh agent client and list
	// the keys from the keyring used by the server
	keys, err := agent.NewClient(agentRW).List()
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestGetDestinationAddress(t *testing.T) {
	testCases := []struct {
		listenerAddr string
		srcAddr      string
		expected     string
	}{
		{
			srcAddr:      "4.3.2.1:65",
			listenerAddr: "1.2.3.4:56",
			expected:     "1.2.3.4:56",
		},
		{
			srcAddr:      "4.3.2.1:65",
			listenerAddr: "[2601:602:8700:4470:a3:813c:1d8c:30b9]:56",
			expected:     "127.0.0.1:56",
		},
		{
			srcAddr:      "[2601:602:8700:4470:a3:813c:1d8c:30b9]:65",
			listenerAddr: "1.2.3.4:56",
			expected:     "[::1]:56",
		},
		{
			srcAddr:      "4.3.2.1:65",
			listenerAddr: "0.0.0.0:56",
			expected:     "127.0.0.1:56",
		},
		{
			srcAddr:      "4.3.2.1:65",
			listenerAddr: "[::]:56",
			expected:     "127.0.0.1:56",
		},
		{
			srcAddr:      "[2601:602:8700:4470:a3:813c:1d8c:30b9]:65",
			listenerAddr: "[::]:56",
			expected:     "[::1]:56",
		},
		{
			srcAddr:      "[2601:602:8700:4470:a3:813c:1d8c:30b9]:65",
			listenerAddr: "0.0.0.0:56",
			expected:     "[::1]:56",
		},
	}

	for i, tt := range testCases {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			res, err := getDestinationAddress(utils.MustParseAddr(tt.srcAddr), utils.MustParseAddr(tt.listenerAddr))
			require.NoError(t, err)
			require.Equal(t, tt.expected, res.String())
		})
	}
}

// clientStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxySSHClient]. Instead of
// reading directly from the stream reads are from an incoming
// channel that is fed by the multiplexer.
type clientStream struct {
	incomingC  chan []byte
	stream     transportv1pb.TransportService_ProxySSHClient
	responseFn func(payload []byte) *transportv1pb.ProxySSHRequest
}

func newClientStream(stream transportv1pb.TransportService_ProxySSHClient, responseFn func(payload []byte) *transportv1pb.ProxySSHRequest) *clientStream {
	return &clientStream{
		incomingC:  make(chan []byte, 10),
		stream:     stream,
		responseFn: responseFn,
	}
}

func (s *clientStream) Recv() ([]byte, error) {
	select {
	case <-s.stream.Context().Done():
		return nil, io.EOF
	case frame := <-s.incomingC:
		return frame, nil
	}
}

func (s *clientStream) Send(frame []byte) error {
	return trace.Wrap(s.stream.Send(s.responseFn(frame)))
}

// testAgent is a marker type used by the test dialer to
// know that it should serve agent protocol on the stream.
type testAgent struct {
	io.ReadWriteCloser
	agent.ExtendedAgent
}

// sshServer a test ssh server that implements Dialer
// by creating a new client connection to itself
type sshServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	handler  func(*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request)

	cSigner ssh.Signer
	hSigner ssh.Signer
	keyring agent.Agent
}

// DialSite returns a connection to the sshServer
func (s *sshServer) DialSite(ctx context.Context, clusterName string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// DialHost returns a connection to the sshServer. If the agentGetter is not
// nil and is of type testAgent, then the server will serve its keyring
// over the underlying [streamutils.ReadWriter] so that tests can exercise
// ssh agent multiplexing.
func (s *sshServer) DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, cluster, loginName string, checker services.AccessChecker, agentGetter teleagent.Getter, singer agentless.SignerCreator) (_ net.Conn, err error) {
	conn, err := s.dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if agentGetter == nil {
		return conn, nil
	}

	agnt, err := agentGetter()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rw, ok := agnt.(testAgent)
	if !ok {
		return conn, nil
	}

	go func() {
		agent.ServeAgent(s.keyring, rw)
	}()

	return conn, nil
}

func (s *sshServer) Run() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				continue
			}
			return
		}

		go func() {
			sconn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
			if err != nil {
				return
			}
			s.handler(sconn, chans, reqs)
		}()
	}
}

func (s *sshServer) Stop() error {
	return s.listener.Close()
}

func generateSigner(t *testing.T, keyring agent.Agent) ssh.Signer {
	private, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromSigner(private)
	require.NoError(t, err)

	require.NoError(t, keyring.Add(agent.AddedKey{
		PrivateKey:   private,
		Comment:      "test",
		LifetimeSecs: math.MaxUint32,
	}))

	return signer
}

func (s *sshServer) clientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.cSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.hSigner.PublicKey()),
	}
}

func (s *sshServer) dial() (net.Conn, error) {
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	return conn, trace.Wrap(err)
}

func newSSHServer(t *testing.T, handler func(*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request)) *sshServer {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	keyring := agent.NewKeyring()
	cSigner := generateSigner(t, keyring)
	hSigner := generateSigner(t, keyring)

	config := &ssh.ServerConfig{
		NoClientAuth:  true,
		ServerVersion: "SSH-2.0-Teleport",
	}
	config.AddHostKey(hSigner)

	srv := &sshServer{
		listener: listener,
		config:   config,
		handler:  handler,
		cSigner:  cSigner,
		hSigner:  hSigner,
		keyring:  keyring,
	}

	t.Cleanup(func() { require.NoError(t, srv.Stop()) })

	return srv
}
