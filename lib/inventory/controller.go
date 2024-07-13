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
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Auth is an interface representing the subset of the auth API that must be made available
// to the controller in order for it to be able to handle control streams.
type Auth interface {
	UpsertNode(context.Context, types.Server) (*types.KeepAlive, error)
	UpsertApplicationServer(context.Context, types.AppServer) (*types.KeepAlive, error)
	DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error

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

	appKeepAliveOk  testEvent = "app-keep-alive-ok"
	appKeepAliveErr testEvent = "app-keep-alive-err"
	appKeepAliveDel testEvent = "app-keep-alive-del"

	appUpsertOk  testEvent = "app-upsert-ok"
	appUpsertErr testEvent = "app-upsert-err"

	appUpsertRetryOk  testEvent = "app-upsert-retry-ok"
	appUpsertRetryErr testEvent = "app-upsert-retry-err"

	instanceHeartbeatOk  testEvent = "instance-heartbeat-ok"
	instanceHeartbeatErr testEvent = "instance-heartbeat-err"

	instanceCompareFailed testEvent = "instance-compare-failed"

	handlerStart = "handler-start"
	handlerClose = "handler-close"

	keepAliveTick = "keep-alive-tick"
)

// intervalKey is used to uniquely identify the subintervals registered with the interval.MultiInterval
// instance that we use for managing periodics associated with upstream handles.
type intervalKey int

const (
	instanceHeartbeatKey intervalKey = 1 + iota
	serverKeepAliveKey
)

type controllerOptions struct {
	serverKeepAlive    time.Duration
	instanceHBInterval time.Duration
	testEvents         chan testEvent
	maxKeepAliveErrs   int
	authID             string
	instanceHeartbeats bool
	onConnectFunc      func(string)
	onDisconnectFunc   func(string)
}

func (options *controllerOptions) SetDefaults() {
	baseKeepAlive := apidefaults.ServerKeepAliveTTL()
	if options.serverKeepAlive == 0 {
		// use 1.5x standard server keep alive since we use a jitter that
		// shortens the actual average interval.
		options.serverKeepAlive = baseKeepAlive + (baseKeepAlive / 2)
	}

	if options.instanceHBInterval == 0 {
		// instance heartbeat interval defaults to ~9x the server keepalive ttl. The instance
		// resource is intended to be a low-impact and lazily intitialized value. We use a multiple
		// of server keepalive rather than a constant because the base server keepalive value is
		// dynamically modified during tests in order to create a teleport cluster with a generally
		// higher presence tick-rate.
		options.instanceHBInterval = baseKeepAlive * 9
	}

	if options.maxKeepAliveErrs == 0 {
		// fail on third error. this is arbitrary, but feels reasonable since if we're on our
		// third error, 3-4m have gone by, so the problem is almost certainly persistent.
		options.maxKeepAliveErrs = 2
	}

	if options.onConnectFunc == nil {
		options.onConnectFunc = func(s string) {}
	}

	if options.onDisconnectFunc == nil {
		options.onDisconnectFunc = func(s string) {}
	}
}

type ControllerOption func(c *controllerOptions)

func WithAuthServerID(serverID string) ControllerOption {
	return func(opts *controllerOptions) {
		opts.authID = serverID
	}
}

// WithInstanceHeartbeats enables heartbeating of the instance resource.
func WithInstanceHeartbeats(enabled bool) ControllerOption {
	return func(opts *controllerOptions) {
		opts.instanceHeartbeats = enabled
	}
}

// WithOnConnect sets a function to be called every time a new
// instance connects via the inventory control stream. The value
// provided to the callback is the keep alive type of the connected
// resource. The callback should return quickly so as not to prevent
// processing of heartbeats.
func WithOnConnect(f func(heartbeatKind string)) ControllerOption {
	return func(opts *controllerOptions) {
		opts.onConnectFunc = f
	}
}

