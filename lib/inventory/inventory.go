/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package inventory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory/internal/delay"
	"github.com/gravitational/teleport/lib/inventory/metadata"
	"github.com/gravitational/teleport/lib/utils"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

const (
	helloGetterCacheTTL = time.Minute
)

// DownstreamCreateFunc is a function that creates a downstream inventory control stream.
type DownstreamCreateFunc func(ctx context.Context) (client.DownstreamInventoryControlStream, error)

// DownstreamPingHandler is a function that handles ping messages that come down the inventory control stream.
type DownstreamPingHandler func(sender DownstreamSender, msg *proto.DownstreamInventoryPing)

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
	// GetSender provides the last known, if any, DownstreamSender. If the control
	// stream has never been established the returned boolean will be false.
	GetSender() (s DownstreamSender, ok bool)
	// RegisterPingHandler registers a handler for downstream ping messages, returning
	// a de-registration function.
	RegisterPingHandler(DownstreamPingHandler) (unregister func())
	// CloseContext gets the close context of the downstream handle.
	CloseContext() context.Context
	// Close closes the downstream handle.
	Close() error
	// SendGoodbye indicates the downstream half of the connection is starting the
	// termination process. This has no impact on the health of the inventory control
	// stream, nor does it perform any clean up of the connection.
	// In case of soft-reloads, the termination process can take up to 30 hours.
	// The Goodbye message may indicate the reason for the connection termination
	// (shutdown versus soft-reload).
	SetAndSendGoodbye(context.Context, bool, bool) error
	// GetUpstreamLabels gets the labels received from upstream.
	GetUpstreamLabels(kind proto.LabelUpdateKind) map[string]string
}

// DownstreamSender is a send-only reference to the downstream half of an inventory control stream. Components that
// require use of the inventory control stream should accept a DownstreamHandle instead, and take a reference to
// the sender via the Sender() method.
type DownstreamSender interface {
	// Send sends a message up the control stream.
	Send(ctx context.Context, msg proto.UpstreamInventoryMessage) error
	// Hello gets the cached downstream hello that was sent by the auth server
	// when the stream was initialized.
	Hello() *proto.DownstreamInventoryHello
	// Done signals closure of the underlying stream.
	Done() <-chan struct{}
}

type downstreamHandleOptions struct {
	metadataGetter func(ctx context.Context) (*metadata.Metadata, error)
	clock          clockwork.Clock
}

func (options *downstreamHandleOptions) SetDefaults() {
	if options.metadataGetter == nil {
		options.metadataGetter = metadata.Get
	}
	if options.clock == nil {
		options.clock = clockwork.NewRealClock()
	}
}

type DownstreamHandleOption func(c *downstreamHandleOptions)

func withMetadataGetter(getter func(ctx context.Context) (*metadata.Metadata, error)) DownstreamHandleOption {
	return func(opts *downstreamHandleOptions) {
		opts.metadataGetter = getter
	}
}

// WithDownstreamClock overrides existing clock for downstream handle.
func WithDownstreamClock(clock clockwork.Clock) DownstreamHandleOption {
	return func(opts *downstreamHandleOptions) {
		opts.clock = clock
	}
}

