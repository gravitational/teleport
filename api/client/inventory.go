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

package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
)

// DownstreamInventoryControlStream is the client/agent side of a bidirectional stream established
// between teleport instances and auth servers.
type DownstreamInventoryControlStream interface {
	// Send attempts to send an upstream message. An error returned from this
	// method either indicates that the stream itself has failed, or that the
	// supplied context was canceled.
	Send(ctx context.Context, msg proto.UpstreamInventoryMessage) error
	// Recv accesses the incoming/downstream message channel.
	Recv() <-chan proto.DownstreamInventoryMessage
	// Close closes the underlying stream without error.
	Close() error
	// CloseWithError closes the underlying stream with an error that can later
	// be retrieved with Error(). Subsequent calls to CloseWithError have no effect.
	CloseWithError(err error) error
	// Done signals that the stream has been closed.
	Done() <-chan struct{}
	// Error checks for any error associated with stream closure (returns `nil` if
	// the stream is open, or io.EOF if the stream was closed without error).
	Error() error
}

// UpstreamInventoryControlStream is the server/controller side of a bidirectional stream established
// between teleport instances and auth servers.
type UpstreamInventoryControlStream interface {
	// Send attempts to send a downstream message.  An error returned from this
	// method either indicates that the stream itself has failed, or that the
	// supplied context was canceled.
	Send(ctx context.Context, msg proto.DownstreamInventoryMessage) error
	// Recv access the incoming/upstream message channel.
	Recv() <-chan proto.UpstreamInventoryMessage
	// PeerAddr gets the underlying TCP peer address (may be empty in some cases).
	PeerAddr() string
	// Close closes the underlying stream without error.
	Close() error
	// CloseWithError closes the underlying stream with an error that can later
	// be retrieved with Error(). Subsequent calls to CloseWithError have no effect.
	CloseWithError(err error) error
	// Done signals that the stream has been closed.
	Done() <-chan struct{}
	// Error checks for any error associated with stream closure (returns `nil` if
	// the stream is open, or io.EOF if the stream closed without error).
	Error() error
}

type ICSPipeOption func(*pipeOptions)

type pipeOptions struct {
	peerAddrFn func() string
}

func ICSPipePeerAddr(peerAddr string) ICSPipeOption {
	return ICSPipePeerAddrFn(func() string {
		return peerAddr
	})
}

func ICSPipePeerAddrFn(fn func() string) ICSPipeOption {
	return func(opts *pipeOptions) {
		opts.peerAddrFn = fn
	}
}

// InventoryControlStreamPipe creates the two halves of an inventory control stream over an in-memory
// pipe.
func InventoryControlStreamPipe(opts ...ICSPipeOption) (UpstreamInventoryControlStream, DownstreamInventoryControlStream) {
	var options pipeOptions
	for _, opt := range opts {
		opt(&options)
	}
	pipe := &pipeControlStream{
		downC:      make(chan proto.DownstreamInventoryMessage),
		upC:        make(chan proto.UpstreamInventoryMessage),
		doneC:      make(chan struct{}),
		peerAddrFn: options.peerAddrFn,
	}
	return upstreamPipeControlStream{pipe}, downstreamPipeControlStream{pipe}
}

type pipeControlStream struct {
	downC      chan proto.DownstreamInventoryMessage
	upC        chan proto.UpstreamInventoryMessage
	peerAddrFn func() string
	mu         sync.Mutex
	err        error
	doneC      chan struct{}
}

func (p *pipeControlStream) Close() error {
	return p.CloseWithError(nil)
}

func (p *pipeControlStream) CloseWithError(err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		// stream already closed
		return nil
	}

	if err != nil {
		p.err = err
	} else {
		// represent "closure without error" with EOF.
		p.err = io.EOF
	}
	close(p.doneC)
	return nil
}

func (p *pipeControlStream) Done() <-chan struct{} {
	return p.doneC
}

func (p *pipeControlStream) Error() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

type upstreamPipeControlStream struct {
	*pipeControlStream
}

func (u upstreamPipeControlStream) Send(ctx context.Context, msg proto.DownstreamInventoryMessage) error {
	select {
	case u.downC <- msg:
		return nil
	case <-u.Done():
		return trace.Errorf("failed to send downstream inventory message (pipe closed)")
	case <-ctx.Done():
		return trace.Errorf("failed to send downstream inventory message: %v", ctx.Err())
	}
}