// WithOnDisconnect sets a function to be called every time an existing
// instance disconnects from the inventory control stream. The value
// provided to the callback is the keep alive type of the disconnected
// resource. The callback should return quickly so as not to prevent
// processing of heartbeats.
func WithOnDisconnect(f func(heartbeatKind string)) ControllerOption {
	return func(opts *controllerOptions) {
		opts.onDisconnectFunc = f
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
	store              *Store
	serviceCounter     *serviceCounter
	auth               Auth
	authID             string
	serverKeepAlive    time.Duration
	serverTTL          time.Duration
	instanceTTL        time.Duration
	instanceHBEnabled  bool
	instanceHBInterval time.Duration
	maxKeepAliveErrs   int
	usageReporter      usagereporter.UsageReporter
	testEvents         chan testEvent
	onConnectFunc      func(string)
	onDisconnectFunc   func(string)
	closeContext       context.Context
	cancel             context.CancelFunc

	firstInstanceHBJitter retryutils.Jitter
}

// NewController sets up a new controller instance.
func NewController(auth Auth, usageReporter usagereporter.UsageReporter, opts ...ControllerOption) *Controller {
	var options controllerOptions
	for _, opt := range opts {
		opt(&options)
	}
	options.SetDefaults()

	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		store:              NewStore(),
		serviceCounter:     &serviceCounter{},
		serverKeepAlive:    options.serverKeepAlive,
		serverTTL:          apidefaults.ServerAnnounceTTL,
		instanceTTL:        apidefaults.ServerAnnounceTTL * 2,
		instanceHBEnabled:  options.instanceHeartbeats || instanceHeartbeatsEnabledEnv(),
		instanceHBInterval: options.instanceHBInterval,
		maxKeepAliveErrs:   options.maxKeepAliveErrs,
		auth:               auth,
		authID:             options.authID,
		testEvents:         options.testEvents,
		usageReporter:      usageReporter,
		onConnectFunc:      options.onConnectFunc,
		onDisconnectFunc:   options.onDisconnectFunc,
		closeContext:       ctx,
		cancel:             cancel,

		// apply a ramping modifier to the first 600 incoming connections s.t. their initial instance hbs happen
		// sooner (the first 100 within the firt 1m, the first 300 within the first 5m, and so on). Long-term we want
		// to have a dynamically scaling hb rate, but this is a decent way to get some of the benefits of that in a
		// simple manner (namely, improving time-to-visibility in small clusters).
		firstInstanceHBJitter: ninthRampingJitter(600, fullJitter),
	}
}

