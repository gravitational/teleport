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

package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
)

// ProxyBidiStream proxies a bidi-streaming RPC. It forwards messages from
// client to server and responses back to client until the server stream
// finishes (cleanly or with error) or the client stream errors.
//
// getServer is called with a context derived from client.Context() and must
// return the server client stream. Canceling that context tears down both
// directions, so callers should pass it directly to the server dial call.
//
// If the server returns early, any still-in-flight messages the client sent are
// dropped by the proxy. Also, the client can half-close and still receive
// messages from the server. Those two behaviors match what the client would see
// talking to the server directly.
//
// If the client stream fails first, the context gets canceled and the server
// stream returns context.Canceled and that's what the handler returns. This is
// fine in practice because if the client is gone, there's no one reading what
// the handler returns.
func ProxyBidiStream[Req, Resp any](log *slog.Logger, client grpc.BidiStreamingServer[Req, Resp],
	getServer func(context.Context) (grpc.BidiStreamingClient[Req, Resp], error),
) error {
	ctx, cancel := context.WithCancel(client.Context())
	defer cancel()

	server, err := getServer(ctx)
	if err != nil {
		return trace.Wrap(err, "establishing server stream")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- trace.Wrap(forwardClientToServer(ctx, cancel, log, client, server))
	}()

	err = forwardServerToClient(ctx, log, client, server)
	if err != nil {
		// Return immediately so gRPC closes the stream, which unblocks client.Recv()
		// in the forwardClientToServer goroutine. The buffered errCh prevents a leak.
		return trace.Wrap(err)
	}
	return trace.Wrap(<-errCh)
}

func forwardClientToServer[Req, Resp any](ctx context.Context, cancel context.CancelFunc, log *slog.Logger,
	client grpc.BidiStreamingServer[Req, Resp],
	server grpc.BidiStreamingClient[Req, Resp],
) error {
	defer func() {
		// CloseSend always returns nil error.
		_ = server.CloseSend()
	}()

	for {
		req, err := client.Recv()
		if errors.Is(err, io.EOF) {
			// The client closed the send direction and won't send more messages.
			// Close the send direction of the server stream by returning and _do not_
			// cancel the context so that the client can receive any messages that the
			// server sends after getting io.EOF from the client.
			return nil
		}
		if err != nil {
			log.WarnContext(ctx, "Failed to receive from client stream", "error", err)
			cancel()
			return trace.Wrap(err)
		}
		if err := server.Send(req); err != nil {
			log.WarnContext(ctx, "Failed to send to server stream", "error", err)
			cancel()
			return trace.Wrap(err)
		}
	}
}

func forwardServerToClient[Req, Resp any](ctx context.Context, log *slog.Logger,
	client grpc.BidiStreamingServer[Req, Resp],
	server grpc.BidiStreamingClient[Req, Resp],
) error {
	for {
		out, err := server.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			log.WarnContext(ctx, "Failed to receive from server stream", "error", err)
			return trace.Wrap(err)
		}
		if err := client.Send(out); err != nil {
			log.WarnContext(ctx, "Failed to send to client stream", "error", err)
			return trace.Wrap(err)
		}
	}
}