func (u upstreamPipeControlStream) Recv() <-chan proto.UpstreamInventoryMessage {
	return u.upC
}

func (u upstreamPipeControlStream) PeerAddr() string {
	if u.peerAddrFn != nil {
		return u.peerAddrFn()
	}
	return ""
}

type downstreamPipeControlStream struct {
	*pipeControlStream
}

func (d downstreamPipeControlStream) Send(ctx context.Context, msg proto.UpstreamInventoryMessage) error {
	select {
	case d.upC <- msg:
		return nil
	case <-d.Done():
		return trace.Errorf("failed to send upstream inventory message (pipe closed)")
	case <-ctx.Done():
		return trace.Errorf("failed to send upstream inventory message: %v", ctx.Err())
	}
}

func (d downstreamPipeControlStream) Recv() <-chan proto.DownstreamInventoryMessage {
	return d.downC
}

// InventoryControlStream opens a new control stream.  The first message sent must be an
// UpstreamInventoryHello, and the first message received must be a DownstreamInventoryHello.
func (c *Client) InventoryControlStream(ctx context.Context) (DownstreamInventoryControlStream, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	stream, err := c.grpc.InventoryControlStream(cancelCtx)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	return newDownstreamInventoryControlStream(stream, cancel), nil
}

func (c *Client) GetInventoryStatus(ctx context.Context, req proto.InventoryStatusRequest) (proto.InventoryStatusSummary, error) {
	rsp, err := c.grpc.GetInventoryStatus(ctx, &req)
	if err != nil {
		return proto.InventoryStatusSummary{}, trace.Wrap(err)
	}

	return *rsp, nil
}

func (c *Client) PingInventory(ctx context.Context, req proto.InventoryPingRequest) (proto.InventoryPingResponse, error) {
	rsp, err := c.grpc.PingInventory(ctx, &req)
	if err != nil {
		return proto.InventoryPingResponse{}, trace.Wrap(err)
	}

	return *rsp, nil
}

func (c *Client) GetInstances(ctx context.Context, filter types.InstanceFilter) stream.Stream[types.Instance] {
	// set up cancelable context so that Stream.Done can close the stream if the caller
	// halts early.
	ctx, cancel := context.WithCancel(ctx)

	instances, err := c.grpc.GetInstances(ctx, &filter)
	if err != nil {
		cancel()
		return stream.Fail[types.Instance](trace.Wrap(err))
	}
	return stream.Func[types.Instance](func() (types.Instance, error) {
		instance, err := instances.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// io.EOF signals that stream has completed successfully
				return nil, io.EOF
			}
			return nil, trace.Wrap(err)
		}
		return instance, nil
	}, cancel)
}

func newDownstreamInventoryControlStream(stream proto.AuthService_InventoryControlStreamClient, cancel context.CancelFunc) DownstreamInventoryControlStream {
	ics := &downstreamICS{
		sendC:  make(chan upstreamSend),
		recvC:  make(chan proto.DownstreamInventoryMessage),
		cancel: cancel,
		doneC:  make(chan struct{}),
	}

	go ics.runRecvLoop(stream)
	go ics.runSendLoop(stream)

	return ics
}

// upstreamSend is a helper message used to help us inject per-send context cancellation
type upstreamSend struct {
	msg  proto.UpstreamInventoryMessage
	errC chan error
}

// downstreamICS is a helper which manages a proto.AuthService_InventoryControlStreamClient
// stream and wraps its API to use friendlier types and support select/cancellation.
type downstreamICS struct {
	sendC  chan upstreamSend
	recvC  chan proto.DownstreamInventoryMessage
	mu     sync.Mutex
	cancel context.CancelFunc
	doneC  chan struct{}
	err    error
}

// runRecvLoop waits for incoming messages, converts them to the friendlier DownstreamInventoryMessage
// type, and pushes them to the recvC channel.
func (i *downstreamICS) runRecvLoop(stream proto.AuthService_InventoryControlStreamClient) {
	for {
		oneOf, err := stream.Recv()
		if err != nil {
			// preserve EOF to help distinguish "ok" closure.
			if !errors.Is(err, io.EOF) {
				err = trace.Errorf("inventory control stream closed: %v", trace.Wrap(err))
			}
			i.CloseWithError(err)
			return
		}

		var msg proto.DownstreamInventoryMessage

		switch {
		case oneOf.GetHello() != nil:
			msg = *oneOf.GetHello()
		case oneOf.GetPing() != nil:
			msg = *oneOf.GetPing()
		case oneOf.GetUpdateLabels() != nil:
			msg = *oneOf.GetUpdateLabels()
		default:
			slog.WarnContext(stream.Context(), "received unknown downstream message", "message", oneOf)
			continue
		}

		select {
		case i.recvC <- msg:
		case <-i.Done():
			// stream closed by other goroutine
			return
		}
	}
}