// RegisterControlStream registers a new control stream with the controller.
func (c *Controller) RegisterControlStream(stream client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello) {
	// set up ticker with instance HB sub-interval. additional sub-intervals are added as needed.
	// note that we are using fullJitter on the first duration to spread out initial instance heartbeats
	// as much as possible. this is intended to mitigate load spikes on auth restart, and is reasonably
	// safe to do since the instance resource is not directly relied upon for use of any particular teleport
	// service.
	ticker := interval.NewMulti(interval.SubInterval[intervalKey]{
		Key:           instanceHeartbeatKey,
		Duration:      c.instanceHBInterval,
		FirstDuration: c.firstInstanceHBJitter(c.instanceHBInterval),
		Jitter:        seventhJitter,
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
		if handle.goodbye.GetDeleteResources() {
			log.WithField("apps", len(handle.appServers)).Debug("Cleaning up resources in response to instance termination")
			for _, app := range handle.appServers {
				if err := c.auth.DeleteApplicationServer(c.closeContext, apidefaults.Namespace, app.resource.GetHostID(), app.resource.GetName()); err != nil && !trace.IsNotFound(err) {
					log.Warnf("Failed to remove app server %q on termination: %v.", handle.Hello().ServerID, err)
				}
			}
		}

		for _, service := range handle.hello.Services {
			c.serviceCounter.decrement(service)
		}
		c.store.Remove(handle)
		handle.Close() // no effect if CloseWithError was called below
		handle.ticker.Stop()

		if handle.sshServer != nil {
			c.onDisconnectFunc(constants.KeepAliveNode)
		}

		for range handle.appServers {
			c.onDisconnectFunc(constants.KeepAliveApp)
		}

		clear(handle.appServers)
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
			case proto.UpstreamInventoryGoodbye:
				handle.goodbye = m
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

// instanceHeartbeatsEnabledEnv checks if instance heartbeats have been enabled via unstable
// env var. This is a placeholder until we've sufficiently improved instance heartbeat
// backend performance to reenable them by default.
// TODO(tross,fspmarshal): remove this once issues with etcd stability are resolved.
func instanceHeartbeatsEnabledEnv() bool {
	return os.Getenv("TELEPORT_UNSTABLE_ENABLE_INSTANCE_HB") == "yes"
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

	if hb.AppServer != nil {
		if err := c.handleAppServerHB(handle, hb.AppServer); err != nil {
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

	if handle.sshServer == nil {
		c.onConnectFunc(constants.KeepAliveNode)
		handle.sshServer = &heartBeatInfo[*types.ServerV2]{}
	}

	now := time.Now()

	sshServer.SetExpiry(now.Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertNode(c.closeContext, sshServer)
	if err == nil {
		c.testEvent(sshUpsertOk)
		// store the new lease and reset retry state
		handle.sshServer.lease = lease
		handle.sshServer.retryUpsert = false
	} else {
		c.testEvent(sshUpsertErr)
		log.Warnf("Failed to upsert ssh server %q on heartbeat: %v.", handle.Hello().ServerID, err)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		handle.sshServer.lease = nil
		handle.sshServer.retryUpsert = true
	}
	handle.sshServer.resource = sshServer
	return nil
}

func (c *Controller) handleAppServerHB(handle *upstreamHandle, appServer *types.AppServerV3) error {
	// the auth layer verifies that a stream's hello message matches the identity and capabilities of the
	// client cert. after that point it is our responsibility to ensure that heartbeated information is
	// consistent with the identity and capabilities claimed in the initial hello.
	if !handle.HasService(types.RoleApp) {
		return trace.AccessDenied("control stream not configured to support app server heartbeats")
	}
	if appServer.GetHostID() != handle.Hello().ServerID {
		return trace.AccessDenied("incorrect app server ID (expected %q, got %q)", handle.Hello().ServerID, appServer.GetHostID())
	}

	if handle.appServers == nil {
		handle.appServers = make(map[appServerKey]*heartBeatInfo[*types.AppServerV3])
	}

	appKey := appServerKey{hostID: appServer.GetHostID(), name: appServer.GetApp().GetName()}

	if _, ok := handle.appServers[appKey]; !ok {
		c.onConnectFunc(constants.KeepAliveApp)
		handle.appServers[appKey] = &heartBeatInfo[*types.AppServerV3]{}
	}

	now := time.Now()

	appServer.SetExpiry(now.Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertApplicationServer(c.closeContext, appServer)
	if err == nil {
		c.testEvent(appUpsertOk)
		// store the new lease and reset retry state
		srv := handle.appServers[appKey]
		srv.lease = lease
		srv.retryUpsert = false
		srv.resource = appServer
	} else {
		c.testEvent(appUpsertErr)
		log.Warnf("Failed to upsert app server %q on heartbeat: %v.", handle.Hello().ServerID, err)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		srv := handle.appServers[appKey]
		srv.lease = nil
		srv.retryUpsert = true
		srv.resource = appServer
	}
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
	// always fire off 'tick' event after keepalive processing to ensure
	// that waiting for N ticks maps intuitively to waiting for N keepalive
	// processing steps.
	defer c.testEvent(keepAliveTick)
	if err := c.keepAliveSSHServer(handle, now); err != nil {
		return trace.Wrap(err)
	}

	if err := c.keepAliveAppServer(handle, now); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Controller) keepAliveAppServer(handle *upstreamHandle, now time.Time) error {
	if handle.appServers == nil {
		return nil
	}

	for name, srv := range handle.appServers {
		if srv.lease != nil {
			lease := *srv.lease
			lease.Expires = now.Add(c.serverTTL).UTC()
			if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
				c.testEvent(appKeepAliveErr)

				srv.keepAliveErrs++
				handle.appServers[name] = srv
				shouldRemove := srv.keepAliveErrs > c.maxKeepAliveErrs
				log.Warnf("Failed to keep alive app server %q: %v (count=%d, removing=%v).", handle.Hello().ServerID, err, srv.keepAliveErrs, shouldRemove)

				if shouldRemove {
					c.testEvent(appKeepAliveDel)
					delete(handle.appServers, name)
				}
			} else {
				srv.keepAliveErrs = 0
				c.testEvent(appKeepAliveOk)
			}
		} else if srv.retryUpsert {
			srv.resource.SetExpiry(time.Now().Add(c.serverTTL).UTC())
			lease, err := c.auth.UpsertApplicationServer(c.closeContext, srv.resource)
			if err != nil {
				c.testEvent(appUpsertRetryErr)
				log.Warnf("Failed to upsert app server %q on retry: %v.", handle.Hello().ServerID, err)
				// since this is retry-specific logic, an error here means that upsert failed twice in
				// a row. Missing upserts is more problematic than missing keepalives so we don'resource bother
				// attempting a third time.
				return trace.Errorf("failed to upsert app server on retry: %v", err)
			}
			c.testEvent(appUpsertRetryOk)

			srv.lease = lease
			srv.retryUpsert = false
		}
	}

	return nil
}

func (c *Controller) keepAliveSSHServer(handle *upstreamHandle, now time.Time) error {
	if handle.sshServer == nil {
		return nil
	}

	if handle.sshServer.lease != nil {
		lease := *handle.sshServer.lease
		lease.Expires = now.Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(sshKeepAliveErr)
			handle.sshServer.keepAliveErrs++
			shouldClose := handle.sshServer.keepAliveErrs > c.maxKeepAliveErrs

			log.Warnf("Failed to keep alive ssh server %q: %v (count=%d, closing=%v).", handle.Hello().ServerID, err, handle.sshServer.keepAliveErrs, shouldClose)

			if shouldClose {
				return trace.Errorf("failed to keep alive ssh server: %v", err)
			}
		} else {
			handle.sshServer.keepAliveErrs = 0
			c.testEvent(sshKeepAliveOk)
		}
	} else if handle.sshServer.retryUpsert {
		handle.sshServer.resource.SetExpiry(time.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertNode(c.closeContext, handle.sshServer.resource)
		if err != nil {
			c.testEvent(sshUpsertRetryErr)
			log.Warnf("Failed to upsert ssh server %q on retry: %v.", handle.Hello().ServerID, err)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don'resource bother
			// attempting a third time.
			return trace.Errorf("failed to upsert ssh server on retry: %v", err)
		}
		c.testEvent(sshUpsertRetryOk)
		handle.sshServer.lease = lease
		handle.sshServer.retryUpsert = false
	}

	return nil
}

// Close terminates all control streams registered with this controller. Control streams
// registered after Close() is called are closed immediately.
func (c *Controller) Close() error {
	c.cancel()
	return nil
}
