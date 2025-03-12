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
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
)

type fakeGetClusterDetailsServer func(context.Context, *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error)

type fakeProxySSHServer func(transportv1pb.TransportService_ProxySSHServer) error

type fakeProxyClusterServer func(transportv1pb.TransportService_ProxyClusterServer) error

// fakeServer is a [transportv1pb.TransportServiceServer] implementation
// that allows tests to manipulate the server side of various RPCs.
type fakeServer struct {
	transportv1pb.UnimplementedTransportServiceServer

	details fakeGetClusterDetailsServer
	ssh     fakeProxySSHServer
	cluster fakeProxyClusterServer
}

func (s fakeServer) GetClusterDetails(ctx context.Context, req *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
	return s.details(ctx, req)
}

func (s fakeServer) ProxySSH(stream transportv1pb.TransportService_ProxySSHServer) error {
	return s.ssh(stream)
}

func (s fakeServer) ProxyCluster(stream transportv1pb.TransportService_ProxyClusterServer) error {
	return s.cluster(stream)
}

// TestClient_ClusterDetails validates that a Client can retrieve
// [transportv1pb.ClusterDetails] from a [transportv1pb.TransportServiceServer].
func TestClient_ClusterDetails(t *testing.T) {
	t.Parallel()

	pack := newServer(t, fakeServer{
		details: func() fakeGetClusterDetailsServer {
			var i atomic.Bool
			return func(ctx context.Context, request *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
				if i.CompareAndSwap(false, true) {
					return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}, nil
				}

				return nil, trail.ToGRPC(trace.NotImplemented("not implemented"))
			}
		}(),
	})

	tests := []struct {
		name      string
		assertion func(t *testing.T, response *transportv1pb.ClusterDetails, err error)
	}{
		{
			name: "details retrieved successfully",
			assertion: func(t *testing.T, response *transportv1pb.ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.True(t, response.FipsEnabled)
			},
		},
		{
			name: "error getting details",
			assertion: func(t *testing.T, response *transportv1pb.ClusterDetails, err error) {
				require.ErrorIs(t, err, trace.NotImplemented("not implemented"))
				require.Nil(t, response)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := pack.Client.ClusterDetails(context.Background())
			test.assertion(t, resp, err)
		})
	}
}

// TestClient_DialCluster validates that a Client can establish a
// connection to a cluster and that said connection is proxied over
// the gRPC stream.
func TestClient_DialCluster(t *testing.T) {
	t.Parallel()

	pack := newServer(t, fakeServer{
		cluster: func(server transportv1pb.TransportService_ProxyClusterServer) error {
			req, err := server.Recv()
			if err != nil {
				return trail.ToGRPC(err)
			}

			switch req.Cluster {
			case "":
				return trail.ToGRPC(trace.BadParameter("first message must contain a cluster"))
			case "not-implemented":
				return trail.ToGRPC(trace.NotImplemented("not implemented"))
			case "echo":
				// get the payload written
				req, err = server.Recv()
				if err != nil {
					return trail.ToGRPC(err)
				}

				// echo the data back
				if err := server.Send(&transportv1pb.ProxyClusterResponse{Frame: &transportv1pb.Frame{Payload: req.Frame.Payload}}); err != nil {
					return trail.ToGRPC(err)
				}

				return nil
			default:
				return trace.NotFound("unknown cluster: %q", req.Cluster)
			}
		},
	})

	tests := []struct {
		name      string
		cluster   string
		assertion func(t *testing.T, conn net.Conn, err error)
	}{
		{
			name:    "stream terminated",
			cluster: "not-implemented",
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)

				n, err := conn.Read(make([]byte, 10))
				require.True(t, trace.IsConnectionProblem(err))
				require.Zero(t, n)
			},
		},
		{
			name:    "invalid cluster name",
			cluster: "unknown",
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)

				n, err := conn.Read(make([]byte, 10))
				require.True(t, trace.IsConnectionProblem(err))
				require.Zero(t, n)
			},
		},
		{
			name:    "connection successfully established",
			cluster: "echo",
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)

				msg := []byte("hello")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Len(t, msg, n)

				out := make([]byte, n)
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
			conn, err := pack.Client.DialCluster(context.Background(), test.cluster, nil)
			test.assertion(t, conn, err)
		})
	}
}

