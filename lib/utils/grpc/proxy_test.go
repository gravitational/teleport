// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package grpc_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// unimplementedFakeServerService is the gRPC service interface implemented both
// by the proxy and the server.
type unimplementedFakeServerSvc = accessgraphsecretsv1pb.UnimplementedSecretsScannerServiceServer

// fakeServerSvcClient is an interface for a gRPC client that talks to a
// gRPC service implemented by the proxy and the server.
type fakeServerSvcClient = accessgraphsecretsv1pb.SecretsScannerServiceClient

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	m.Run()
}

// TestProxyBidiStream creates two gRPC services: one acting as a server and one
// as a proxy. The proxy uses [grpcutils.ProxyBidiStream] to proxy messages from
// the client to the server.
//
// Both services implement [unimplementedFakeServerSvc]. The server uses
// [fakeServerSvc] as its implementation, whereas the proxy uses [proxyService].
//
// The other tests in this file use the same setup. TestProxyBidiStream tests
// the happy path.
func TestProxyBidiStream(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ReportSecrets(ctx)
	require.NoError(t, err)

	// Send the device assertion init message
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_Init{
					Init: &devicepb.AssertDeviceInit{},
				},
			},
		},
	})
	require.NoError(t, err)

	// Receive the device assertion challenge message
	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.NotNil(t, msg.GetDeviceAssertion().GetChallenge())

	// Send the device assertion challenge response message
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_ChallengeResponse{
					ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{Signature: []byte("response")},
				},
			},
		},
	})
	require.NoError(t, err)

	// Receive the device assertion response message
	msg, err = stream.Recv()
	require.NoError(t, err)
	assert.NotNil(t, msg.GetDeviceAssertion().GetDeviceAsserted())

	// Send close message
	err = stream.CloseSend()
	require.NoError(t, err)

	// Receive the termination message
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestProxyBidiStream_HandlesServerReturningErr(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	// Add a short timeout so if the proxy hangs (as it did before introducing
	// this regression test), the test doesn't wait for a whole minute to fail.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ReportSecrets(ctx)
	require.NoError(t, err)

	// Send incomplete message which should cause the server to return an error.
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.ErrorContains(t, err, "missing device init")
}

// TestProxyBidiStream_PropagatesServerErrorAfterClientEOF asserts that a
// terminal error produced by the server SecretsScannerService *after* the
// client has half-closed (CloseSend) is still propagated through the proxy to
// the client.
//
// This exercises the handler path where forwardClientToServer returns first
// (normal CloseSend) and forwardServerToClient is the one that ends up carrying
// server's terminal status. A handler that treats forwardClientToServer as
// authoritative will finish and the client will see io.EOF instead of the real
// error, masking real server failures.
func TestProxyBidiStream_PropagatesServerErrorAfterClientEOF(t *testing.T) {
	t.Parallel()
	service, fakeServerSvcClient := newFakeServerSvc(t)
	service.postClientEOFErr = trace.AccessDenied("post-EOF validation failed")

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)

	stream, err := client.ReportSecrets(ctx)
	require.NoError(t, err)

	// Full handshake so server reaches the final in.Recv() that ends in EOF.
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_Init{
					Init: &devicepb.AssertDeviceInit{},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_ChallengeResponse{
					ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{Signature: []byte("response")},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	err = stream.CloseSend()
	require.NoError(t, err)

	// The client must see the server error, not a clean io.EOF.
	_, recvErr := stream.Recv()
	require.NotErrorIs(t, recvErr, io.EOF, "client saw clean EOF; server error was swallowed")
	require.ErrorContains(t, recvErr, "post-EOF validation failed")
}

// TestProxyBidiStream_ReturnsEOFWhenServerReturnsEarly asserts that when the
// server ends its handler cleanly (nil) *before* the client has half-closed,
// the proxy propagates that as io.EOF to the client rather than hanging or
// reshaping the server's nil into an error.
func TestProxyBidiStream_ReturnsEOFWhenServerReturnsEarly(t *testing.T) {
	t.Parallel()
	service, fakeServerSvcClient := newFakeServerSvc(t)
	service.returnAfterChallenge = true

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	// Short timeout so a hang surfaces as a test failure rather than waiting
	// out the default go-test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ReportSecrets(ctx)
	require.NoError(t, err)

	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_Init{
					Init: &devicepb.AssertDeviceInit{},
				},
			},
		},
	})
	require.NoError(t, err)

	// Drain the challenge the server sent before returning.
	_, err = stream.Recv()
	require.NoError(t, err)

	// Client sends the next message it would naturally send, not knowing the
	// server has already returned. Under the bug this reshapes the server's
	// clean completion into an error via a failed upstream Send; under the
	// fix the handler has already returned and the Send is irrelevant.
	_ = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_ChallengeResponse{
					ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{Signature: []byte("response")},
				},
			},
		},
	})

	// Server has returned nil. The client must see clean io.EOF, not a
	// proxy-reshaped error and not a hang (which would surface as a
	// DeadlineExceeded from ctx).
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func newFakeServerSvc(t *testing.T) (*fakeServerSvc, fakeServerSvcClient) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	service := &fakeServerSvc{}
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(server, service)
	go func() {
		err := server.Serve(lis)
		assert.NoError(t, err)
	}()
	t.Cleanup(server.GracefulStop)

	client, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return service, accessgraphsecretsv1pb.NewSecretsScannerServiceClient(client)
}