// runSendLoop pulls messages off of the sendC channel, applies the appropriate protobuf wrapper types,
// and sends them over the stream.
func (i *downstreamICS) runSendLoop(stream proto.AuthService_InventoryControlStreamClient) {
	for {
		select {
		case sendMsg := <-i.sendC:
			var oneOf proto.UpstreamInventoryOneOf
			switch msg := sendMsg.msg.(type) {
			case proto.UpstreamInventoryHello:
				oneOf.Msg = &proto.UpstreamInventoryOneOf_Hello{
					Hello: &msg,
				}
			case proto.InventoryHeartbeat:
				oneOf.Msg = &proto.UpstreamInventoryOneOf_Heartbeat{
					Heartbeat: &msg,
				}
			case proto.UpstreamInventoryPong:
				oneOf.Msg = &proto.UpstreamInventoryOneOf_Pong{
					Pong: &msg,
				}
			case proto.UpstreamInventoryAgentMetadata:
				oneOf.Msg = &proto.UpstreamInventoryOneOf_AgentMetadata{
					AgentMetadata: &msg,
				}
			case proto.UpstreamInventoryGoodbye:
				oneOf.Msg = &proto.UpstreamInventoryOneOf_Goodbye{
					Goodbye: &msg,
				}
			default:
				sendMsg.errC <- trace.BadParameter("cannot send unexpected upstream msg type: %T", msg)
				continue
			}
			err := stream.Send(&oneOf)
			sendMsg.errC <- err
			if err != nil {
				// preserve EOF errors
				if !errors.Is(err, io.EOF) {
					err = trace.Errorf("upstream send failed: %v", err)
				}
				i.CloseWithError(err)
				return
			}
		case <-i.Done():
			// stream closed by other goroutine
			return
		}
	}
}

func (i *downstreamICS) Send(ctx context.Context, msg proto.UpstreamInventoryMessage) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}

	errC := make(chan error, 1)
	select {
	case i.sendC <- upstreamSend{msg: msg, errC: errC}:
		select {
		case err := <-errC:
			return trace.Wrap(err)
		case <-ctx.Done():
			return trace.Errorf("inventory control msg send result skipped: %w", ctx.Err())
		}
	case <-ctx.Done():
		return trace.Errorf("inventory control msg not sent: %w", ctx.Err())
	case <-i.Done():
		err := i.Error()
		if err == nil {
			return trace.Errorf("inventory control stream externally closed during send")
		}
		return trace.Errorf("inventory control msg not sent: %w", err)
	}
}

func (i *downstreamICS) Recv() <-chan proto.DownstreamInventoryMessage {
	return i.recvC
}

func (i *downstreamICS) Done() <-chan struct{} {
	return i.doneC
}

func (i *downstreamICS) Close() error {
	return i.CloseWithError(nil)
}

func (i *downstreamICS) CloseWithError(err error) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.err != nil {
		// already closed
		return nil
	}
	if err != nil {
		i.err = err
	} else {
		i.err = io.EOF
	}
	i.cancel()
	close(i.doneC)
	return nil
}

func (i *downstreamICS) Error() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.err
}

// NewUpstreamInventoryControlStream wraps the server-side control stream handle. For use as part of the internals
// of the auth server's gRPC API implementation.
func NewUpstreamInventoryControlStream(stream proto.AuthService_InventoryControlStreamServer, peerAddr string) UpstreamInventoryControlStream {
	ics := &upstreamICS{
		sendC:    make(chan downstreamSend),
		recvC:    make(chan proto.UpstreamInventoryMessage),
		doneC:    make(chan struct{}),
		peerAddr: peerAddr,
	}

	go ics.runRecvLoop(stream)
	go ics.runSendLoop(stream)

	return ics
}

// downstreamSend is a helper message used to help us inject per-send context cancellation
type downstreamSend struct {
	msg  proto.DownstreamInventoryMessage
	errC chan error
}

