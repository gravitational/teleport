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

package inventory

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// DownstreamCreateFunc is a function that creates a downstream inventory control stream.
type DownstreamCreateFunc func(ctx context.Context) (client.DownstreamInventoryControlStream, error)

// DownstreamPingHandler is a function that handles ping messages that come down the inventory control stream.
type DownstreamPingHandler func(sender DownstreamSender, msg proto.DownstreamInventoryPing)

// DownstreamHandle is a persistent handle used to interact with the current downstream half of the inventory
// control stream. This handle automatically re-creates the control stream if it fails. The latest (or next, if
// currently unhealthy) control stream send-half can be accessed/awaited via the Sender() channel. The intended usage
// pattern is that handlers for incoming messages are registered once, while components that need to send messages
// should re-acquire a new sender each time the old one fails. If send logic cares about auth server version, make
// sure to re-check the version *for each* sender, since different streams may be connected to different auth servers.
type DownstreamHandle interface {
	// Sender is used to asynchronously access a send-only reference to the current control
	// stream instance. If not currently healthy, this blocks indefinitely until a healthy control
	// stream is established.
	Sender() <-chan DownstreamSender
	// RegisterPingHandler registers a handler for downstream ping messages, returning
	// a de-registration function.
	RegisterPingHandler(DownstreamPingHandler) (unregister func())
	// CloseContext gets the close context of the downstream handle.
	CloseContext() context.Context
	// Close closes the downstream handle.
	Close() error
}

// DownstreamSender is a send-only reference to the downstream half of an inventory control stream. Components that
// require use of the inventory control stream should accept a DownstreamHandle instead, and take a reference to
// the sender via the Sender() method.
type DownstreamSender interface {
	// Send sends a message up the control stream.
	Send(ctx context.Context, msg proto.UpstreamInventoryMessage) error
	// Hello gets the cached downstream hello that was sent by the auth server
	// when the stream was initialized.
	Hello() proto.DownstreamInventoryHello
	// Done signals closure of the underlying stream.
	Done() <-chan struct{}
}

// NewDownstreamHandle creates a new downstream inventory control handle which will create control streams via the
// supplied create func and manage hello exchange with the supplied upstream hello.
func NewDownstreamHandle(fn DownstreamCreateFunc, hello proto.UpstreamInventoryHello) DownstreamHandle {
	ctx, cancel := context.WithCancel(context.Background())
	handle := &downstreamHandle{
		senderC:      make(chan DownstreamSender),
		pingHandlers: make(map[uint64]DownstreamPingHandler),
		closeContext: ctx,
		cancel:       cancel,
	}
	go handle.run(fn, hello)
	return handle
}

func SendHeartbeat(ctx context.Context, handle DownstreamHandle, hb proto.InventoryHeartbeat, retry utils.Retry) {
	for {
		select {
		case sender := <-handle.Sender():
			if err := sender.Send(ctx, hb); err != nil {
				continue
			}
			return
		case <-ctx.Done():
			return
		case <-handle.CloseContext().Done():
			return
		}
	}
}

type downstreamHandle struct {
	mu           sync.Mutex
	handlerNonce uint64
	pingHandlers map[uint64]DownstreamPingHandler
	senderC      chan DownstreamSender
	closeContext context.Context
	cancel       context.CancelFunc
}

func (h *downstreamHandle) closing() bool {
	return h.closeContext.Err() != nil
}

func (h *downstreamHandle) run(fn DownstreamCreateFunc, hello proto.UpstreamInventoryHello) {
	retry := utils.NewDefaultLinear()
	for {
		h.tryRun(fn, hello)

		if h.closing() {
			return
		}

		log.Debugf("Re-attempt control stream acquisition in ~%s.", retry.Duration())
		select {
		case <-retry.After():
			retry.Inc()
		case <-h.closeContext.Done():
			return
		}
	}
}

func (h *downstreamHandle) tryRun(fn DownstreamCreateFunc, hello proto.UpstreamInventoryHello) {
	stream, err := fn(h.CloseContext())
	if err != nil {
		if !h.closing() {
			log.Warnf("Failed to create inventory control stream: %v.", err)
		}
		return
	}

	if err := h.handleStream(stream, hello); err != nil {
		if !h.closing() {
			log.Warnf("Inventory control stream failed: %v", err)
		}
		return
	}
}