type fakeServerSvc struct {
	unimplementedFakeServerSvc

	// postClientEOFErr, if non-nil, is returned by ReportSecrets after it
	// gets EOF from the client, modeling the server producing a terminal error
	// during post-upload processing (after the client has already half-closed).
	postClientEOFErr error

	// returnAfterChallenge, if true, makes ReportSecrets return nil right after
	// sending the challenge, without waiting for the client's challenge
	// response or for the client to half-close. It models a server that ends
	// the stream early while the client is still mid-conversation.
	returnAfterChallenge bool
}

func (f *fakeServerSvc) ReportSecrets(in accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	msg, err := in.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	if msg.GetDeviceAssertion().GetInit() == nil {
		return trace.BadParameter("missing device init")
	}

	err = in.Send(&accessgraphsecretsv1pb.ReportSecretsResponse{
		Payload: &accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceResponse{
				Payload: &devicepb.AssertDeviceResponse_Challenge{
					Challenge: &devicepb.AuthenticateDeviceChallenge{Challenge: []byte("challenge")},
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if f.returnAfterChallenge {
		return nil
	}
	msg, err = in.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	if msg.GetDeviceAssertion().GetChallengeResponse() == nil {
		return trace.BadParameter("missing device challenge")
	}

	err = in.Send(&accessgraphsecretsv1pb.ReportSecretsResponse{
		Payload: &accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceResponse{
				Payload: &devicepb.AssertDeviceResponse_DeviceAsserted{
					DeviceAsserted: &devicepb.DeviceAsserted{},
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = in.Recv()
	if errors.Is(err, io.EOF) {
		if f.postClientEOFErr != nil {
			return f.postClientEOFErr
		}
		return nil
	}
	return trace.BadParameter("unexpected message")
}

// newProxyService creates a gRPC server under lis and registers in it a gRPC
// service that proxies a specific RPC to the server using
// [grpcutils.ProxyBidiStream].
func newProxyService(t *testing.T, lis net.Listener, client fakeServerSvcClient) {
	t.Helper()

	s := grpc.NewServer()
	t.Cleanup(s.GracefulStop)

	proxySvc := &proxyService{
		serverSvcClient: client,
	}

	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(s, proxySvc)

	go func() {
		err := s.Serve(lis)
		assert.NoError(t, err)
	}()
}

type proxyService struct {
	unimplementedFakeServerSvc

	serverSvcClient fakeServerSvcClient
}

// ReportSecrets is the gRPC handler in the proxy.
// client goes from a client to the proxy. From that point of view, the proxy is
// a server for the client.
// server from getServer goes from the proxy to the server. From that point of
// view, the proxy is a client of the server.
func (p *proxyService) ReportSecrets(client accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	getServer := func(ctx context.Context) (accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient, error) {
		return p.serverSvcClient.ReportSecrets(ctx)
	}
	err := grpcutils.ProxyBidiStream(logtest.NewLogger(), client, getServer)
	return trace.Wrap(err)
}

func newProxyServiceClient(t *testing.T, lis net.Listener) fakeServerSvcClient {
	t.Helper()
	clientConn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return accessgraphsecretsv1pb.NewSecretsScannerServiceClient(clientConn)
}