// upstreamICS is a helper which manages a proto.AuthService_InventoryControlStreamServer
// stream and wraps its API to use friendlier types and support select/cancellation.
type upstreamICS struct {
	sendC    chan downstreamSend
	recvC    chan proto.UpstreamInventoryMessage
	peerAddr string
	mu       sync.Mutex
	doneC    chan struct{}
	err      error
}

// runRecvLoop waits for incoming messages, converts them to the friendlier UpstreamInventoryMessage
// type, and pushes them to the recvC channel.
func (i *upstreamICS) runRecvLoop(stream proto.AuthService_InventoryControlStreamServer) {
	for {
		oneOf, err := stream.Recv()
		if err != nil {
			// preserve eof errors
			if !errors.Is(err, io.EOF) {
				err = trace.Errorf("inventory control stream recv failed: %v", trace.Wrap(err))
			}
			i.CloseWithError(err)
			return
		}

		var msg proto.UpstreamInventoryMessage

		switch {
		case oneOf.GetHello() != nil:
			msg = *oneOf.GetHello()
		case oneOf.GetHeartbeat() != nil:
			msg = *oneOf.GetHeartbeat()
		case oneOf.GetPong() != nil:
			msg = *oneOf.GetPong()
		case oneOf.GetAgentMetadata() != nil:
			msg = *oneOf.GetAgentMetadata()
		case oneOf.GetGoodbye() != nil:
			msg = *oneOf.GetGoodbye()
		default:
			slog.WarnContext(stream.Context(), "received unknown upstream message", "message", oneOf)
			continue
		}

		select {
		case i.recvC <- msg:
		case <-i.Done():
			// stream closed by other goroutine
			return
		}
	}
}

// runSendLoop pulls messages off of the sendC channel, applies the appropriate protobuf wrapper types,
// and sends them over the channel.
func (i *upstreamICS) runSendLoop(stream proto.AuthService_InventoryControlStreamServer) {
	for {
		select {
		case sendMsg := <-i.sendC:
			var oneOf proto.DownstreamInventoryOneOf
			switch msg := sendMsg.msg.(type) {
			case proto.DownstreamInventoryHello:
				oneOf.Msg = &proto.DownstreamInventoryOneOf_Hello{
					Hello: &msg,
				}
			case proto.DownstreamInventoryPing:
				oneOf.Msg = &proto.DownstreamInventoryOneOf_Ping{
					Ping: &msg,
				}
			case proto.DownstreamInventoryUpdateLabels:
				oneOf.Msg = &proto.DownstreamInventoryOneOf_UpdateLabels{
					UpdateLabels: &msg,
				}
			default:
				sendMsg.errC <- trace.BadParameter("cannot send unexpected downstream msg type: %T", msg)
				continue
			}
			err := stream.Send(&oneOf)
			sendMsg.errC <- err
			if err != nil {
				// preserve eof errors
				if !errors.Is(err, io.EOF) {
					err = trace.Errorf("downstream send failed: %v", err)
				}
				i.CloseWithError(err)
				return
			}
		case <-i.Done():
			// stream closed by other goroutine
			return
		}
	}
}

func (i *upstreamICS) Send(ctx context.Context, msg proto.DownstreamInventoryMessage) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}

	errC := make(chan error, 1)
	select {
	case i.sendC <- downstreamSend{msg: msg, errC: errC}:
		select {
		case err := <-errC:
			return trace.Wrap(err)
		case <-ctx.Done():
			return trace.Errorf("inventory control msg send result skipped: %w", ctx.Err())
		}
	case <-ctx.Done():
		return trace.Errorf("inventory control msg not sent: %w", ctx.Err())
	case <-i.Done():
		err := i.Error()
		if err == nil {
			return trace.Errorf("inventory control stream externally closed during send")
		}
		return trace.Errorf("inventory control msg not sent: %w", err)
	}
}

func (i *upstreamICS) Recv() <-chan proto.UpstreamInventoryMessage {
	return i.recvC
}

func (i *upstreamICS) PeerAddr() string {
	return i.peerAddr
}

func (i *upstreamICS) Done() <-chan struct{} {
	return i.doneC
}

func (i *upstreamICS) Close() error {
	return i.CloseWithError(nil)
}

func (i *upstreamICS) CloseWithError(err error) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.err != nil {
		// already closed
		return nil
	}
	if err != nil {
		i.err = err
	} else {
		i.err = io.EOF
	}
	close(i.doneC)
	return nil
}

func (i *upstreamICS) Error() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.err
}
