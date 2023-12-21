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
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Auth is an interface representing the subset of the auth API that must be made available
// to the controller in order for it to be able to handle control streams.
type Auth interface {
	UpsertNode(context.Context, types.Server) (*types.KeepAlive, error)

	KeepAliveServer(context.Context, types.KeepAlive) error

	UpsertInstance(ctx context.Context, instance types.Instance) error
}

type testEvent string

const (
	sshKeepAliveOk  testEvent = "ssh-keep-alive-ok"
	sshKeepAliveErr testEvent = "ssh-keep-alive-err"

	sshUpsertOk  testEvent = "ssh-upsert-ok"
	sshUpsertErr testEvent = "ssh-upsert-err"

	sshUpsertRetryOk  testEvent = "ssh-upsert-retry-ok"
	sshUpsertRetryErr testEvent = "ssh-upsert-retry-err"

	instanceHeartbeatOk  testEvent = "instance-heartbeat-ok"
	instanceHeartbeatErr testEvent = "instance-heartbeat-err"

	instanceCompareFailed testEvent = "instance-compare-failed"

	handlerStart = "handler-start"
	handlerClose = "handler-close"
)

// intervalKey is used to uniquely identify the subintervals registered with the interval.MultiInterval
// instance that we use for managing periodics associated with upstream handles.
type intervalKey int

const (
	instanceHeartbeatKey intervalKey = 1 + iota
	serverKeepAliveKey
)

// instanceHBStepSize is the step size used for the variable instance hearbteat duration. This value is
// basically arbitrary. It was selected because it produces a scaling curve that makes a fairly reasonable
// tradeoff between heartbeat availability and load scaling. See test coverage in the 'interval' package
// for a demonstration of the relationship between step sizes and interval/duration scaling.
const instanceHBStepSize = 1024

type controllerOptions struct {
	serverKeepAlive    time.Duration
	instanceHBInterval time.Duration
	testEvents         chan testEvent
	maxKeepAliveErrs   int
	authID             string
}

func (options *controllerOptions) SetDefaults() {
	baseKeepAlive := apidefaults.ServerKeepAliveTTL()
	if options.serverKeepAlive == 0 {
		// use 1.5x standard server keep alive since we use a jitter that
		// shortens the actual average interval.
		options.serverKeepAlive = baseKeepAlive + (baseKeepAlive / 2)
	}

	if options.instanceHBInterval == 0 {
		options.instanceHBInterval = apidefaults.MinInstanceHeartbeatInterval()
	}

	if options.maxKeepAliveErrs == 0 {
		// fail on third error. this is arbitrary, but feels reasonable since if we're on our
		// third error, 3-4m have gone by, so the problem is almost certainly persistent.
		options.maxKeepAliveErrs = 2
	}
}

type ControllerOption func(c *controllerOptions)

func WithAuthServerID(serverID string) ControllerOption {
	return func(opts *controllerOptions) {
		opts.authID = serverID
	}
}

func withServerKeepAlive(d time.Duration) ControllerOption {
	return func(opts *controllerOptions) {
		opts.serverKeepAlive = d
	}
}

func withInstanceHBInterval(d time.Duration) ControllerOption {
	return func(opts *controllerOptions) {
		opts.instanceHBInterval = d
	}
}

func withTestEventsChannel(ch chan testEvent) ControllerOption {
	return func(opts *controllerOptions) {
		opts.testEvents = ch
	}
}

// Controller manages the inventory control streams registered with a given auth instance. Incoming
// messages are processed by invoking the appropriate methods on the Auth interface.
type Controller struct {
	store                      *Store
	serviceCounter             *serviceCounter
	auth                       Auth
	authID                     string
	serverKeepAlive            time.Duration
	serverTTL                  time.Duration
	instanceTTL                time.Duration
	instanceHBEnabled          bool
	instanceHBVariableDuration *interval.VariableDuration
	maxKeepAliveErrs           int
	usageReporter              usagereporter.UsageReporter
	testEvents                 chan testEvent
	closeContext               context.Context
	cancel                     context.CancelFunc
}

