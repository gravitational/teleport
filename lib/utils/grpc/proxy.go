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
// During the brief window between the server ending the stream and the proxy's
// handler returning, client Send calls return nil rather than io.EOF and are
// dropped. A client that interleaves Send with Recv is unaffected because the
// next Recv carries the terminal status.
func ProxyBidiStream[Req, Resp any](log *slog.Logger, client grpc.BidiStreamingServer[Req, Resp],
	getServer func(context.Context) (grpc.BidiStreamingClient[Req, Resp], error),
) error {
	ctx, cancel := context.WithCancel(client.Context())
	defer cancel()

	server, err := getServer(ctx)
	if err != nil {
		return trace.Wrap(err, "establishing server stream")
	}

	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)

	go func() { clientErrCh <- forwardClientToServer(ctx, log, client, server) }()
	go func() { serverErrCh <- forwardServerToClient(ctx, log, client, server) }()

	for {
		select {
		case err := <-serverErrCh:
			// The server stream is authoritative for the RPC's terminal status.
			// Whatever it returns is what the client should see.
			return trace.Wrap(err)
		case err := <-clientErrCh:
			if err != nil {
				// Something went wrong on the client side (client.Recv failure, or a
				// locally-generated server.Send failure). Cancel the server stream and
				// surface the client error — it's more specific than whatever Canceled
				// serverErrCh is about to produce.
				cancel()
				return trace.Wrap(err)
			}
			// forwardClientToServer finished cleanly: the client half-closed or the
			// server stream is already terminal (Send returned io.EOF). In either
			// case, keep waiting on the server stream to deliver its terminal status.
		}
	}
}

func forwardClientToServer[Req, Resp any](ctx context.Context, log *slog.Logger,
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
			// The client half-closed its send side and won't send more messages.
			// Returning here triggers the deferred CloseSend on the server stream.
			// The caller keeps waiting on the server stream for its terminal status.
			return nil
		}
		if err != nil {
			// Debug log because it's impossible to distinguish between transport and
			// application errors.
			//
			// If both proxying functions were to warn on err from Recv, each
			// application-level err from the server would result in two log lines.
			// First with the server error and the second with a context canceled for
			// the client stream.
			log.DebugContext(ctx, "Failed to receive from client stream", "error", err)
			return trace.Wrap(err)
		}

		err = server.Send(req)
		if errors.Is(err, io.EOF) {
			// io.EOF means the server ended the stream and the real status is
			// discoverable via Recv. forwardServerToClient is running that Recv.
			// Let it surface the terminal status.
			// We can't forward this io.EOF to the client because the client already
			// got nil from its Send when we got its message through client.Recv.
			return nil
		}
		if err != nil {
			log.WarnContext(ctx, "Failed to send to server stream", "error", err)
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
			// Debug log because it's impossible to distinguish between transport and
			// application errors.
			log.DebugContext(ctx, "Failed to receive from server stream", "error", err)
			return trace.Wrap(err)
		}
		if err := client.Send(out); err != nil {
			log.WarnContext(ctx, "Failed to send to client stream", "error", err)
			return trace.Wrap(err)
		}
	}
}