// TestClient_DialHost validates that a Client can establish a
// connection to a host and that both SSH and SSH Agent protocol is
// proxied over the gRPC stream.
func TestClient_DialHost(t *testing.T) {
	t.Parallel()

	keyring := newKeyring(t)

	pack := newServer(t, fakeServer{
		ssh: func(server transportv1pb.TransportService_ProxySSHServer) error {
			req, err := server.Recv()
			if err != nil {
				return trail.ToGRPC(err)
			}

			switch {
			case req == nil:
				return trail.ToGRPC(trace.BadParameter("first message must contain a dial target"))
			case req.DialTarget.Cluster == "":
				return trail.ToGRPC(trace.BadParameter("first message must contain a cluster"))
			case req.DialTarget.HostPort == "":
				return trail.ToGRPC(trace.BadParameter("invalid dial target"))
			case req.DialTarget.Cluster == "not-implemented":
				return trail.ToGRPC(trace.NotImplemented("not implemented"))
			case req.DialTarget.Cluster == "payload-too-large":
				// send the initial cluster details
				if err := server.Send(&transportv1pb.ProxySSHResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}); err != nil && !errors.Is(err, io.EOF) {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// wait for the first ssh frame
				req, err = server.Recv()
				if err != nil {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// write too much data to terminate the stream
				switch req.Frame.(type) {
				case *transportv1pb.ProxySSHRequest_Ssh:
					if err := server.Send(&transportv1pb.ProxySSHResponse{
						Details: nil,
						Frame:   &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: bytes.Repeat([]byte{0}, 1001)}},
					}); err != nil && !errors.Is(err, io.EOF) {
						return trail.ToGRPC(trace.Wrap(err))
					}
				case *transportv1pb.ProxySSHRequest_Agent:
					return trail.ToGRPC(trace.BadParameter("test expects first frame to be ssh. got an agent frame"))
				}

				return nil
			case req.DialTarget.Cluster == "echo":
				// send the initial cluster details
				if err := server.Send(&transportv1pb.ProxySSHResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}); err != nil && !errors.Is(err, io.EOF) {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// wait for the first ssh frame
				req, err = server.Recv()
				if err != nil {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// write too much data to terminate the stream
				switch f := req.Frame.(type) {
				case *transportv1pb.ProxySSHRequest_Ssh:
					if err := server.Send(&transportv1pb.ProxySSHResponse{
						Details: nil,
						Frame:   &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: f.Ssh.Payload}},
					}); err != nil && !errors.Is(err, io.EOF) {
						return trail.ToGRPC(trace.Wrap(err))
					}
				case *transportv1pb.ProxySSHRequest_Agent:
					return trail.ToGRPC(trace.BadParameter("test expects first frame to be ssh. got an agent frame"))
				}
				return nil
			case req.DialTarget.Cluster == "forward":
				// send the initial cluster details
				if err := server.Send(&transportv1pb.ProxySSHResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: true}}); err != nil && !errors.Is(err, io.EOF) {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// wait for the first ssh frame
				req, err = server.Recv()
				if err != nil {
					return trail.ToGRPC(trace.Wrap(err))
				}

				// echo the data back on an ssh frame
				switch f := req.Frame.(type) {
				case *transportv1pb.ProxySSHRequest_Ssh:
					if err := server.Send(&transportv1pb.ProxySSHResponse{
						Details: nil,
						Frame:   &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: f.Ssh.Payload}},
					}); err != nil && !errors.Is(err, io.EOF) {
						return trail.ToGRPC(trace.Wrap(err))
					}
				case *transportv1pb.ProxySSHRequest_Agent:
					return trail.ToGRPC(trace.BadParameter("test expects first frame to be ssh. got an agent frame"))
				}

				// create an agent stream and writer to communicate agent protocol on
				agentStream := newServerStream(server, func(payload []byte) *transportv1pb.ProxySSHResponse {
					return &transportv1pb.ProxySSHResponse{Frame: &transportv1pb.ProxySSHResponse_Agent{Agent: &transportv1pb.Frame{Payload: payload}}}
				})
				agentStreamRW, err := streamutils.NewReadWriter(agentStream)
				if err != nil {
					return trail.ToGRPC(trace.Wrap(err, "failed constructing ssh agent streamer"))
				}

				// read in agent frames
				go func() {
					for {
						req, err := server.Recv()
						if err != nil {
							if errors.Is(err, io.EOF) {
								return
							}

							return
						}

						switch frame := req.Frame.(type) {
						case *transportv1pb.ProxySSHRequest_Agent:
							agentStream.incomingC <- frame.Agent.Payload
						default:
							continue
						}
					}
				}()

				// create an agent that will communicate over the agent frames
				// and list the keys from the client
				clt := agent.NewClient(agentStreamRW)
				keys, err := clt.List()
				if err != nil {
					return trail.ToGRPC(trace.Wrap(err))
				}

				if len(keys) != 1 {
					return trail.ToGRPC(fmt.Errorf("expected to receive 1 key. got %v", len(keys)))
				}

				// send the key blob back via an ssh frame to alert the
				// test that we finished listing keys
				if err := server.Send(&transportv1pb.ProxySSHResponse{
					Details: nil,
					Frame:   &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: keys[0].Blob}},
				}); err != nil && !errors.Is(err, io.EOF) {
					return trail.ToGRPC(trace.Wrap(err))
				}
				return nil
			default:
				return trail.ToGRPC(trace.BadParameter("invalid cluster"))
			}
		},
	})

	tests := []struct {
		name      string
		cluster   string
		target    string
		keyring   agent.ExtendedAgent
		assertion func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error)
	}{
		{
			name:    "stream terminated",
			cluster: "not-implemented",
			target:  "127.0.0.1:8080",
			assertion: func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error) {
				require.ErrorIs(t, err, trace.NotImplemented("not implemented"))
				require.Nil(t, conn)
				require.Nil(t, details)
			},
		},
		{
			name:    "invalid dial target",
			cluster: "valid",
			assertion: func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error) {
				require.ErrorIs(t, err, trace.BadParameter("invalid dial target"))
				require.Nil(t, conn)
				require.Nil(t, details)
			},
		},
		{
			name:    "connection terminated when receive returns an error",
			cluster: "payload-too-large",
			target:  "127.0.0.1:8080",
			assertion: func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)

				msg := []byte("hello")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Len(t, msg, n)

				out := make([]byte, 10)
				n, err = conn.Read(out)
				require.True(t, trace.IsConnectionProblem(err))
				require.Zero(t, n)

				require.NoError(t, conn.Close())
			},
		},
		{
			name:    "connection successfully established without agent forwarding",
			cluster: "echo",
			target:  "127.0.0.1:8080",
			assertion: func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)

				msg := []byte("hello")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Len(t, msg, n)

				out := make([]byte, n)
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Len(t, msg, n)
				require.Equal(t, msg, out)

				n, err = conn.Read(out)
				require.ErrorIs(t, err, io.EOF)
				require.Zero(t, n)

				require.NoError(t, conn.Close())
			},
		},
		{
			name:    "connection successfully established with agent forwarding",
			cluster: "forward",
			target:  "127.0.0.1:8080",
			keyring: keyring,
			assertion: func(t *testing.T, conn net.Conn, details *transportv1pb.ClusterDetails, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
				require.True(t, details.FipsEnabled)

				// write data via ssh frames
				msg := []byte("hello")
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Len(t, msg, n)

				// read data via ssh frames
				out := make([]byte, n)
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Len(t, msg, n)
				require.Equal(t, msg, out)

				// get the keys from our local keyring
				keys, err := keyring.List()
				require.NoError(t, err)
				require.Len(t, keys, 1)

				// the server performs a remote list of keys
				// via ssh frames. to prevent the test from terminating
				// before it can complete it will write the blob of the
				// listed key back on the ssh frame. verify that the key
				// it received matches the one from out local keyring.
				out = make([]byte, len(keys[0].Blob))
				n, err = conn.Read(out)
				require.NoError(t, err)
				require.Len(t, keys[0].Blob, n)
				require.Equal(t, keys[0].Blob, out)

				// close the stream
				require.NoError(t, conn.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const noLoginName = ""
			conn, details, err := pack.Client.DialHost(context.Background(), test.target, test.cluster, noLoginName, nil, test.keyring)
			test.assertion(t, conn, details, err)
		})
	}
}