// NewController sets up a new controller instance.
func NewController(auth Auth, usageReporter usagereporter.UsageReporter, opts ...ControllerOption) *Controller {
	var options controllerOptions
	for _, opt := range opts {
		opt(&options)
	}
	options.SetDefaults()

	instanceHBVariableDuration := interval.NewVariableDuration(interval.VariableDurationConfig{
		MinDuration: options.instanceHBInterval,
		MaxDuration: apidefaults.MaxInstanceHeartbeatInterval,
		Step:        instanceHBStepSize,
	})

	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		store:                      NewStore(),
		serviceCounter:             &serviceCounter{},
		serverKeepAlive:            options.serverKeepAlive,
		serverTTL:                  apidefaults.ServerAnnounceTTL,
		instanceTTL:                apidefaults.InstanceHeartbeatTTL,
		instanceHBEnabled:          !instanceHeartbeatsDisabledEnv(),
		instanceHBVariableDuration: instanceHBVariableDuration,
		maxKeepAliveErrs:           options.maxKeepAliveErrs,
		auth:                       auth,
		authID:                     options.authID,
		testEvents:                 options.testEvents,
		usageReporter:              usageReporter,
		closeContext:               ctx,
		cancel:                     cancel,
	}
}

// RegisterControlStream registers a new control stream with the controller.
func (c *Controller) RegisterControlStream(stream client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello) {
	// increment the concurrent connection counter that we use to calculate the variable
	// instance heartbeat duration.
	c.instanceHBVariableDuration.Inc()
	// set up ticker with instance HB sub-interval. additional sub-intervals are added as needed.
	// note that we are using fullJitter on the first duration to spread out initial instance heartbeats
	// as much as possible. this is intended to mitigate load spikes on auth restart, and is reasonably
	// safe to do since the instance resource is not directly relied upon for use of any particular teleport
	// service.
	ticker := interval.NewMulti(interval.SubInterval[intervalKey]{
		Key:              instanceHeartbeatKey,
		VariableDuration: c.instanceHBVariableDuration,
		FirstDuration:    fullJitter(c.instanceHBVariableDuration.Duration()),
		Jitter:           seventhJitter,
	})
	handle := newUpstreamHandle(stream, hello, ticker)
	c.store.Insert(handle)
	go c.handleControlStream(handle)
}

// GetControlStream gets a control stream for the given server ID if one exists (if multiple control streams
// exist one is selected pseudorandomly).
func (c *Controller) GetControlStream(serverID string) (handle UpstreamHandle, ok bool) {
	handle, ok = c.store.Get(serverID)
	return
}

// Iter iterates across all handles registered with this controller.
// note: if multiple handles are registered for a given server, only
// one handle is selected pseudorandomly to be observed.
func (c *Controller) Iter(fn func(UpstreamHandle)) {
	c.store.Iter(fn)
}

// ConnectedInstances gets the total number of connected instances. Note that this is the total number of
// *handles*, not the number of unique instances by id.
func (c *Controller) ConnectedInstances() int {
	return c.store.Len()
}

// ConnectedServiceCounts returns the number of each connected service seen in the inventory.
func (c *Controller) ConnectedServiceCounts() map[types.SystemRole]uint64 {
	return c.serviceCounter.counts()
}

// ConnectedServiceCount returns the number of a particular connected service in the inventory.
func (c *Controller) ConnectedServiceCount(systemRole types.SystemRole) uint64 {
	return c.serviceCounter.get(systemRole)
}

func (c *Controller) testEvent(event testEvent) {
	if c.testEvents == nil {
		return
	}
	c.testEvents <- event
}

