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
	"net"

	"github.com/gravitational/trace"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/teleport"
	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/lib/join"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "joinv1")

// RegisterJoinServiceServer registers the Join gRPC service.
func RegisterJoinServiceServer(s grpc.ServiceRegistrar, server *join.Server) {
	joinv1.RegisterJoinServiceServer(s, &joinServer{
		server: server,
	})
}

// joinServer is the gRPC implementation of the new Join service with
// auth-assigned host UUIDs. This implementation is replacing
// lib/join/legacyservice. It is a thin gRPC layer that converts gRPC messages
// to [messages.Request] and [messages.Response] and passes the stream down to
// the protocol-agnostic [join.Server].
type joinServer struct {
	joinv1.UnsafeJoinServiceServer

	server *join.Server
}

// Join is a bidirectional streaming RPC that implements all join methods.
// The client does not need to know the join method ahead of time, all it
// needs is the token name.
//
// The client must send an ClientInit message on the JoinRequest stream to
// initiate the join flow.
//
// The server will reply with a JoinResponse where the payload will vary
// based on the join method specified in the provision token.
func (s *joinServer) Join(grpcStream grpc.BidiStreamingServer[joinv1.JoinRequest, joinv1.JoinResponse]) (err error) {
	peerInfo, err := peerInfoFromContext(grpcStream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	diag := diagnostic.New()
	diag.Set(func(i *diagnostic.Info) {
		i.RemoteAddr = peerInfo.remoteAddr
		i.ClientVersion = peerInfo.clientVersion
	})
	ctx, cancel := context.WithTimeoutCause(grpcStream.Context(), joinRequestTimeout,
		trace.LimitExceeded(
			"join attempt timed out after %s, terminating the stream on the server",
			joinRequestTimeout,
		))
	defer cancel()
	messageStream := &serverStream{
		grpcStream: grpcStream,
		ctx:        ctx,
		diag:       diag,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- trace.Wrap(s.server.Join(messageStream))
	}()
	select {
	case <-ctx.Done():
		return trace.Wrap(context.Cause(ctx))
	case err := <-errCh:
		return trace.Wrap(err)
	}
}

// serverStream wraps a [grpc.BidiStreamingServer] to implement
// [messages.ServerStream].
type serverStream struct {
	grpcStream grpc.BidiStreamingServer[joinv1.JoinRequest, joinv1.JoinResponse]
	ctx        context.Context
	diag       *diagnostic.Diagnostic
}

// Context returns the stream context.
func (s *serverStream) Context() context.Context {
	return s.ctx
}

// Diagnostic returns a diagnostic for the join attempt.
func (s *serverStream) Diagnostic() *diagnostic.Diagnostic {
	return s.diag
}

// Send translates a [messages.Response] to a [joinv1.JoinResponse] and sends
// it on the gRPC stream.
func (s *serverStream) Send(msg messages.Response) error {
	resp, err := responseFromMessage(msg)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.grpcStream.Send(resp))
}

// Recv receives a [joinv1.JoinRequest] from the gRPC stream and translates it
// to a [messages.Request] before returning it.
func (s *serverStream) Recv() (messages.Request, error) {
	req, err := s.grpcStream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	msg, err := requestToMessage(req)
	return msg, trace.Wrap(err)
}

type peerInfo struct {
	remoteAddr    string
	clientVersion string
}

func peerInfoFromContext(ctx context.Context) (peerInfo, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return peerInfo{}, trace.Errorf("could not get gRPC peer from context (this is a bug)")
	}
	remoteAddr := p.Addr.String()
	if ip, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = ip
	}
	clientVersion, _ := metadata.ClientVersionFromContext(ctx)
	return peerInfo{
		remoteAddr:    remoteAddr,
		clientVersion: clientVersion,
	}, nil
}
