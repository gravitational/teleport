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

	"github.com/gravitational/trace"
	grpc "google.golang.org/grpc"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// Client wraps a gRPC client to provide a protocol-agnostic client for cluster
// joining.
type Client struct {
	grpcClient joinv1.JoinServiceClient
}

// NewClient returns a new [Client] wrapping the plain gRPC client.
func NewClient(grpcClient joinv1.JoinServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// NewClientFromConn returns a new [Client] wrapping plain gRPC ClientConn.
func NewClientFromConn(cc *grpc.ClientConn) *Client {
	return &Client{
		grpcClient: joinv1.NewJoinServiceClient(cc),
	}
}

// Join implements cluster joining for nodes and bots.
func (c *Client) Join(ctx context.Context) (messages.ClientStream, error) {
	ctx, cancel := context.WithCancelCause(ctx)

	grpcStream, err := c.grpcClient.Join(ctx)
	if err != nil {
		cancel(err)
		return nil, trace.Wrap(err)
	}

	return &clientStream{
		grpcStream: grpcStream,
		cancel:     cancel,
	}, nil
}

// clientStream implements [messages.ClientStream]. Internally it converts
// [messages.Request] and [messages.Response] type to/from gRPC messages and
// translates Send/Recv calls to calls on the real gRPC client.
type clientStream struct {
	grpcStream grpc.BidiStreamingClient[joinv1.JoinRequest, joinv1.JoinResponse]
	cancel     context.CancelCauseFunc
}

// Send sends a request to the join service. Canceling the parent context
// passed to Join will unblock the operation.
func (s *clientStream) Send(msg messages.Request) (err error) {
	defer func() {
		if err != nil {
			s.cancel(err)
		}
	}()
	req, err := requestFromMessage(msg)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.grpcStream.Send(req); err != nil {
		return trace.Wrap(err, "sending request to gRPC stream")
	}
	return nil
}

// Recv receives a response from the join service. Canceling the parent
// context passed to Join will unblock the operation.
func (s *clientStream) Recv() (msg messages.Response, err error) {
	defer func() {
		if err != nil && !errors.Is(err, io.EOF) {
			s.cancel(err)
		}
	}()
	resp, err := s.grpcStream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, io.EOF
	}
	if err != nil {
		return nil, trace.Wrap(err, "reading response from gRPC stream")
	}
	msg, err = responseToMessage(resp)
	return msg, trace.Wrap(err)
}

func (s *clientStream) CloseSend() error {
	return trace.Wrap(s.grpcStream.CloseSend())
}