// handleControlStream is the "main loop" of the upstream control stream. It handles incoming messages
// and also manages keepalives for previously heartbeated state.
func (c *Controller) handleControlStream(handle *upstreamHandle) {
	c.testEvent(handlerStart)

	for _, service := range handle.hello.Services {
		c.serviceCounter.increment(service)
	}

	defer func() {
		c.instanceHBVariableDuration.Dec()
		for _, service := range handle.hello.Services {
			c.serviceCounter.decrement(service)
		}
		c.store.Remove(handle)
		handle.Close() // no effect if CloseWithError was called below
		handle.ticker.Stop()
		c.testEvent(handlerClose)
	}()

	// keepAliveInit tracks wether or not we've initialized the server keepalive sub-interval. we do this lazily
	// upon receipt of the first heartbeat since not all servers send heartbeats.
	var keepAliveInit bool

	for {
		select {
		case msg := <-handle.Recv():
			switch m := msg.(type) {
			case proto.UpstreamInventoryHello:
				log.Warnf("Unexpected upstream hello on control stream of server %q.", handle.Hello().ServerID)
				handle.CloseWithError(trace.BadParameter("unexpected upstream hello"))
				return
			case proto.UpstreamInventoryAgentMetadata:
				c.handleAgentMetadata(handle, m)
			case proto.InventoryHeartbeat:
				if err := c.handleHeartbeatMsg(handle, m); err != nil {
					handle.CloseWithError(err)
					return
				}
				if !keepAliveInit {
					// this is the first heartbeat, so we need to initialize the keepalive sub-interval
					handle.ticker.Push(interval.SubInterval[intervalKey]{
						Key:           serverKeepAliveKey,
						Duration:      c.serverKeepAlive,
						FirstDuration: halfJitter(c.serverKeepAlive),
						Jitter:        seventhJitter,
					})
					keepAliveInit = true
				}
			case proto.UpstreamInventoryPong:
				c.handlePong(handle, m)
			default:
				log.Warnf("Unexpected upstream message type %T on control stream of server %q.", m, handle.Hello().ServerID)
				handle.CloseWithError(trace.BadParameter("unexpected upstream message type %T", m))
				return
			}
		case tick := <-handle.ticker.Next():
			switch tick.Key {
			case instanceHeartbeatKey:
				if err := c.heartbeatInstanceState(handle, tick.Time); err != nil {
					handle.CloseWithError(err)
					return
				}
			case serverKeepAliveKey:
				if err := c.keepAliveServer(handle, tick.Time); err != nil {
					handle.CloseWithError(err)
					return
				}
			default:
				log.Warnf("Unexpected sub-interval key '%v' in control stream handler of server %q (this is a bug).", tick.Key, handle.Hello().ServerID)
			}
		case req := <-handle.pingC:
			// pings require multiplexing, so we need to do the sending from this
			// goroutine rather than sending directly via the handle.
			if err := c.handlePingRequest(handle, req); err != nil {
				handle.CloseWithError(err)
				return
			}
		case <-handle.Done():
			return
		case <-c.closeContext.Done():
			return
		}
	}
}

// instanceHeartbeatsDisabledEnv checks if instance heartbeats have been explicitly disabled
// via environment variable.
func instanceHeartbeatsDisabledEnv() bool {
	return os.Getenv("TELEPORT_UNSTABLE_DISABLE_INSTANCE_HB") == "yes"
}

func (c *Controller) heartbeatInstanceState(handle *upstreamHandle, now time.Time) error {
	if !c.instanceHBEnabled {
		return nil
	}

	tracker := &handle.stateTracker

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// withoutLock is used to perform backend I/O outside of the tracker lock. use of this
	// helper rather than a manual lock/unlock is useful if we ever have a panic in backend logic,
	// since it preserves the original normal panic rather than causing the less helpful and unrecoverable
	// 'unlock of unlocked mutex' we would otherwise experience.
	withoutLock := func(fn func()) {
		tracker.mu.Unlock()
		defer tracker.mu.Lock()
		fn()
	}

	instance, err := tracker.nextHeartbeat(now, handle.Hello(), c.authID)
	if err != nil {
		log.Warnf("Failed to construct next heartbeat value for instance %q: %v (this is a bug)", handle.Hello().ServerID, err)
		return trace.Wrap(err)
	}

	// update expiry values using serverTTL as default resource/log ttl.
	instance.SyncLogAndResourceExpiry(c.instanceTTL)

	// perform backend I/O outside of lock
	withoutLock(func() {
		err = c.auth.UpsertInstance(c.closeContext, instance)
	})

	if err != nil {
		log.Warnf("Failed to hb instance %q: %v", handle.Hello().ServerID, err)
		c.testEvent(instanceHeartbeatErr)
		if !tracker.retryHeartbeat {
			// suppress failure and retry exactly once
			tracker.retryHeartbeat = true
			return nil
		}
		return trace.Wrap(err)
	}

	// update 'last heartbeat' value.
	tracker.lastHeartbeat = instance
	tracker.retryHeartbeat = false

	c.testEvent(instanceHeartbeatOk)

	return nil
}

func (c *Controller) handlePong(handle *upstreamHandle, msg proto.UpstreamInventoryPong) {
	pending, ok := handle.pings[msg.ID]
	if !ok {
		log.Warnf("Unexpected upstream pong from server %q (id=%d).", handle.Hello().ServerID, msg.ID)
		return
	}
	pending.rspC <- pingResponse{
		d: time.Since(pending.start),
	}
	delete(handle.pings, msg.ID)
}

func (c *Controller) handlePingRequest(handle *upstreamHandle, req pingRequest) error {
	ping := proto.DownstreamInventoryPing{
		ID: req.id,
	}
	start := time.Now()
	if err := handle.Send(c.closeContext, ping); err != nil {
		req.rspC <- pingResponse{
			err: err,
		}
		return trace.Wrap(err)
	}
	handle.pings[req.id] = pendingPing{
		start: start,
		rspC:  req.rspC,
	}
	return nil
}