// NewDownstreamHandle creates a new downstream inventory control handle which will create control streams via the
// supplied create func and manage hello exchange with the supplied upstream hello.
func NewDownstreamHandle(fn DownstreamCreateFunc, hello HelloGetter, opts ...DownstreamHandleOption) (DownstreamHandle, error) {
	var options downstreamHandleOptions
	for _, opt := range opts {
		opt(&options)
	}
	options.SetDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	// Create a new cache to save the hello result, this cache ensure we don't
	// perform too many lookups instead of crash-looping stream.
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         helloGetterCacheTTL,
		Clock:       options.clock,
		Context:     ctx,
		ReloadOnErr: true,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err, "initializing hello cache")
	}

	// Replace the getter we had by a cached and time-boxed one.
	cachedHelloGetter := func(ctx context.Context) (*proto.UpstreamInventoryHello, error) {
		getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return utils.FnCacheGet(getCtx, cache, "hello", func(ctx context.Context) (*proto.UpstreamInventoryHello, error) {
			return hello(ctx)
		})
	}

	handle := &downstreamHandle{
		senderC:        make(chan DownstreamSender),
		pingHandlers:   make(map[uint64]DownstreamPingHandler),
		closeContext:   ctx,
		cancel:         cancel,
		metadataGetter: options.metadataGetter,
		clock:          options.clock,
		helloGetter:    cachedHelloGetter,
	}
	go handle.run(fn)
	go handle.autoEmitMetadata()
	go handle.autoEmitGoodbye()
	return handle, nil
}

type downstreamHandle struct {
	sender            atomic.Pointer[downstreamSender]
	mu                sync.Mutex
	handlerNonce      uint64
	pingHandlers      map[uint64]DownstreamPingHandler
	senderC           chan DownstreamSender
	closeContext      context.Context
	cancel            context.CancelFunc
	upstreamSSHLabels map[string]string
	metadataGetter    func(ctx context.Context) (*metadata.Metadata, error)
	clock             clockwork.Clock
	helloGetter       HelloGetter
	goodbye           atomic.Pointer[proto.UpstreamInventoryGoodbye]
}

func (h *downstreamHandle) closing() bool {
	return h.closeContext.Err() != nil
}

// autoEmitMetadata sends the agent goodbye once per stream (i.e. connection
// with the auth server) if the agent has already goodbye-ed once. Else it
// does nothing.
func (h *downstreamHandle) autoEmitGoodbye() {
	for {
		// Wait for stream to be opened.
		var sender DownstreamSender
		select {
		case sender = <-h.Sender():
		case <-h.CloseContext().Done():
			return
		}

		goodbye := h.goodbye.Load()
		if goodbye != nil {
			if err := h.sendGoodbye(h.closeContext, sender, goodbye); err != nil && !errors.Is(err, context.Canceled) {
				slog.WarnContext(h.CloseContext(), "Failed to goodbye the upstream", "error", err)
			}
		}

		// Block for the duration of the stream.
		select {
		case <-sender.Done():
		case <-h.CloseContext().Done():
			return
		}
	}
}

// autoEmitMetadata sends the agent metadata once per stream (i.e. connection
// with the auth server).
func (h *downstreamHandle) autoEmitMetadata() {
	md, err := h.metadataGetter(h.CloseContext())
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.WarnContext(h.CloseContext(), "Failed to get agent metadata", "error", err)
		}
		return
	}
	msg := &proto.UpstreamInventoryAgentMetadata{
		OS:                    md.OS,
		OSVersion:             md.OSVersion,
		HostArchitecture:      md.HostArchitecture,
		GlibcVersion:          md.GlibcVersion,
		InstallMethods:        md.InstallMethods,
		ContainerRuntime:      md.ContainerRuntime,
		ContainerOrchestrator: md.ContainerOrchestrator,
		CloudEnvironment:      md.CloudEnvironment,
	}
	for {
		// Wait for stream to be opened.
		var sender DownstreamSender
		select {
		case sender = <-h.Sender():
		case <-h.CloseContext().Done():
			return
		}

		// Send metadata.
		if err := sender.Send(h.CloseContext(), msg); err != nil && !errors.Is(err, context.Canceled) {
			slog.WarnContext(h.CloseContext(), "Failed to send agent metadata", "error", err)
		}

		// Block for the duration of the stream.
		select {
		case <-sender.Done():
		case <-h.CloseContext().Done():
			return
		}
	}
}

