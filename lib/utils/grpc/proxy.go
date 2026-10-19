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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ProxyBidiStreamOption configures [ProxyBidiStream].
type ProxyBidiStreamOption func(*proxyBidiStreamConfig)

type proxyBidiStreamConfig struct {
	firstMessageTimeout    time.Duration
	firstMessageTimeoutErr error
	streamTimeout          proxyTimeout
}

// WithStreamTimeout makes [ProxyBidiStream] return timeoutErr if the timeout
// fires before the server ends the stream. The timeout is the deadline of the
// context that getServer receives and reaches the server as the RPC deadline.
//
// The server's handler should usually enforce a timeout of its own, and the
// timeout here should be longer, so that in the normal case the server ends the
// stream and can audit and log its own timeout, rather than a stream cut short
// by the proxy, which the server cannot reliably tell from a client
// disconnecting.
func WithStreamTimeout(timeout time.Duration, timeoutErr error) ProxyBidiStreamOption {
	return func(cfg *proxyBidiStreamConfig) {
		cfg.streamTimeout = proxyTimeout{timeout: timeout, err: timeoutErr}
	}
}

// WithFirstClientMessageTimeout makes [ProxyBidiStream] wait for the client's
// first message before it opens the server stream, and return timeoutErr if
// the timeout fires before the message arrives. A client that opens a stream
// and goes silent therefore costs the server nothing.
//
// The timeout should be shorter than any stream timeout, which keeps running
// during the wait.
func WithFirstClientMessageTimeout(timeout time.Duration, timeoutErr error) ProxyBidiStreamOption {
	return func(cfg *proxyBidiStreamConfig) {
		cfg.firstMessageTimeout = timeout
		cfg.firstMessageTimeoutErr = timeoutErr
	}
}

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
	opts ...ProxyBidiStreamOption,
) error {
	var cfg proxyBidiStreamConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := cfg.streamTimeout.start(client.Context())
	defer cancel()

	if md, ok := metadata.FromIncomingContext(client.Context()); ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// With a first message timeout the proxy receives the client's first message
	// before it opens the server stream, so that a client that goes silent never
	// reaches the server.
	var first *Req
	if cfg.firstMessageTimeout > 0 {
		var err error
		first, err = recvFirstClientMessage(ctx, log, client, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	server, err := getServer(ctx)
	if err != nil {
		if timeoutErr := cfg.streamTimeout.logAndReturnCause(ctx, log); timeoutErr != nil {
			return timeoutErr
		}
		return trace.Wrap(err, "establishing server stream")
	}

	if first != nil {
		// The first client message goes out before the forwarding starts.
		// An io.EOF from Send means the server has already ended the stream. Its
		// terminal status is only available through Recv, so the forwarding still
		// starts below and forwardServerToClient delivers the status to the client.
		if err := server.Send(first); err != nil && !errors.Is(err, io.EOF) {
			log.WarnContext(ctx, "Failed to send to server stream", "error", err)
			return trace.Wrap(err)
		}
	}

	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)

	go func() { clientErrCh <- forwardClientToServer(ctx, log, client, server) }()
	go func() { serverErrCh <- forwardServerToClient(ctx, log, client, server) }()

	for {
		select {
		case <-ctx.Done():
			// The deadline on ctx ends the server stream, but the loop learns of that
			// only from forwardServerToClient, which can be stuck in client.Send
			// waiting for the client to read. Otherwise gRPC releases Send only once
			// this handler returns, so the handler has to observe the deadline on ctx
			// itself.

			if timeoutErr := cfg.streamTimeout.logAndReturnCause(ctx, log); timeoutErr != nil {
				return timeoutErr
			}
			return trace.Wrap(context.Cause(ctx))
		case err := <-serverErrCh:
			// The server stream is authoritative for the RPC's terminal status.
			// Whatever it returns is what the client should see, unless the stream
			// timeout is what ended it: the client then gets timeoutErr in place of
			// the DeadlineExceeded the deadline produced.
			if timeoutErr := cfg.streamTimeout.logAndReturnCause(ctx, log); timeoutErr != nil {
				return timeoutErr
			}
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

// recvFirstClientMessage receives the client's first message under
// cfg.firstMessageTimeout. A client that half-closed without sending anything
// yields (nil, nil): the caller then proceeds as if there were no first
// message, and the next Recv on client returns io.EOF again.
//
// A blocked Recv on a server stream is only released by returning from the RPC
// handler, so the Recv runs in its own goroutine and this function returns on
// timeout. gRPC then cancels the client stream and the goroutine exits.
func recvFirstClientMessage[Req, Resp any](ctx context.Context, log *slog.Logger,
	client grpc.BidiStreamingServer[Req, Resp], cfg proxyBidiStreamConfig,
) (*Req, error) {
	type result struct {
		req *Req
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		req, err := client.Recv()
		resCh <- result{req: req, err: err}
	}()

	timer := time.NewTimer(cfg.firstMessageTimeout)
	defer timer.Stop()
	select {
	case res := <-resCh:
		if errors.Is(res.err, io.EOF) {
			return nil, nil
		}
		if res.err != nil {
			log.DebugContext(ctx, "Failed to receive from client stream", "error", res.err)
			return nil, trace.Wrap(res.err)
		}
		return res.req, nil
	case <-timer.C:
		log.DebugContext(ctx, "Proxied stream timed out waiting for the first client message",
			"timeout", cfg.firstMessageTimeout)
		return nil, trace.Wrap(cfg.firstMessageTimeoutErr)
	case <-ctx.Done():
		// The stream deadline keeps running during the wait.
		if timeoutErr := cfg.streamTimeout.logAndReturnCause(ctx, log); timeoutErr != nil {
			return nil, timeoutErr
		}
		return nil, trace.Wrap(context.Cause(ctx))
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
	defer func() { client.SetTrailer(server.Trailer()) }()

	if md, err := server.Header(); err != nil {
		log.DebugContext(ctx, "Failed to receive headers from server stream", "error", err)
	} else if len(md) > 0 {
		if sendErr := client.SendHeader(md); sendErr != nil {
			log.WarnContext(ctx, "Failed to send headers to client", "error", sendErr)
		}
	}

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

// ProxyUnaryCallOption configures [ProxyUnaryCall].
type ProxyUnaryCallOption func(*proxyUnaryCallConfig)

type proxyUnaryCallConfig struct {
	callTimeout proxyTimeout
}

// WithCallTimeout makes [ProxyUnaryCall] return timeoutErr if the timeout fires
// before the server answers. The timeout is the deadline of the context that
// call receives and reaches the server as the RPC deadline.
//
// The server's handler should usually enforce a timeout of its own, and the
// timeout here should be longer, so that in the normal case the server ends the
// call and tells the client why.
func WithCallTimeout(timeout time.Duration, timeoutErr error) ProxyUnaryCallOption {
	return func(cfg *proxyUnaryCallConfig) {
		cfg.callTimeout = proxyTimeout{timeout: timeout, err: timeoutErr}
	}
}

// ProxyUnaryCall forwards a unary RPC from a client to a server. call makes the
// RPC to the server with a context derived from ctx, and what it returns is
// what the client gets, so the server's status reaches the client as is.
func ProxyUnaryCall[Resp any](ctx context.Context, log *slog.Logger,
	call func(context.Context) (*Resp, error),
	opts ...ProxyUnaryCallOption,
) (*Resp, error) {
	var cfg proxyUnaryCallConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := cfg.callTimeout.start(ctx)
	defer cancel()

	resp, err := call(ctx)
	if err != nil {
		if timeoutErr := cfg.callTimeout.logAndReturnCause(ctx, log); timeoutErr != nil {
			return nil, timeoutErr
		}
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// proxyTimeout is the timeout a proxy puts on the server side of the RPC it
// forwards. The zero value is no timeout.
type proxyTimeout struct {
	timeout time.Duration
	// err is what the proxy returns once timeout expires.
	err error
	// deadline is when timeout expires, set by start.
	deadline time.Time
}

// start derives from ctx the context for the server side of the RPC, with err
// as the cause of its deadline. Without a timeout the context still gets its
// own cancel func, which ProxyBidiStream uses to end the server stream when the
// client side fails.
func (t *proxyTimeout) start(ctx context.Context) (context.Context, context.CancelFunc) {
	if t.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	t.deadline = time.Now().Add(t.timeout)
	return context.WithDeadlineCause(ctx, t.deadline, t.err)
}

// cause returns err once the timeout has expired, and nil before that, so that
// callers can tell the proxy timeout from the client disconnecting or from a
// deadline the client set itself.
//
// It checks the clock rather than a context. gRPC hands the server the same
// deadline, and the server's copy can end the RPC a moment before the proxy's
// context observes its own timer, in which case that context still reports no
// error and no cause.
func (t proxyTimeout) cause() error {
	if t.timeout <= 0 || time.Now().Before(t.deadline) {
		return nil
	}
	return trace.Wrap(t.err)
}

func (t proxyTimeout) logAndReturnCause(ctx context.Context, log *slog.Logger) error {
	err := t.cause()
	if err != nil {
		// This generally shouldn't happen unless the server side stops responding.
		// Log the timeout at warn level for better visibility.
		log.WarnContext(ctx, "Proxied RPC timed out", "timeout", t.timeout)
	}
	return err
}
