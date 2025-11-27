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

// proxy implements a proxy service that proxies requests from the unauthenticated gRPC service in
// the proxy service to the device trust gRPC service in the auth service.
package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/trace"
)

type ServiceConfig struct {
	DevicesClient devicepb.DeviceTrustServiceClient
	Log           *slog.Logger
}

func New(cfg ServiceConfig) (*Service, error) {
	if cfg.DevicesClient == nil {
		return nil, trace.BadParameter("missing DevicesClient")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Service{
		devicesClient: cfg.DevicesClient,
		log:           cfg.Log,
	}, nil
}

type Service struct {
	devicepb.UnimplementedDeviceTrustServiceServer
	// devicesClient is the client to the device trust gRPC service in the auth service.
	devicesClient devicepb.DeviceTrustServiceClient
	log           *slog.Logger
}

func (s *Service) AuthenticateDevice(client grpc.BidiStreamingServer[devicepb.AuthenticateDeviceRequest, devicepb.AuthenticateDeviceResponse]) error {
	ctx, cancel := context.WithCancel(client.Context())
	defer cancel()
	s.log.DebugContext(ctx, "AuthenticateDevice has started")
	defer s.log.DebugContext(ctx, "AuthenticateDevice has ended")
	server, err := s.devicesClient.AuthenticateDevice(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return proxyBidiStream(ctx, s.log, client, server, func(clientMsg *devicepb.AuthenticateDeviceRequest) error {
		switch clientMsg.Payload.(type) {
		case *devicepb.AuthenticateDeviceRequest_Init:
			p, ok := peer.FromContext(ctx)
			if !ok {
				return trace.BadParameter("no peer information found in context")
			}
			clientIP, _, err := net.SplitHostPort(p.Addr.String())
			if err != nil {
				return trace.Wrap(err, "extracting IP from client address")
			}
			clientMsg.GetInit().ClientIp = clientIP
		}
		return nil
	})
}

// TODO: Transform modifyReqFn into an opt.
func proxyBidiStream[Req any, Res any](ctx context.Context, log *slog.Logger, client grpc.BidiStreamingServer[Req, Res], server grpc.BidiStreamingClient[Req, Res], modifyReqFn func(*Req) error) error {
	errChan := make(chan error, 2)
	go func() {
		errChan <- trace.Wrap(forwardClientToServer(ctx, log, client, server, modifyReqFn))
	}()

	go func() {
		errChan <- trace.Wrap(forwardServerToClient(ctx, log, client, server))
	}()

	// Return immediately on the first value. Since we're within a handler for a bidi stream,
	// returning an error from the handler is the only way of passing the error back to the client.
	return trace.Wrap(<-errChan)
}

func forwardClientToServer[Req any, Res any](ctx context.Context,
	log *slog.Logger,
	client grpc.BidiStreamingServer[Req, Res],
	server grpc.BidiStreamingClient[Req, Res],
	modifyReqFn func(*Req) error) (err error) {
	defer log.DebugContext(ctx, "Finished forwarding from client to server")
	defer func() {
		// CloseSend always returns nil error.
		_ = server.CloseSend()
	}()

	for {
		log.DebugContext(ctx, "Waiting for client message")
		clientMsg, err := client.Recv()
		log.DebugContext(ctx, "Got client message", "message", clientMsg, "error", err)
		if errors.Is(err, io.EOF) {
			// Client is done sending messages.
			return nil
		}
		if err != nil {
			return trace.Wrap(err, "receiving message from client")
		}

		if modifyReqFn != nil {
			if err := modifyReqFn(clientMsg); err != nil {
				return trace.Wrap(err, "modifying client message")
			}
		}

		if err := server.Send(clientMsg); err != nil {
			return trace.Wrap(err, "sending message from client to server")
		}
	}
}

func forwardServerToClient[Req any, Res any](ctx context.Context,
	log *slog.Logger,
	client grpc.BidiStreamingServer[Req, Res],
	server grpc.BidiStreamingClient[Req, Res]) (err error) {
	defer log.DebugContext(ctx, "Finished forwarding from server to client")

	for {
		log.DebugContext(ctx, "Waiting for server message")
		serverMsg, err := server.Recv()
		log.DebugContext(ctx, "Got server message", "message", serverMsg, "error", err)
		if errors.Is(err, io.EOF) {
			// Server stream has terminated with an OK status.
			return nil
		}
		if err != nil {
			// Do not add a message to trace.Wrap here. If the server returns an error in response to a
			// message from the client, the error needs to be proxied with no changes to its structure.
			// Any message added to trace.Wrap here would be appended to the error delivered to the
			// client.
			return trace.Wrap(err)
		}
		if err := client.Send(serverMsg); err != nil {
			return trace.Wrap(err, "sending message from server to client")
		}
	}
}