func (h *downstreamHandle) run(fn DownstreamCreateFunc) {
	retry := utils.NewDefaultLinear(h.clock)
	for {
		h.tryRun(fn)

		if h.closing() {
			return
		}

		slog.DebugContext(h.closeContext, "Re-attempt control stream acquisition", "backoff", retry.Duration())
		select {
		case <-retry.After():
			retry.Inc()
		case <-h.closeContext.Done():
			return
		}
	}
}

func (h *downstreamHandle) tryRun(fn DownstreamCreateFunc) {
	stream, err := fn(h.CloseContext())
	if err != nil {
		if !h.closing() {
			slog.WarnContext(h.CloseContext(), "Failed to create inventory control stream", "error", err)
		}
		return
	}

	if err := h.handleStream(stream); err != nil {
		if !h.closing() {
			slog.WarnContext(h.CloseContext(), "Inventory control stream failed", "error", err)
		}
		return
	}
}

// HelloGetter is a function that returns a proto.UpstreamInventoryHello.
// This is evaluated each time the stream is being established to craft the upstream Hello.
// Its output is cached for a minute by the downstreamHandle to mitigate load
// issues when if the stream is crash-looping.
type HelloGetter func(ctx context.Context) (*proto.UpstreamInventoryHello, error)

func (h *downstreamHandle) handleStream(stream client.DownstreamInventoryControlStream) error {
	defer stream.Close()

	upstreamHello, err := h.helloGetter(h.CloseContext())
	if err != nil {
		return trace.Wrap(err, "getting upstream hello")
	}

	// send upstream hello
	if err := stream.Send(h.closeContext, upstreamHello); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return trace.Errorf("failed to send upstream hello: %v", err)
	}

	// wait for downstream hello
	var downstreamHello *proto.DownstreamInventoryHello
	select {
	case msg := <-stream.Recv():
		switch m := msg.(type) {
		case *proto.DownstreamInventoryHello:
			downstreamHello = m
		default:
			return trace.BadParameter("expected downstream hello, got %T", msg)
		}
	case <-stream.Done():
		if errors.Is(stream.Error(), io.EOF) {
			return nil
		}
		return trace.Wrap(stream.Error())
	case <-h.closeContext.Done():
		return nil
	}

	sender := downstreamSender{stream, downstreamHello}
	h.sender.Swap(&sender)

	// handle incoming messages and distribute sender references
	for {
		select {
		case h.senderC <- sender:
		case msg := <-stream.Recv():
			switch m := msg.(type) {
			case *proto.DownstreamInventoryHello:
				return trace.BadParameter("unexpected downstream hello")
			case *proto.DownstreamInventoryPing:
				h.handlePing(sender, m)
			case *proto.DownstreamInventoryUpdateLabels:
				h.handleUpdateLabels(m)
			default:
				return trace.BadParameter("unexpected downstream message type: %T", m)
			}
		case <-stream.Done():
			if errors.Is(stream.Error(), io.EOF) {
				return nil
			}
			return trace.Wrap(stream.Error())
		case <-h.closeContext.Done():
			return nil
		}
	}
}

func (h *downstreamHandle) handlePing(sender DownstreamSender, msg *proto.DownstreamInventoryPing) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.pingHandlers) == 0 {
		slog.WarnContext(h.closeContext, "Got ping with no handlers registered", "ping_id", msg.ID)
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

func (h *downstreamHandle) handleUpdateLabels(msg *proto.DownstreamInventoryUpdateLabels) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if msg.Kind == proto.LabelUpdateKind_SSHServerCloudLabels {
		h.upstreamSSHLabels = msg.Labels
	}
}

func (h *downstreamHandle) GetUpstreamLabels(kind proto.LabelUpdateKind) map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if kind == proto.LabelUpdateKind_SSHServerCloudLabels {
		return h.upstreamSSHLabels
	}
	return nil
}

