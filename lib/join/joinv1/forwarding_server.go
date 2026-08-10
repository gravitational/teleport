// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package joinv1

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	grpc "google.golang.org/grpc"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
)

const (
	joinRequestTimeout = time.Minute
)

// forwardingServer is a gRPC service meant to run on the Proxy which forwards
// cluster join requests to the Auth service.
//
// This service intercepts ClientInit requests to set the ForwardedByProxy flag
// and add ProxySuppliedParameters.
type forwardingServer struct {
	joinv1.UnsafeJoinServiceServer

	grpcClient joinv1.JoinServiceClient
}

// RegisterProxyForwardingJoinServiceServer registers the Join gRPC service for
// use on the Proxy, it will forward the client's IP address and Teleport version.
func RegisterProxyForwardingJoinServiceServer(s grpc.ServiceRegistrar, grpcClient joinv1.JoinServiceClient) {
	joinv1.RegisterJoinServiceServer(s, &forwardingServer{
		grpcClient: grpcClient,
	})
}

// Join forwards streaming join requests to the Auth service.
func (s *forwardingServer) Join(serverStream grpc.BidiStreamingServer[joinv1.JoinRequest, joinv1.JoinResponse]) error {
	peerInfo, err := peerInfoFromContext(serverStream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	proxySuppliedParams := &joinv1.ClientInit_ProxySuppliedParams{
		RemoteAddr:    peerInfo.remoteAddr,
		ClientVersion: peerInfo.clientVersion,
	}

	ctx, cancel := context.WithTimeoutCause(serverStream.Context(), joinRequestTimeout,
		trace.LimitExceeded(
			"join attempt timed out after %s, terminating the stream on the proxy",
			joinRequestTimeout,
		))
	defer cancel()

	clientStream, err := s.grpcClient.Join(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	errs := make(chan error, 2)
	go func() {
		errs <- s.forwardRequests(serverStream, clientStream, proxySuppliedParams)
	}()
	go func() {
		errs <- s.forwardResponses(serverStream, clientStream)
	}()

	// Any Send or Recv on a gRPC stream may block and there is no way to
	// cancel it without returning from the gRPC handler, so this function must
	// return immediately on any error without waiting for all spawned
	// goroutines to terminate. The parent context will be canceled before the
	// handler terminates so any blocked Send or Recv on either stream should
	// terminate quickly.
	for range 2 {
		select {
		case <-ctx.Done():
			err := context.Cause(ctx)
			if errors.Is(err, context.Canceled) {
				// A normal context.Canceled is expected when the client cleanly
				// disconnects, no need to log or return it.
				return nil
			}
			log.LogAttrs(ctx, slog.LevelWarn, "Forwarded join request canceled",
				slog.Any("error", err),
				slog.String("remote_addr", peerInfo.remoteAddr),
				slog.String("client_version", peerInfo.clientVersion))
			return trace.Wrap(err)
		case err := <-errs:
			if err != nil {
				log.LogAttrs(ctx, slog.LevelWarn, "Forwarded join request failed",
					slog.Any("error", err),
					slog.String("remote_addr", peerInfo.remoteAddr),
					slog.String("client_version", peerInfo.clientVersion))
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (s *forwardingServer) forwardRequests(
	serverStream grpc.BidiStreamingServer[joinv1.JoinRequest, joinv1.JoinResponse],
	clientStream grpc.BidiStreamingClient[joinv1.JoinRequest, joinv1.JoinResponse],
	proxySuppliedParams *joinv1.ClientInit_ProxySuppliedParams,
) error {
	for {
		req, err := serverStream.Recv()
		if errors.Is(err, io.EOF) {
			return trace.Wrap(clientStream.CloseSend())
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if _, ok := req.Payload.(*joinv1.JoinRequest_ClientInit); ok {
			clientInit := req.Payload.(*joinv1.JoinRequest_ClientInit).ClientInit
			clientInit.ForwardedByProxy = true
			clientInit.ProxySuppliedParameters = proxySuppliedParams
		}
		if err := clientStream.Send(req); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (s *forwardingServer) forwardResponses(
	serverStream grpc.BidiStreamingServer[joinv1.JoinRequest, joinv1.JoinResponse],
	clientStream grpc.BidiStreamingClient[joinv1.JoinRequest, joinv1.JoinResponse],
) error {
	for {
		resp, err := clientStream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if err := serverStream.Send(resp); err != nil {
			return trace.Wrap(err)
		}
	}
}