func (h *downstreamHandle) handleStream(stream client.DownstreamInventoryControlStream, upstreamHello proto.UpstreamInventoryHello) error {
	defer stream.Close()
	// send upstream hello
	if err := stream.Send(h.closeContext, upstreamHello); err != nil {
		if trace.IsEOF(err) {
			return nil
		}
		return trace.Errorf("failed to send upstream hello: %v", err)
	}

	// wait for downstream hello
	var downstreamHello proto.DownstreamInventoryHello
	select {
	case msg := <-stream.Recv():
		switch m := msg.(type) {
		case proto.DownstreamInventoryHello:
			downstreamHello = m
		default:
			return trace.BadParameter("expected downstream hello, got %T", msg)
		}
	case <-stream.Done():
		if trace.IsEOF(stream.Error()) {
			return nil
		}
		return trace.Wrap(stream.Error())
	case <-h.closeContext.Done():
		return nil
	}

	sender := downstreamSender{stream, downstreamHello}

	// handle incoming messages and distribute sender references
	for {
		select {
		case h.senderC <- sender:
		case msg := <-stream.Recv():
			switch m := msg.(type) {
			case proto.DownstreamInventoryHello:
				return trace.BadParameter("unexpected downstream hello")
			case proto.DownstreamInventoryPing:
				h.handlePing(sender, m)
			default:
				return trace.BadParameter("unexpected downstream message type: %T", m)
			}
		case <-stream.Done():
			if trace.IsEOF(stream.Error()) {
				return nil
			}
			return trace.Wrap(stream.Error())
		case <-h.closeContext.Done():
			return nil
		}
	}
}

func (h *downstreamHandle) handlePing(sender DownstreamSender, msg proto.DownstreamInventoryPing) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.pingHandlers) == 0 {
		log.Warnf("Got ping with no handlers registered (id=%d).", msg.ID)
		return
	}
	for _, handler := range h.pingHandlers {
		go handler(sender, msg)
	}
}

func (h *downstreamHandle) RegisterPingHandler(handler DownstreamPingHandler) (unregister func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	nonce := h.handlerNonce
	h.handlerNonce++
	h.pingHandlers[nonce] = handler
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.pingHandlers, nonce)
	}
}

func (h *downstreamHandle) Sender() <-chan DownstreamSender {
	return h.senderC
}

func (h *downstreamHandle) CloseContext() context.Context {
	return h.closeContext
}

func (h *downstreamHandle) Close() error {
	h.cancel()
	return nil
}

type downstreamSender struct {
	client.DownstreamInventoryControlStream
	hello proto.DownstreamInventoryHello
}

func (d downstreamSender) Hello() proto.DownstreamInventoryHello {
	return d.hello
}

// UpstreamHandle is the primary mechanism for interacting with a fully initialized upstream
// control stream. The hello message cached in this handle has already passed through the
// auth layer, meaning that it represents the verified identity and capabilities of the
// remote entity.
type UpstreamHandle interface {
	client.UpstreamInventoryControlStream
	// Hello gets the cached upstream hello that was used to initialize the stream.
	Hello() proto.UpstreamInventoryHello

	Ping(ctx context.Context) (d time.Duration, err error)
	// HasService is a helper for checking if a given service is associated with this
	// stream.
	HasService(types.SystemRole) bool
}

type upstreamHandle struct {
	client.UpstreamInventoryControlStream
	hello proto.UpstreamInventoryHello

	pingC chan pingRequest

	// --- fields below this point only safe for access by handler goroutine

	// pingCounter is incremented on pings, and used as the ping multiplexing ID.
	pingCounter uint64
	// pings are in-flight pings to be multiplexed by ID.
	pings map[uint64]pendingPing

	// sshServer is the most recently heartbeated ssh server resource (if any).
	sshServer *types.ServerV2
	// retryUpstert inidcates that writing the ssh server lease failed and should be retried.
	retrySSHServerUpsert bool

	// sshServerLease is used to keep alive an ssh server resource that was previously
	// sent over a heartbeat.
	sshServerLease *types.KeepAlive
	// sshServerKeepAliveErrs is a counter used to track the number of failed keepalives
	// with the above lease. too many failures clears the lease.
	sshServerKeepAliveErrs int
}

func newUpstreamHandle(stream client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello) *upstreamHandle {
	pings := make(map[uint64]pendingPing)
	return &upstreamHandle{
		UpstreamInventoryControlStream: stream,
		pingC:                          make(chan pingRequest),
		hello:                          hello,
		pings:                          pings,
	}
}

type pendingPing struct {
	start time.Time
	rspC  chan pingResponse
}

type pingRequest struct {
	rspC chan pingResponse
}

type pingResponse struct {
	d   time.Duration
	err error
}

func (h *upstreamHandle) Ping(ctx context.Context) (d time.Duration, err error) {
	rspC := make(chan pingResponse, 1)
	select {
	case h.pingC <- pingRequest{rspC}:
	case <-h.Done():
		return 0, trace.Errorf("failed to send downstream ping (stream closed)")
	case <-ctx.Done():
		return 0, trace.Errorf("failed to send downstream ping: %v", ctx.Err())
	}

	select {
	case rsp := <-rspC:
		return rsp.d, rsp.err
	case <-h.Done():
		return 0, trace.Errorf("failed to recv upstream pong (stream closed)")
	case <-ctx.Done():
		return 0, trace.Errorf("failed to recv upstream ping: %v", ctx.Err())
	}
}

func (h *upstreamHandle) Hello() proto.UpstreamInventoryHello {
	return h.hello
}

func (h *upstreamHandle) HasService(service types.SystemRole) bool {
	for _, s := range h.hello.Services {
		if s == service {
			return true
		}
	}
	return false
}
