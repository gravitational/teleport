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

package utils

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/features/proto/echo"
)

// service is used to implement EchoServer
type service struct {
	pb.UnimplementedEchoServer
}

func (s *service) UnaryEcho(ctx context.Context, in *pb.EchoRequest) (*pb.EchoResponse, error) {
	return nil, trace.NotFound("not found")
}

func (s *service) BidirectionalStreamingEcho(stream pb.Echo_BidirectionalStreamingEchoServer) error {
	return trace.AlreadyExists("already exists")
}

// TestGRPCErrorWrapping tests the error wrapping capability of the client
// and server unary and stream interceptors
func TestGRPCErrorWrapping(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(GRPCServerUnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(GRPCServerStreamErrorInterceptor),
	)
	pb.RegisterEchoServer(server, &service{})
	go func() {
		server.Serve(listener)
	}()
	defer server.Stop()

	conn, err := grpc.Dial(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(GRPCClientUnaryErrorInterceptor),
		grpc.WithChainStreamInterceptor(GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	defer conn.Close()

	// test unary interceptor
	client := pb.NewEchoClient(conn)
	resp, err := client.UnaryEcho(context.Background(), &pb.EchoRequest{Message: "Hi!"})
	require.Nil(t, resp)
	require.True(t, trace.IsNotFound(err))
	require.Equal(t, err.Error(), "not found")

	// test stream interceptor
	stream, err := client.BidirectionalStreamingEcho(context.Background())
	require.NoError(t, err)

	sendErr := stream.Send(&pb.EchoRequest{Message: "Hi!"})

	// io.EOF means the server closed the stream, which can
	// happen depending in timing. In either case, it is
	// still safe to recv from the stream and check for
	// the already exists error.
	if sendErr != nil && !errors.Is(sendErr, io.EOF) {
		require.FailNowf(t, "unexpected error", "%v", sendErr)
	}

	_, err = stream.Recv()
	require.True(t, trace.IsAlreadyExists(err))
	require.Equal(t, err.Error(), "already exists")
}