func (h *downstreamHandle) GetSender() (s DownstreamSender, ok bool) {
	sender := h.sender.Load()
	if sender == nil {
		return nil, false
	}

	return sender, true
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

// SendGoodbye crafts a goodbye message, save it, waits for a working stream and sends it to the auth.
// If the downstreamHandle were to reconnect later, the h.autoEmitGoodbye routine would re-emit it.
func (h *downstreamHandle) SetAndSendGoodbye(ctx context.Context, deleteResources bool, softReload bool) error {
	goodbye := &proto.UpstreamInventoryGoodbye{DeleteResources: deleteResources, SoftReload: softReload}
	h.goodbye.Store(goodbye)

	// Wait for an available stream
	select {
	case sender := <-h.Sender():
		return trace.Wrap(h.sendGoodbye(ctx, sender, goodbye))
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-h.CloseContext().Done():
		return nil
	}
}

func (h *downstreamHandle) sendGoodbye(ctx context.Context, sender DownstreamSender, goodbye *proto.UpstreamInventoryGoodbye) error {
	if goodbye == nil {
		return trace.BadParameter("trying to send a nil goodbye, this is a bug")
	}

	capabilities := sender.Hello().Capabilities
	switch {
	case capabilities == nil:
		return nil
	case !capabilities.AppCleanup:
		return nil
	}

	return trace.Wrap(sender.Send(ctx, goodbye))
}

type downstreamSender struct {
	client.DownstreamInventoryControlStream
	hello *proto.DownstreamInventoryHello
}

func (d downstreamSender) Hello() *proto.DownstreamInventoryHello {
	return d.hello
}

// UpstreamHandle is the primary mechanism for interacting with a fully initialized upstream
// control stream. The hello message cached in this handle has already passed through the
// auth layer, meaning that it represents the verified identity and capabilities of the
// remote entity.
type UpstreamHandle interface {
	client.UpstreamInventoryControlStream
	// Hello gets the cached upstream hello that was used to initialize the stream.
	Hello() *proto.UpstreamInventoryHello

	// Goodbye gets the cached upstream goodbye. Returns nil if downstream never sent a Goodbye.
	// This is used to identify if the instance is terminating or being soft-reloaded.
	Goodbye() *proto.UpstreamInventoryGoodbye

	// RegistrationTime gets the timestamp of the control stream initialization.
	RegistrationTime() time.Time

	// AgentMetadata is the service's metadata: OS, glibc version, install methods, ...
	AgentMetadata() *proto.UpstreamInventoryAgentMetadata

	Ping(ctx context.Context, id uint64) (d time.Duration, err error)

	// HasService is a helper for checking if a given service is associated with this
	// stream.
	HasService(types.SystemRole) bool

	// HasControlPlaneService returns true if at least a control plane service
	// is associated with this stream.
	HasControlPlaneService() bool

	// VisitInstanceState runs the provided closure against a representation of the most
	// recently observed instance state, plus any pending control log entries. The returned
	// value may optionally include additional control log entries to add to the pending
	// queues. Inputs and outputs are deep copied to avoid concurrency issues. See the InstanceStateTracker
	// for an explanation of how this system works.
	VisitInstanceState(func(ref InstanceStateRef) InstanceStateUpdate)

	// UpdateLabels updates the labels on the instance.
	UpdateLabels(ctx context.Context, kind proto.LabelUpdateKind, labels map[string]string) error
}

// instanceStateTracker tracks the state of a connected instance from the point of view of
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
type instanceStateTracker struct {
	// mu protects all below fields and must be locked during access of any of the
	// below fields.
	mu sync.Mutex

	// qualifiedPendingControlLog encodes sensitive control log entries that should be written to the backend
	// on the next heartbeat. Appending a value to this field does not guarantee that it will be written. The
	// qualified pending control log is reset if a concurrent create/write occurs. This is a deliberate design
	// choice that is intended to force recalculation of the conditions that merited the log entry being applied
	// if/when a concurrent update occurs. Take, for example, a log entry indicating that an upgrade will be
	// attempted. We don't want to double-attempt an upgrade, so if the underlying resource is concurrently updated,
	// we need to reexamine the new state before deciding if an upgrade attempt is still adviseable. This loop may
	// continue indefinitely until we either observe that our entry was successfully written, or that the condition
	// which originally merited the write no longer holds.
	//
	// NOTE: Since mu is not held *during* an attempt to write to the backend, it is important
	// that this slice is only appended to and never reordered. The heartbeat logic relies upon the ability
	// to trim the first N elements upon successful write in order to avoid skipping and/or double-writing
	// a given entry.
	//
	qualifiedPendingControlLog []types.InstanceControlLogEntry

	// unqualifiedPendingControlLog is functionally equivalent to QualifiedPendingControlLog except that it is not
	// reset upon concurrent create/update. Appending items here is effectively "fire and forget", though items may
	// still not make it into the control log of the underlying resource if the instance disconnects or the auth server
	// restarts before the next successful write. As a general rule, use the QualifiedPendingControlLog to announce your
	// intention to perform an action, and use the UnqualifiedPendingControlLog to store the results of an action.
	//
	// NOTE: Since mu is not held *during* an attempt to write to the backend, it is important
	// that this slice is only appended to and never reordered. The heartbeat logic relies upon the ability
	// to trim the first N elements upon successful write in order to avoid skipping and/or double-writing
	// a given entry.
	unqualifiedPendingControlLog []types.InstanceControlLogEntry

	// lastHeartbeat is the last observed heartbeat for this instance. This field is filled lazily and
	// will be nil if the instance only recently connected or joined. Operations that expect to be able to
	// observe the committed state of the instance control log should skip instances for which this field is nil.
	lastHeartbeat types.Instance

	// pingResponse stores information about last system clock request to propagate this data in the
	// next heartbeat request.
	pingResponse pingResponse

	// retryHeartbeat is set to true if an unexpected error is hit. We retry exactly once, closing
	// the stream if the retry does not succeede.
	retryHeartbeat bool
}

// InstanceStateRef is a helper used to present a copy of the public subset of instanceStateTracker. Used by
// the VisitInstanceState helper to show callers the current state without risking concurrency issues due
// to misuse.
type InstanceStateRef struct {
	QualifiedPendingControlLog   []types.InstanceControlLogEntry
	UnqualifiedPendingControlLog []types.InstanceControlLogEntry
	LastHeartbeat                types.Instance
}

// InstanceStateUpdate encodes additional pending control log entries that should be included in future heartbeats. Used by
// the VisitInstanceState helper to provide a mechanism of appending to the primary pending queues without risking
// concurrency issues due to misuse.
type InstanceStateUpdate struct {
	QualifiedPendingControlLog   []types.InstanceControlLogEntry
	UnqualifiedPendingControlLog []types.InstanceControlLogEntry
}

// VisitInstanceState provides a mechanism of viewing and potentially updating the instance control log of a
// given instance. The supplied closure is given a view of the most recent successful heartbeat, as well as
// any existing pending entries. It may then return additional pending entries. This method performs
// significant defensive copying, so care should be taken to limit its use.
func (h *upstreamHandle) VisitInstanceState(fn func(InstanceStateRef) InstanceStateUpdate) {
	h.stateTracker.mu.Lock()
	defer h.stateTracker.mu.Unlock()

	var ref InstanceStateRef

	// copy over last heartbeat if set
	if h.stateTracker.lastHeartbeat != nil {
		ref.LastHeartbeat = h.stateTracker.lastHeartbeat.Clone()
	}

	// copy over control log entries
	ref.QualifiedPendingControlLog = cloneAppendLog(ref.QualifiedPendingControlLog, h.stateTracker.qualifiedPendingControlLog...)
	ref.UnqualifiedPendingControlLog = cloneAppendLog(ref.UnqualifiedPendingControlLog, h.stateTracker.unqualifiedPendingControlLog...)

	// run closure
	update := fn(ref)

	// copy updates back into state tracker
	h.stateTracker.qualifiedPendingControlLog = cloneAppendLog(h.stateTracker.qualifiedPendingControlLog, update.QualifiedPendingControlLog...)
	h.stateTracker.unqualifiedPendingControlLog = cloneAppendLog(h.stateTracker.unqualifiedPendingControlLog, update.UnqualifiedPendingControlLog...)
}

// cloneAppendLog is a helper for performing deep copies of control log entries
func cloneAppendLog(log []types.InstanceControlLogEntry, entries ...types.InstanceControlLogEntry) []types.InstanceControlLogEntry {
	for _, entry := range entries {
		log = append(log, entry.Clone())
	}
	return log
}

// WithLock runs the provided closure with the tracker lock held.
func (i *instanceStateTracker) WithLock(fn func()) {
	i.mu.Lock()
	defer i.mu.Unlock()
	fn()
}

// nextHeartbeat calculates the next heartbeat value. *Must* be called only while lock is held.
func (i *instanceStateTracker) nextHeartbeat(now time.Time, hello *proto.UpstreamInventoryHello, authID string) (types.Instance, error) {
	var lastMeasurement *types.SystemClockMeasurement
	if !i.pingResponse.systemClock.IsZero() {
		lastMeasurement = &types.SystemClockMeasurement{
			ControllerSystemClock: i.pingResponse.controllerClock,
			SystemClock:           i.pingResponse.systemClock,
			RequestDuration:       i.pingResponse.reqDuration,
		}
	}

	services := make([]types.SystemRole, 0, len(hello.GetServices()))
	for _, s := range hello.GetServices() {
		services = append(services, types.SystemRole(s))
	}

	instance, err := types.NewInstance(hello.ServerID, types.InstanceSpecV1{
		Version:                 vc.Normalize(hello.Version),
		Services:                services,
		Hostname:                hello.Hostname,
		AuthID:                  authID,
		LastSeen:                now.UTC(),
		ExternalUpgrader:        hello.GetExternalUpgrader(),
		ExternalUpgraderVersion: vc.Normalize(hello.GetExternalUpgraderVersion()),
		LastMeasurement:         lastMeasurement,
		UpdaterInfo:             hello.GetUpdaterInfo(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// preserve control log entries from previous instance if present
	if i.lastHeartbeat != nil {
		instance.AppendControlLog(i.lastHeartbeat.GetControlLog()...)
	}

	if len(i.qualifiedPendingControlLog) > 0 {
		instance.AppendControlLog(i.qualifiedPendingControlLog...)
	}

	if len(i.unqualifiedPendingControlLog) > 0 {
		instance.AppendControlLog(i.unqualifiedPendingControlLog...)
	}

	return instance, nil
}

type upstreamHandle struct {
	client.UpstreamInventoryControlStream
	hello            *proto.UpstreamInventoryHello
	registrationTime time.Time

	agentMetadata atomic.Pointer[proto.UpstreamInventoryAgentMetadata]
	goodbye       atomic.Pointer[proto.UpstreamInventoryGoodbye]

	pingC chan pingRequest

	stateTracker instanceStateTracker

	// --- fields below this point only safe for access by handler goroutine

	// pings are in-flight pings to be multiplexed by ID.
	pings map[uint64]pendingPing

	// sshServer track ssh server details.
	sshServer *heartBeatInfo[*types.ServerV2]

	// appServers track app server details.
	appServers map[resourceKey]*heartBeatInfo[*types.AppServerV3]

	// databaseServers track database server details.
	databaseServers map[resourceKey]*heartBeatInfo[*types.DatabaseServerV3]

	// kubernetesServers track kubernetesServers server details.
	kubernetesServers map[resourceKey]*heartBeatInfo[*types.KubernetesServerV3]

	// appKeepAliveDelay is a multi-delay that controls the cadence of app server keepalive
	// operations. Note that this is not created automatically by newUpstreamHandle.
	appKeepAliveDelay *delay.Multi[resourceKey]

	// dbKeepAliveDelay is a multi-delay that controls the cadence of database server keepalive
	// operations. Note that this is not created automatically by newUpstreamHandle.
	dbKeepAliveDelay *delay.Multi[resourceKey]

	// kubeKeepAliveDelay is a multi-delay that controls the cadence of kubernetes server keepalive
	// operations. Note that this is not created automatically by newUpstreamHandle.
	kubeKeepAliveDelay *delay.Multi[resourceKey]
}

type resourceKey struct {
	hostID, name string
}

type heartBeatInfo[T any] struct {
	// resource is the most recently heartbeated item (if any).
	resource T
	// retryUpsert indicates that writing the lease failed and should be retried.
	retryUpsert bool
	// lease is used to keep alive a resource that was previously sent over a heartbeat.
	lease *types.KeepAlive
	// keepAliveErrs is a counter used to track the number of failed keepalives
	// with the above lease. too many failures clears the lease.
	keepAliveErrs int
}

func newUpstreamHandle(stream client.UpstreamInventoryControlStream, hello *proto.UpstreamInventoryHello, now time.Time) *upstreamHandle {
	return &upstreamHandle{
		UpstreamInventoryControlStream: stream,
		pingC:                          make(chan pingRequest),
		hello:                          hello,
		pings:                          make(map[uint64]pendingPing),
		registrationTime:               now,
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
	reqDuration     time.Duration
	systemClock     time.Time
	controllerClock time.Time
	err             error
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
		return rsp.reqDuration, rsp.err
	case <-h.Done():
		return 0, trace.Errorf("failed to recv upstream pong (stream closed)")
	case <-ctx.Done():
		return 0, trace.Errorf("failed to recv upstream ping: %v", ctx.Err())
	}
}

// Goodbye gets the cached upstream goodbye. Returns nil if downstream never sent a Goodbye.
// This is used to identify if the instance is terminating or being soft-reloaded.
func (h *upstreamHandle) Goodbye() *proto.UpstreamInventoryGoodbye {
	return h.goodbye.Load()
}

// setGoodbye sets the goodbye for the current handler.
func (h *upstreamHandle) setGoodbye(goodbye *proto.UpstreamInventoryGoodbye) {
	h.goodbye.Store(goodbye)
}

func (h *upstreamHandle) Hello() *proto.UpstreamInventoryHello {
	return h.hello
}

// RegistrationTime implements UpstreamHandle by returning the handle's creation timestamp.
func (h *upstreamHandle) RegistrationTime() time.Time {
	return h.registrationTime
}

// AgentMetadata returns the Agent's metadata (eg os, glibc version, install methods, teleport version).
func (h *upstreamHandle) AgentMetadata() *proto.UpstreamInventoryAgentMetadata {
	return h.agentMetadata.Load()
}

// setAgentMetadata sets the agent metadata for the current handler.
func (h *upstreamHandle) setAgentMetadata(agentMD *proto.UpstreamInventoryAgentMetadata) {
	h.agentMetadata.Store(agentMD)
}

// HasService is a helper for checking if a given service is associated with this
// stream.
func (h *upstreamHandle) HasService(service types.SystemRole) bool {
	return slices.Contains(h.hello.GetServices(), string(service))
}

// HasControlPlaneService implements UpstreamHandle and returns true if at
// least a control plane service is associated with this stream.
func (h *upstreamHandle) HasControlPlaneService() bool {
	for _, s := range h.hello.Services {
		if types.SystemRole(s).IsControlPlane() {
			return true
		}
	}
	return false
}

func (h *upstreamHandle) UpdateLabels(ctx context.Context, kind proto.LabelUpdateKind, labels map[string]string) error {
	req := &proto.DownstreamInventoryUpdateLabels{
		Kind:   kind,
		Labels: labels,
	}
	return trace.Wrap(h.Send(ctx, req))
}
