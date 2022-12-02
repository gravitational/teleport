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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

// we use dedicated global jitters for all the intervals/retires in this
// package. we do this because our jitter usage in this package can scale by
// the number of concurrent connections to auth, making dedicated jitters a
// poor choice (high memory usage for all the rngs).
var (
	seventhJitter = retryutils.NewShardedSeventhJitter()
	halfJitter    = retryutils.NewShardedHalfJitter()
	fullJitter    = retryutils.NewShardedFullJitter()
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

func SendHeartbeat(ctx context.Context, handle DownstreamHandle, hb proto.InventoryHeartbeat, retry retryutils.Retry) {
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

	Ping(ctx context.Context, id uint64) (d time.Duration, err error)
	// HasService is a helper for checking if a given service is associated with this
	// stream.
	HasService(types.SystemRole) bool

	// StateTracker gets the instance state tracker.
	//
	// NOTE: The returned state tracker is *not* locked, and must be locked prior to any
	// access. Prefer using the InstanceStateTracker.WithLock method where possible.
	StateTracker() *InstanceStateTracker

	// HeartbeatInstance triggers an early instance heartbeat. This function does not
	// wait for the instance heartbeat to actually be completed, so calling this and then
	// immediately locking the InstanceStateTracker will likely result in observing the
	// pre-heartbeat state.
	HeartbeatInstance()
}

// InstanceStateTracker tracks the state of a connected instance from the point of view of
// the inventory controller. Values in this struct tend to be lazily updated/consumed. For
// example, the LastHeartbeat value is nil until the first attempt to heartbeat the instance
// has been made *for this TCP connection*, meaning that you can't distinguish between an instance
// that just joined the cluster from one that just reconnected by looking at this struct. Similarly,
// when using this struct to inject control log entries, said entries won't be included until the next
// normal instance heartbeat (though this may be triggered early via UpstreamHandle.HeartbeatInstance()).
// The primary intended usage pattern is for periodic operations to append entries to the QualifiedPendingControlLog,
// and then observe wether or not said entries end up being included on subsequent iterations. This
// patterns lets us achieve a kind of lazy "locking", whereby complex coordination can occur without
// large spikes in backend load. See the QualifiedPendingControlLog field for an example of this pattern.
type InstanceStateTracker struct {
	// MU protects all below fields and must be locked during access of any of the
	// below fields.
	MU sync.Mutex

	// QualifiedPendingControlLog encodes sensitive control log entries that should be written to the backend
	// on the next heartbeat. Appending a value to this field does not guarantee that it will be written. The
	// qualified pending control log is reset if a concurrent create/write occurs. This is a deliberate design
	// choice that is intended to force recalculation of the conditions that merited the log entry being applied
	// if/when a concurrent update occurs. Take, for example, a log entry indicating that an upgrade will be
	// attempted. We don't want to double-attempt an upgrade, so if the underlying resource is concurrently updated,
	// we need to reexamine the new state before deciding if an upgrade attempt is still adviseable. This loop may
	// continue indefinitely until we either observe that our entry was successfully written, or that the condition
	// which originally merited the write no longer holds.
	//
	// NOTE: Since MU is not held *during* an attempt to write to the backend, it is important
	// that this slice is only appended to and never reordered. The heartbeat logic relies upon the ability
	// to trim the first N elements upon successful write in order to avoid skipping and/or double-writing
	// a given entry.
	//
	QualifiedPendingControlLog []types.InstanceControlLogEntry

	// UnqualifiedPendingControlLog is functionally equivalent to QualifiedPendingControlLog except that it is not
	// reset upon concurrent create/update. Appending items here is effectively "fire and forget", though items may
	// still not make it into the control log of the underlying resource if the instance disconnects or the auth server
	// restarts before the next successful write. As a general rule, use the QualifiedPendingControlLog to announce your
	// intention to perform an action, and use the UnqualifiedPendingControlLog to store the results of an action.
	//
	// NOTE: Since MU is not held *during* an attempt to write to the backend, it is important
	// that this slice is only appended to and never reordered. The heartbeat logic relies upon the ability
	// to trim the first N elements upon successful write in order to avoid skipping and/or double-writing
	// a given entry.
	UnqualifiedPendingControlLog []types.InstanceControlLogEntry

	// LastHeartbeat is the last observed heartbeat for this instance. This field is filled lazily and
	// will be nil if the instance only recently connected or joined. Operations that expect to be able to
	// observe the committed state of the instance control log should skip instances for which this field is nil.
	LastHeartbeat types.Instance

	// lastRawHeartbeat is the raw backend value associated with LastHeartbeat. Used to
	// enabled CompareAndSwap based updates.
	lastRawHeartbeat []byte

	// retryHeartbeat is set to true if an unexpected error is hit. We retry exactly once, closing
	// the stream if the retry does not succeede.
	retryHeartbeat bool
}

// WithLock runs the provided closure with the tracker lock held.
func (i *InstanceStateTracker) WithLock(fn func()) {
	i.MU.Lock()
	defer i.MU.Unlock()
	fn()
}

// nextHeartbeat calculates the next heartbeat value. *Must* be called only while lock is held.
func (i *InstanceStateTracker) nextHeartbeat(now time.Time, hello proto.UpstreamInventoryHello, authID string) (types.Instance, error) {
	instance, err := types.NewInstance(hello.ServerID, types.InstanceSpecV1{
		Version:  vc.Normalize(hello.Version),
		Services: hello.Services,
		Hostname: hello.Hostname,
		AuthID:   authID,
		LastSeen: now.UTC(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// preserve control log entries from previous instance if present
	if i.LastHeartbeat != nil {
		instance.AppendControlLog(i.LastHeartbeat.GetControlLog()...)
	}

	if len(i.QualifiedPendingControlLog) > 0 {
		instance.AppendControlLog(i.QualifiedPendingControlLog...)
	}

	if len(i.UnqualifiedPendingControlLog) > 0 {
		instance.AppendControlLog(i.UnqualifiedPendingControlLog...)
	}

	return instance, nil
}

type upstreamHandle struct {
	client.UpstreamInventoryControlStream
	hello proto.UpstreamInventoryHello

	ticker *interval.MultiInterval[intervalKey]

	pingC chan pingRequest

	stateTracker InstanceStateTracker

	// --- fields below this point only safe for access by handler goroutine

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

func (h *upstreamHandle) StateTracker() *InstanceStateTracker {
	return &h.stateTracker
}

func (h *upstreamHandle) HeartbeatInstance() {
	h.ticker.FireNow(instanceHeartbeatKey)
}

func newUpstreamHandle(stream client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello, ticker *interval.MultiInterval[intervalKey]) *upstreamHandle {
	return &upstreamHandle{
		UpstreamInventoryControlStream: stream,
		pingC:                          make(chan pingRequest),
		hello:                          hello,
		pings:                          make(map[uint64]pendingPing),
		ticker:                         ticker,
	}
}

type pendingPing struct {
	start time.Time
	rspC  chan pingResponse
}

type pingRequest struct {
	id   uint64
	rspC chan pingResponse
}

type pingResponse struct {
	d   time.Duration
	err error
}

func (h *upstreamHandle) Ping(ctx context.Context, id uint64) (d time.Duration, err error) {
	rspC := make(chan pingResponse, 1)
	select {
	case h.pingC <- pingRequest{rspC: rspC, id: id}:
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
