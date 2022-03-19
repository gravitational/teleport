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
	"fmt"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "google.golang.org/grpc/examples/features/proto/echo"
)

func TestChainUnaryServerInterceptors(t *testing.T) {
	handler := func(context.Context, interface{}) (interface{}, error) { return "resp", fmt.Errorf("error") }

	interceptors := []grpc.UnaryServerInterceptor{}
	for i := 1; i < 5; i++ {
		i := i
		interceptors = append(
			interceptors,
			func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
				resp, err := handler(ctx, req)
				return fmt.Sprintf("%d %v", i, resp), fmt.Errorf("%d %v", i, err)
			})
	}

	chainedInterceptor := ChainUnaryServerInterceptors(interceptors[0], interceptors[1:]...)

	// interceptors should be called in order and errors should be propagated
	resp, err := chainedInterceptor(nil, nil, nil, handler)
	require.Equal(t, "1 2 3 4 resp", resp)
	require.Equal(t, "1 2 3 4 error", err.Error())
}

func TestChainStreamServerInterceptors(t *testing.T) {
	handler := func(interface{}, grpc.ServerStream) error { return fmt.Errorf("handler") }

	interceptors := []grpc.StreamServerInterceptor{}
	for i := 1; i < 5; i++ {
		i := i
		interceptors = append(
			interceptors,
			func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				return fmt.Errorf("%d %v", i, handler(srv, ss))
			})
	}

	chainedInterceptor := ChainStreamServerInterceptors(interceptors[0], interceptors[1:]...)

	// interceptors should be called in order
	err := chainedInterceptor(nil, nil, nil, handler)
	require.Equal(t, "1 2 3 4 handler", err.Error())
}

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

// TestGRPCUnaryErrorWrapping tests the error wrapping capability of the client
// and server unary and stream interceptors
func TestGRPCUnaryErrorWrapping(t *testing.T) {
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
		grpc.WithInsecure(),
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

	err = stream.Send(&pb.EchoRequest{Message: "Hi!"})
	require.NoError(t, err)

	resp, err = stream.Recv()
	require.True(t, trace.IsAlreadyExists(err))
	require.Equal(t, err.Error(), "already exists")
}