func (c *Controller) handleHeartbeatMsg(handle *upstreamHandle, hb proto.InventoryHeartbeat) error {
	// XXX: when adding new services to the heartbeat logic, make sure to also update the
	// 'icsServiceToMetricName' mapping in auth/grpcserver.go in order to ensure that metrics
	// start counting the control stream as a registered keepalive stream for that service.

	if hb.SSHServer != nil {
		if err := c.handleSSHServerHB(handle, hb.SSHServer); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *Controller) handleSSHServerHB(handle *upstreamHandle, sshServer *types.ServerV2) error {
	// the auth layer verifies that a stream's hello message matches the identity and capabilities of the
	// client cert. after that point it is our responsibility to ensure that heartbeated information is
	// consistent with the identity and capabilities claimed in the initial hello.
	if !handle.HasService(types.RoleNode) {
		return trace.AccessDenied("control stream not configured to support ssh server heartbeats")
	}
	if sshServer.GetName() != handle.Hello().ServerID {
		return trace.AccessDenied("incorrect ssh server ID (expected %q, got %q)", handle.Hello().ServerID, sshServer.GetName())
	}

	// if a peer address is available in the context, use it to override zero-value addresses from
	// the server heartbeat.
	if handle.PeerAddr() != "" {
		sshServer.SetAddr(utils.ReplaceLocalhost(sshServer.GetAddr(), handle.PeerAddr()))
	}

	now := time.Now()

	sshServer.SetExpiry(now.Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertNode(c.closeContext, sshServer)
	if err == nil {
		c.testEvent(sshUpsertOk)
		// store the new lease and reset retry state
		handle.sshServerLease = lease
		handle.retrySSHServerUpsert = false
	} else {
		c.testEvent(sshUpsertErr)
		log.Warnf("Failed to upsert ssh server %q on heartbeat: %v.", handle.Hello().ServerID, err)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		handle.sshServerLease = nil
		handle.retrySSHServerUpsert = true
	}
	handle.sshServer = sshServer
	return nil
}

func (c *Controller) handleAgentMetadata(handle *upstreamHandle, m proto.UpstreamInventoryAgentMetadata) {
	handle.SetAgentMetadata(m)

	svcs := make([]string, 0, len(handle.Hello().Services))
	for _, svc := range handle.Hello().Services {
		svcs = append(svcs, strings.ToLower(svc.String()))
	}

	c.usageReporter.AnonymizeAndSubmit(&usagereporter.AgentMetadataEvent{
		Version:               handle.Hello().Version,
		HostId:                handle.Hello().ServerID,
		Services:              svcs,
		Os:                    m.OS,
		OsVersion:             m.OSVersion,
		HostArchitecture:      m.HostArchitecture,
		GlibcVersion:          m.GlibcVersion,
		InstallMethods:        m.InstallMethods,
		ContainerRuntime:      m.ContainerRuntime,
		ContainerOrchestrator: m.ContainerOrchestrator,
		CloudEnvironment:      m.CloudEnvironment,
		ExternalUpgrader:      handle.Hello().ExternalUpgrader,
	})
}

func (c *Controller) keepAliveServer(handle *upstreamHandle, now time.Time) error {
	if handle.sshServerLease != nil {
		lease := *handle.sshServerLease
		lease.Expires = now.Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(sshKeepAliveErr)
			handle.sshServerKeepAliveErrs++
			shouldClose := handle.sshServerKeepAliveErrs > c.maxKeepAliveErrs

			log.Warnf("Failed to keep alive ssh server %q: %v (count=%d, closing=%v).", handle.Hello().ServerID, err, handle.sshServerKeepAliveErrs, shouldClose)

			if shouldClose {
				return trace.Errorf("failed to keep alive ssh server: %v", err)
			}
		} else {
			c.testEvent(sshKeepAliveOk)
		}
	} else if handle.retrySSHServerUpsert {
		handle.sshServer.SetExpiry(time.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertNode(c.closeContext, handle.sshServer)
		if err != nil {
			c.testEvent(sshUpsertRetryErr)
			log.Warnf("Failed to upsert ssh server %q on retry: %v.", handle.Hello().ServerID, err)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don't bother
			// attempting a third time.
			return trace.Errorf("failed to upsert ssh server on retry: %v", err)
		}
		c.testEvent(sshUpsertRetryOk)
		handle.sshServerLease = lease
		handle.retrySSHServerUpsert = false
	}

	return nil
}

// Close terminates all control streams registered with this controller. Control streams
// registered after Close() is called are closed immediately.
func (c *Controller) Close() error {
	c.cancel()
	return nil
}