// testPack used to test a [Client].
type testPack struct {
	Client *Client
	Server transportv1pb.TransportServiceServer
}

// newServer creates a [grpc.Server] and registers the
// provided [transportv1pb.TransportServiceServer] with it opens
// an authenticated Client.
func newServer(t *testing.T, srv transportv1pb.TransportServiceServer) testPack {
	// gRPC testPack.
	const bufSize = 100 // arbitrary
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})

	s := grpc.NewServer()
	t.Cleanup(func() {
		s.GracefulStop()
		s.Stop()
	})

	// Register service.
	transportv1pb.RegisterTransportServiceServer(s, srv)

	// Start.
	go func() {
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
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
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1000)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return testPack{
		Client: &Client{clt: transportv1pb.NewTransportServiceClient(cc)},
		Server: srv,
	}
}

// newKeyring returns an [agent.ExtendedAgent] that has
// one key populated in it.
func newKeyring(t *testing.T) agent.ExtendedAgent {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	keyring := agent.NewKeyring()

	require.NoError(t, keyring.Add(agent.AddedKey{
		PrivateKey:   private,
		Comment:      "test",
		LifetimeSecs: math.MaxUint32,
	}))

	extendedKeyring, ok := keyring.(agent.ExtendedAgent)
	require.True(t, ok)

	return extendedKeyring
}

// serverStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxySSHServer]. Instead of
// reading directly from the stream reads are from an incoming
// channel that is fed by the multiplexer.
type serverStream struct {
	incomingC  chan []byte
	stream     transportv1pb.TransportService_ProxySSHServer
	responseFn func(payload []byte) *transportv1pb.ProxySSHResponse
}

func newServerStream(stream transportv1pb.TransportService_ProxySSHServer, responseFn func(payload []byte) *transportv1pb.ProxySSHResponse) *serverStream {
	return &serverStream{
		incomingC:  make(chan []byte, 10),
		stream:     stream,
		responseFn: responseFn,
	}
}

func (s *serverStream) Recv() ([]byte, error) {
	select {
	case <-s.stream.Context().Done():
		return nil, io.EOF
	case frame := <-s.incomingC:
		return frame, nil
	}
}

func (s *serverStream) Send(frame []byte) error {
	return trace.Wrap(s.stream.Send(s.responseFn(frame)))
}
