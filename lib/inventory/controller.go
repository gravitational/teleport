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
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/inventory/internal/delay"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Auth is an interface representing the subset of the auth API that must be made available
// to the controller in order for it to be able to handle control streams.
type Auth interface {
	UpsertNode(context.Context, types.Server) (*types.KeepAlive, error)

	UpsertApplicationServer(context.Context, types.AppServer) (*types.KeepAlive, error)
	DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error

	UpsertDatabaseServer(context.Context, types.DatabaseServer) (*types.KeepAlive, error)
	DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error

	UpsertKubernetesServer(context.Context, types.KubeServer) (*types.KeepAlive, error)
	DeleteKubernetesServer(ctx context.Context, hostID, name string) error

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

	dbKeepAliveOk  testEvent = "db-keep-alive-ok"
	dbKeepAliveErr testEvent = "db-keep-alive-err"
	dbKeepAliveDel testEvent = "db-keep-alive-del"

	dbUpsertOk  testEvent = "db-upsert-ok"
	dbUpsertErr testEvent = "db-upsert-err"

	dbUpsertRetryOk  testEvent = "db-upsert-retry-ok"
	dbUpsertRetryErr testEvent = "db-upsert-retry-err"

	kubeKeepAliveOk  testEvent = "kube-keep-alive-ok"
	kubeKeepAliveErr testEvent = "kube-keep-alive-err"
	kubeKeepAliveDel testEvent = "kube-keep-alive-del"

	kubeUpsertOk  testEvent = "kube-upsert-ok"
	kubeUpsertErr testEvent = "kube-upsert-err"

	kubeUpsertRetryOk  testEvent = "kube-upsert-retry-ok"
	kubeUpsertRetryErr testEvent = "kube-upsert-retry-err"

	instanceHeartbeatOk  testEvent = "instance-heartbeat-ok"
	instanceHeartbeatErr testEvent = "instance-heartbeat-err"

	pongOk testEvent = "pong-ok"

	instanceCompareFailed testEvent = "instance-compare-failed"

	handlerStart = "handler-start"
	handlerClose = "handler-close"

	keepAliveSSHTick      = "keep-alive-ssh-tick"
	keepAliveAppTick      = "keep-alive-app-tick"
	keepAliveDatabaseTick = "keep-alive-db-tick"
	keepAliveKubeTick     = "keep-alive-kube-tick"
)

// heartbeatStepSize is the step size used for the variable heartbeat intervals.
// This value is basically arbitrary. It was selected because it produces a
// scaling curve that makes a fairly reasonable tradeoff between heartbeat
// availability and load scaling. See test coverage in the 'interval' package
// for a demonstration of the relationship between step sizes and
// interval/duration scaling.
const heartbeatStepSize = 1024

type controllerOptions struct {
	serverKeepAlive    time.Duration
	instanceHBInterval time.Duration
	testEvents         chan testEvent
	maxKeepAliveErrs   int
	authID             string
	onConnectFunc      func(string)
	onDisconnectFunc   func(string, int)
	cleanupLimiter     *rate.Limiter
	cleanupTimeout     time.Duration
	clock              clockwork.Clock
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

	if options.onConnectFunc == nil {
		options.onConnectFunc = func(string) {}
	}

	if options.onDisconnectFunc == nil {
		options.onDisconnectFunc = func(string, int) {}
	}

	if options.cleanupLimiter == nil {
		// limit resource cleanup writes to 128 per second to reduce the chances that
		// agents with very large resource counts cause throttling on graceful disconnect.
		options.cleanupLimiter = rate.NewLimiter(rate.Every(time.Second/128), 1)
	}

	if options.cleanupTimeout == 0 {
		options.cleanupTimeout = time.Second * 30
	}

	if options.clock == nil {
		options.clock = clockwork.NewRealClock()
	}
}

type ControllerOption func(c *controllerOptions)

func WithAuthServerID(serverID string) ControllerOption {
	return func(opts *controllerOptions) {
		opts.authID = serverID
	}
}

// WithOnConnect sets a function to be called every time a new
// heartbeat connects via the inventory control stream. The value
// provided to the callback is the keep alive type of the connected
// resource. The callback should return quickly so as not to prevent
// processing of heartbeats.
func WithOnConnect(f func(heartbeatKind string)) ControllerOption {
	return func(opts *controllerOptions) {
		opts.onConnectFunc = f
	}
}

// WithOnDisconnect sets a function to be called every time an existing heartbeat
// disconnects from the inventory control stream. The values provided to the
// callback are the keep alive type of the disconnected resource, as well as a
// count of how many resources disconnected at once. The callback should return
// quickly so as not to prevent processing of heartbeats.
func WithOnDisconnect(f func(heartbeatKind string, amount int)) ControllerOption {
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

// WithClock sets the clock for the controller to have a general clock configuration.
func WithClock(clock clockwork.Clock) ControllerOption {
	return func(opts *controllerOptions) {
		opts.clock = clock
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
	sshHBVariableDuration      *interval.VariableDuration
	appHBVariableDuration      *interval.VariableDuration
	dbHBVariableDuration       *interval.VariableDuration
	kubeHBVariableDuration     *interval.VariableDuration
	maxKeepAliveErrs           int
	usageReporter              usagereporter.UsageReporter
	testEvents                 chan testEvent
	onConnectFunc              func(string)
	onDisconnectFunc           func(string, int)
	cleanupLimiter             *rate.Limiter
	cleanupTimeout             time.Duration
	clock                      clockwork.Clock
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
		Step:        heartbeatStepSize,
	})

	var (
		sshHBVariableDuration  *interval.VariableDuration
		appHBVariableDuration  *interval.VariableDuration
		dbHBVariableDuration   *interval.VariableDuration
		kubeHBVariableDuration *interval.VariableDuration
	)
	serverTTL := apidefaults.ServerAnnounceTTL
	if !variableRateHeartbeatsDisabledEnv() {
		// by default, heartbeats will scale from 1.5 to 6 minutes, and will
		// have a TTL of 15 minutes
		serverTTL = apidefaults.ServerAnnounceTTL * 3 / 2
		sshHBVariableDuration = interval.NewVariableDuration(interval.VariableDurationConfig{
			MinDuration: options.serverKeepAlive,
			MaxDuration: options.serverKeepAlive * 4,
			Step:        heartbeatStepSize,
		})
		appHBVariableDuration = interval.NewVariableDuration(interval.VariableDurationConfig{
			MinDuration: options.serverKeepAlive,
			MaxDuration: options.serverKeepAlive * 4,
			Step:        heartbeatStepSize,
		})
		dbHBVariableDuration = interval.NewVariableDuration(interval.VariableDurationConfig{
			MinDuration: options.serverKeepAlive,
			MaxDuration: options.serverKeepAlive * 4,
			Step:        heartbeatStepSize,
		})
		kubeHBVariableDuration = interval.NewVariableDuration(interval.VariableDurationConfig{
			MinDuration: options.serverKeepAlive,
			MaxDuration: options.serverKeepAlive * 4,
			Step:        heartbeatStepSize,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		store:                      NewStore(),
		serviceCounter:             &serviceCounter{},
		serverKeepAlive:            options.serverKeepAlive,
		serverTTL:                  serverTTL,
		instanceTTL:                apidefaults.InstanceHeartbeatTTL,
		instanceHBEnabled:          !instanceHeartbeatsDisabledEnv(),
		instanceHBVariableDuration: instanceHBVariableDuration,
		sshHBVariableDuration:      sshHBVariableDuration,
		appHBVariableDuration:      appHBVariableDuration,
		dbHBVariableDuration:       dbHBVariableDuration,
		kubeHBVariableDuration:     kubeHBVariableDuration,
		maxKeepAliveErrs:           options.maxKeepAliveErrs,
		auth:                       auth,
		authID:                     options.authID,
		testEvents:                 options.testEvents,
		usageReporter:              usageReporter,
		onConnectFunc:              options.onConnectFunc,
		onDisconnectFunc:           options.onDisconnectFunc,
		cleanupLimiter:             options.cleanupLimiter,
		cleanupTimeout:             options.cleanupTimeout,
		clock:                      options.clock,
		closeContext:               ctx,
		cancel:                     cancel,
	}
}

// RegisterControlStream registers a new control stream with the controller.
func (c *Controller) RegisterControlStream(stream client.UpstreamInventoryControlStream, hello *proto.UpstreamInventoryHello) {
	handle := newUpstreamHandle(stream, hello, c.clock.Now())
	c.store.Insert(handle)

	// Increment the concurrent connection counter that we use to calculate the
	// variable instance heartbeat duration. To make the behavior more easily
	// testable, we increment it here and we decrement it before closing the
	// stream in handleControlStream.
	c.instanceHBVariableDuration.Inc()
	go c.handleControlStream(handle)
}

// GetControlStream gets a control stream for the given server ID if one exists (if multiple control streams
// exist one is selected pseudorandomly).
func (c *Controller) GetControlStream(serverID string) (handle UpstreamHandle, ok bool) {
	handle, ok = c.store.Get(serverID)
	return
}

// UniqueHandles iterates across unique handles registered with this controller.
// If multiple handles are registered for a given server, only
// one handle is selected pseudorandomly to be observed.
func (c *Controller) UniqueHandles(fn func(UpstreamHandle)) {
	c.store.UniqueHandles(fn)
}

// AllHandles iterates across all handles registered with this
// controller. If multiple handles are registered for a given server,
// all of them will be observed.
func (c *Controller) AllHandles(fn func(UpstreamHandle)) {
	c.store.AllHandles(fn)
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

	// Note that we are using fullJitter on the first duration to spread out
	// initial instance heartbeats as much as possible. This is intended to
	// mitigate load spikes on auth restart, and is reasonably safe to do since
	// the instance resource is not directly relied upon for use of any
	// particular Teleport service.
	firstDuration := retryutils.FullJitter(c.instanceHBVariableDuration.Duration())
	instanceHeartbeatDelay := delay.New(delay.Params{
		FirstInterval:    firstDuration,
		VariableInterval: c.instanceHBVariableDuration,
		Jitter:           retryutils.SeventhJitter,
	})
	defer instanceHeartbeatDelay.Stop()
	timeReconciliationDelay := delay.New(delay.Params{
		FirstInterval:    firstDuration / 2,
		VariableInterval: c.instanceHBVariableDuration,
		Jitter:           retryutils.SeventhJitter,
	})
	defer timeReconciliationDelay.Stop()

	// these delays are lazily initialized upon receipt of the first heartbeat
	// since not all servers send all heartbeats
	var sshKeepAliveDelay *delay.Delay

	defer func() {
		// this is a function expression because the variables are initialized
		// later and we want to call Stop on the initialized value (if any)
		sshKeepAliveDelay.Stop()
		handle.appKeepAliveDelay.Stop()
		handle.dbKeepAliveDelay.Stop()
		handle.kubeKeepAliveDelay.Stop()
	}()

	for _, service := range handle.hello.Services {
		c.serviceCounter.increment(types.SystemRole(service))
	}

	defer func() {
		if handle.Goodbye().GetDeleteResources() {
			c.doResourceCleanup(handle)
		}

		c.instanceHBVariableDuration.Dec()
		for _, service := range handle.hello.Services {
			c.serviceCounter.decrement(types.SystemRole(service))
		}
		c.store.Remove(handle)
		handle.Close() // no effect if CloseWithError was called below

		if handle.sshServer != nil {
			c.onDisconnectFunc(constants.KeepAliveNode, 1)
			if c.sshHBVariableDuration != nil {
				c.sshHBVariableDuration.Dec()
			}
			handle.sshServer = nil
		}

		if len(handle.appServers) > 0 {
			c.onDisconnectFunc(constants.KeepAliveApp, len(handle.appServers))
			if c.appHBVariableDuration != nil {
				c.appHBVariableDuration.Add(-len(handle.appServers))
			}
			clear(handle.appServers)
		}

		if len(handle.databaseServers) > 0 {
			c.onDisconnectFunc(constants.KeepAliveDatabase, len(handle.databaseServers))
			if c.dbHBVariableDuration != nil {
				c.dbHBVariableDuration.Add(-len(handle.databaseServers))
			}
			clear(handle.databaseServers)
		}

		if len(handle.kubernetesServers) > 0 {
			c.onDisconnectFunc(constants.KeepAliveKube, len(handle.kubernetesServers))
			if c.kubeHBVariableDuration != nil {
				c.kubeHBVariableDuration.Add(-len(handle.kubernetesServers))
			}
			clear(handle.kubernetesServers)
		}

		c.testEvent(handlerClose)
	}()

	for {
		select {
		case msg := <-handle.Recv():
			switch m := msg.(type) {
			case *proto.UpstreamInventoryHello:
				slog.WarnContext(c.closeContext, "Unexpected upstream hello on control stream of server", "server_id", handle.Hello().ServerID)
				handle.CloseWithError(trace.BadParameter("unexpected upstream hello"))
				return
			case *proto.UpstreamInventoryAgentMetadata:
				c.handleAgentMetadata(handle, m)
			case *proto.InventoryHeartbeat:
				// XXX: when adding new services to the heartbeat logic, make
				// sure to also update the 'icsServiceToMetricName' mapping in
				// auth/grpcserver.go in order to ensure that metrics start
				// counting the control stream as a registered keepalive stream
				// for that service.

				if m.SSHServer != nil {
					// we initialize sshKeepAliveDelay before calling
					// handleSSHServerHB unlike the other heartbeat types
					// because handleSSHServerHB needs the delay to reset it
					// after an announce, including the first one
					if sshKeepAliveDelay == nil {
						sshKeepAliveDelay = c.createKeepAliveDelay(c.sshHBVariableDuration)
					}

					if err := c.handleSSHServerHB(handle, m.SSHServer, sshKeepAliveDelay); err != nil {
						handle.CloseWithError(trace.Wrap(err))
						return
					}
				}

				if m.AppServer != nil {
					if handle.appKeepAliveDelay == nil {
						handle.appKeepAliveDelay = c.createKeepAliveMultiDelay(c.appHBVariableDuration)
					}

					if err := c.handleAppServerHB(handle, m.AppServer); err != nil {
						handle.CloseWithError(err)
						return
					}
				}

				if m.DatabaseServer != nil {
					if handle.dbKeepAliveDelay == nil {
						handle.dbKeepAliveDelay = c.createKeepAliveMultiDelay(c.dbHBVariableDuration)
					}

					if err := c.handleDatabaseServerHB(handle, m.DatabaseServer); err != nil {
						handle.CloseWithError(err)
						return
					}
				}

				if m.KubernetesServer != nil {
					if handle.kubeKeepAliveDelay == nil {
						handle.kubeKeepAliveDelay = c.createKeepAliveMultiDelay(c.kubeHBVariableDuration)
					}

					if err := c.handleKubernetesServerHB(handle, m.KubernetesServer); err != nil {
						handle.CloseWithError(err)
						return
					}
				}

			case *proto.UpstreamInventoryPong:
				c.handlePong(handle, m)
			case *proto.UpstreamInventoryGoodbye:
				handle.setGoodbye(m)
			default:
				slog.WarnContext(c.closeContext, "Unexpected upstream message type on control stream",
					"message_type", logutils.TypeAttr(m),
					"server_id", handle.Hello().ServerID,
				)
				handle.CloseWithError(trace.BadParameter("unexpected upstream message type %T", m))
				return
			}
		case now := <-instanceHeartbeatDelay.Elapsed():
			instanceHeartbeatDelay.Advance(now)

			if err := c.heartbeatInstanceState(handle, now); err != nil {
				handle.CloseWithError(err)
				return
			}

		case now := <-sshKeepAliveDelay.Elapsed():
			sshKeepAliveDelay.Advance(now)

			if err := c.keepAliveSSHServer(handle, now); err != nil {
				handle.CloseWithError(err)
				return
			}
			c.testEvent(keepAliveSSHTick)

		case now := <-handle.appKeepAliveDelay.Elapsed():
			key := handle.appKeepAliveDelay.Tick(now)

			if err := c.keepAliveAppServer(handle, now, key); err != nil {
				handle.CloseWithError(err)
				return
			}
			c.testEvent(keepAliveAppTick)

		case now := <-timeReconciliationDelay.Elapsed():
			timeReconciliationDelay.Advance(now)

			if err := c.handlePingRequest(handle, pingRequest{
				id:   rand.Uint64(),
				rspC: make(chan pingResponse, 1),
			}); err != nil {
				handle.CloseWithError(err)
				return
			}

		case now := <-handle.dbKeepAliveDelay.Elapsed():
			key := handle.dbKeepAliveDelay.Tick(now)

			if err := c.keepAliveDatabaseServer(handle, now, key); err != nil {
				handle.CloseWithError(err)
				return
			}

			c.testEvent(keepAliveDatabaseTick)

		case now := <-handle.kubeKeepAliveDelay.Elapsed():
			key := handle.kubeKeepAliveDelay.Tick(now)

			if err := c.keepAliveKubernetesServer(handle, now, key); err != nil {
				handle.CloseWithError(err)
				return
			}
			c.testEvent(keepAliveKubeTick)

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

// variableRateHeartbeatsDisabledEnv checks if variable rate heartbeats have
// been explicitly disabled via environment variable.
func variableRateHeartbeatsDisabledEnv() bool {
	return os.Getenv("TELEPORT_UNSTABLE_DISABLE_VARIABLE_RATE_HEARTBEATS") == "yes"
}

// doResourceCleanup handles deletion of resources in response to a goodbye message. cleanup may only
// be partially applied if too much concurrent cleanup is ongoing and/or if cleanup is taking too long.
func (c *Controller) doResourceCleanup(handle *upstreamHandle) {
	cleanupCtx, cancel := context.WithTimeout(c.closeContext, c.cleanupTimeout)
	defer cancel()
	slog.DebugContext(c.closeContext, "Cleaning up resources in response to instance termination",
		"apps", len(handle.appServers),
		"dbs", len(handle.databaseServers),
		"kube", len(handle.kubernetesServers),
		"server_id", handle.Hello().ServerID,
	)
	for _, app := range handle.appServers {
		if err := c.cleanupLimiter.Wait(cleanupCtx); err != nil {
			slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
			return
		}

		if err := c.auth.DeleteApplicationServer(cleanupCtx, apidefaults.Namespace, app.resource.GetHostID(), app.resource.GetName()); err != nil && !trace.IsNotFound(err) {
			if cleanupCtx.Err() != nil {
				slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
				return
			}
			slog.WarnContext(c.closeContext, "Failed to remove app server on termination",
				"app_server", handle.Hello().ServerID,
				"error", err,
			)
		}
	}

	for _, db := range handle.databaseServers {
		if err := c.cleanupLimiter.Wait(cleanupCtx); err != nil {
			slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
			return
		}

		if err := c.auth.DeleteDatabaseServer(cleanupCtx, apidefaults.Namespace, db.resource.GetHostID(), db.resource.GetName()); err != nil && !trace.IsNotFound(err) {
			if cleanupCtx.Err() != nil {
				slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
				return
			}
			slog.WarnContext(c.closeContext, "Failed to remove db server on termination",
				"db_server", handle.Hello().ServerID,
				"error", err,
			)
		}
	}

	for _, kube := range handle.kubernetesServers {
		if err := c.cleanupLimiter.Wait(cleanupCtx); err != nil {
			slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
			return
		}

		if err := c.auth.DeleteKubernetesServer(c.closeContext, kube.resource.GetHostID(), kube.resource.GetName()); err != nil && !trace.IsNotFound(err) {
			if cleanupCtx.Err() != nil {
				slog.WarnContext(c.closeContext, "halting remaining resource cleanup", "instance_id", handle.Hello().ServerID, "error", err)
				return
			}

			slog.WarnContext(c.closeContext, "Failed to remove kube server on termination",
				"kube_server", handle.Hello().ServerID,
				"error", err,
			)
		}
	}
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
		slog.WarnContext(c.closeContext, "Failed to construct next heartbeat value for instance (this is a bug)",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)
		return trace.Wrap(err)
	}

	// update expiry values using serverTTL as default resource/log ttl.
	instance.SyncLogAndResourceExpiry(c.instanceTTL)

	// perform backend I/O outside of lock
	withoutLock(func() {
		err = c.auth.UpsertInstance(c.closeContext, instance)
	})

	if err != nil {
		slog.WarnContext(c.closeContext, "Failed to hb instance",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)
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

func (c *Controller) handlePong(handle *upstreamHandle, msg *proto.UpstreamInventoryPong) {
	pending, ok := handle.pings[msg.ID]
	if !ok {
		slog.WarnContext(c.closeContext, "Unexpected upstream pong",
			"server_id", handle.Hello().ServerID,
			"pong_id", msg.ID)
		return
	}
	now := c.clock.Now()
	var systemClock time.Time
	if c := msg.GetSystemClock(); c != nil {
		systemClock = c.AsTime()
	}
	pong := pingResponse{
		reqDuration:     now.Sub(pending.start),
		systemClock:     systemClock,
		controllerClock: now,
	}

	handle.stateTracker.mu.Lock()
	handle.stateTracker.pingResponse = pong
	handle.stateTracker.mu.Unlock()

	pending.rspC <- pong
	delete(handle.pings, msg.ID)
	c.testEvent(pongOk)
}

func (c *Controller) handlePingRequest(handle *upstreamHandle, req pingRequest) error {
	ping := &proto.DownstreamInventoryPing{
		ID: req.id,
	}
	start := c.clock.Now()
	if err := handle.Send(c.closeContext, ping); err != nil {
		req.rspC <- pingResponse{
			controllerClock: start,
			err:             err,
		}
		return trace.Wrap(err)
	}
	handle.pings[req.id] = pendingPing{
		start: start,
		rspC:  req.rspC,
	}
	return nil
}

func (c *Controller) handleSSHServerHB(handle *upstreamHandle, sshServer *types.ServerV2, sshDelay *delay.Delay) error {
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

	sshServer.SetExpiry(time.Now().Add(c.serverTTL).UTC())

	if handle.sshServer == nil {
		c.onConnectFunc(constants.KeepAliveNode)
		if c.sshHBVariableDuration != nil {
			c.sshHBVariableDuration.Inc()
		}
		handle.sshServer = &heartBeatInfo[*types.ServerV2]{
			resource: sshServer,
		}
	} else if handle.sshServer.keepAliveErrs == 0 && services.CompareServers(handle.sshServer.resource, sshServer) < services.Different {
		// if we have successfully upserted this exact server the last time
		// (except for the expiry), we don't need to upsert it again right now
		return nil
	} else {
		handle.sshServer.resource = sshServer
	}

	if _, err := c.auth.UpsertNode(c.closeContext, handle.sshServer.resource); err == nil {
		c.testEvent(sshUpsertOk)
		// reset the error status
		handle.sshServer.keepAliveErrs = 0
		handle.sshServer.retryUpsert = false

		sshDelay.Reset()
	} else {
		c.testEvent(sshUpsertErr)
		slog.WarnContext(c.closeContext, "Failed to announce SSH server",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)

		// we use keepAliveErrs as a general upsert error count for SSH,
		// retryUpsert as a flag to signify that we MUST succeed the very next
		// upsert: if we're here it means that we have a new resource to upsert
		// and we have failed to do so once, so if we fail again we are going to
		// fall too far behind and we should let the instance go and connect to
		// a healthier auth server
		handle.sshServer.keepAliveErrs++
		if handle.sshServer.retryUpsert || handle.sshServer.keepAliveErrs > c.maxKeepAliveErrs {
			return trace.Wrap(err, "failed to announce SSH server")
		}
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
		handle.appServers = make(map[resourceKey]*heartBeatInfo[*types.AppServerV3])
	}

	appKey := resourceKey{hostID: appServer.GetHostID(), name: appServer.GetApp().GetName()}

	if _, ok := handle.appServers[appKey]; !ok {
		c.onConnectFunc(constants.KeepAliveApp)
		if c.appHBVariableDuration != nil {
			c.appHBVariableDuration.Inc()
		}
		handle.appServers[appKey] = &heartBeatInfo[*types.AppServerV3]{}
		handle.appKeepAliveDelay.Add(appKey)
	}

	now := c.clock.Now()

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
		slog.WarnContext(c.closeContext, "Failed to upsert app server on heartbeat",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		srv := handle.appServers[appKey]
		srv.lease = nil
		srv.retryUpsert = true
		srv.resource = appServer
	}
	return nil
}

func (c *Controller) handleDatabaseServerHB(handle *upstreamHandle, databaseServer *types.DatabaseServerV3) error {
	// the auth layer verifies that a stream's hello message matches the identity and capabilities of the
	// client cert. after that point it is our responsibility to ensure that heartbeated information is
	// consistent with the identity and capabilities claimed in the initial hello.
	if !handle.HasService(types.RoleDatabase) {
		return trace.AccessDenied("control stream not configured to support database server heartbeats")
	}
	if databaseServer.GetHostID() != handle.Hello().ServerID {
		return trace.AccessDenied("incorrect database server ID (expected %q, got %q)", handle.Hello().ServerID, databaseServer.GetHostID())
	}

	if handle.databaseServers == nil {
		handle.databaseServers = make(map[resourceKey]*heartBeatInfo[*types.DatabaseServerV3])
	}

	dbKey := resourceKey{hostID: databaseServer.GetHostID(), name: databaseServer.GetDatabase().GetName()}

	if _, ok := handle.databaseServers[dbKey]; !ok {
		c.onConnectFunc(constants.KeepAliveDatabase)
		if c.dbHBVariableDuration != nil {
			c.dbHBVariableDuration.Inc()
		}
		handle.databaseServers[dbKey] = &heartBeatInfo[*types.DatabaseServerV3]{}
		handle.dbKeepAliveDelay.Add(dbKey)
	}

	now := time.Now()

	databaseServer.SetExpiry(now.Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertDatabaseServer(c.closeContext, databaseServer)
	if err == nil {
		c.testEvent(dbUpsertOk)
		// store the new lease and reset retry state
		srv := handle.databaseServers[dbKey]
		srv.lease = lease
		srv.retryUpsert = false
		srv.resource = databaseServer
	} else {
		c.testEvent(dbUpsertErr)
		slog.WarnContext(c.closeContext, "Failed to upsert database server on heartbeat",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		srv := handle.databaseServers[dbKey]
		srv.lease = nil
		srv.retryUpsert = true
		srv.resource = databaseServer
	}
	return nil
}

func (c *Controller) handleKubernetesServerHB(handle *upstreamHandle, kubernetesServer *types.KubernetesServerV3) error {
	// the auth layer verifies that a stream's hello message matches the identity and capabilities of the
	// client cert. after that point it is our responsibility to ensure that heartbeated information is
	// consistent with the identity and capabilities claimed in the initial hello.
	if !handle.HasService(types.RoleKube) && !handle.HasService(types.RoleProxy) {
		return trace.AccessDenied("control stream not configured to support kubernetes server heartbeats")
	}
	if kubernetesServer.GetHostID() != handle.Hello().ServerID {
		return trace.AccessDenied("incorrect kubernetes server ID (expected %q, got %q)", handle.Hello().ServerID, kubernetesServer.GetHostID())
	}

	if handle.kubernetesServers == nil {
		handle.kubernetesServers = make(map[resourceKey]*heartBeatInfo[*types.KubernetesServerV3])
	}

	kubeKey := resourceKey{hostID: kubernetesServer.GetHostID(), name: kubernetesServer.GetCluster().GetName()}

	if _, ok := handle.kubernetesServers[kubeKey]; !ok {
		c.onConnectFunc(constants.KeepAliveKube)
		if c.kubeHBVariableDuration != nil {
			c.kubeHBVariableDuration.Inc()
		}
		handle.kubernetesServers[kubeKey] = &heartBeatInfo[*types.KubernetesServerV3]{}
		handle.kubeKeepAliveDelay.Add(kubeKey)
	}

	now := time.Now()

	kubernetesServer.SetExpiry(now.Add(c.serverTTL).UTC())

	lease, err := c.auth.UpsertKubernetesServer(c.closeContext, kubernetesServer)
	if err == nil {
		c.testEvent(kubeUpsertOk)
		// store the new lease and reset retry state
		srv := handle.kubernetesServers[kubeKey]
		srv.lease = lease
		srv.retryUpsert = false
		srv.resource = kubernetesServer
	} else {
		c.testEvent(kubeUpsertErr)
		slog.WarnContext(c.closeContext, "Failed to upsert kubernetes server on heartbeat",
			"server_id", handle.Hello().ServerID,
			"error", err,
		)

		// blank old lease if any and set retry state. next time handleKeepAlive is called
		// we will attempt to upsert the server again.
		srv := handle.kubernetesServers[kubeKey]
		srv.lease = nil
		srv.retryUpsert = true
		srv.resource = kubernetesServer
	}
	return nil
}

func (c *Controller) handleAgentMetadata(handle *upstreamHandle, m *proto.UpstreamInventoryAgentMetadata) {
	handle.setAgentMetadata(m)

	svcs := make([]string, 0, len(handle.Hello().Services))
	for _, svc := range handle.Hello().Services {
		svcs = append(svcs, strings.ToLower(svc))
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

func (c *Controller) keepAliveAppServer(handle *upstreamHandle, now time.Time, name resourceKey) error {
	srv, ok := handle.appServers[name]
	if !ok {
		handle.appKeepAliveDelay.Remove(name)
		return trace.Errorf("desync between app server hb registry and keepalive delay (this is a bug)")
	}

	if srv.lease != nil {
		lease := *srv.lease
		lease.Expires = now.Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(appKeepAliveErr)

			srv.keepAliveErrs++
			handle.appServers[name] = srv
			shouldRemove := srv.keepAliveErrs > c.maxKeepAliveErrs
			slog.WarnContext(c.closeContext, "Failed to keep alive app server",
				"server_id", handle.Hello().ServerID,
				"error", err,
				"error_count", srv.keepAliveErrs,
				"should_remove", shouldRemove,
			)

			if shouldRemove {
				c.testEvent(appKeepAliveDel)
				c.onDisconnectFunc(constants.KeepAliveApp, 1)
				if c.appHBVariableDuration != nil {
					c.appHBVariableDuration.Dec()
				}
				delete(handle.appServers, name)
				handle.appKeepAliveDelay.Remove(name)
			}
		} else {
			srv.keepAliveErrs = 0
			c.testEvent(appKeepAliveOk)
		}
	} else if srv.retryUpsert {
		srv.resource.SetExpiry(c.clock.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertApplicationServer(c.closeContext, srv.resource)
		if err != nil {
			c.testEvent(appUpsertRetryErr)
			slog.WarnContext(c.closeContext, "Failed to upsert app server on retry",
				"server_id", handle.Hello().ServerID,
				"error", err,
			)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don't bother
			// attempting a third time.
			return trace.Errorf("failed to upsert app server on retry: %v", err)
		}
		c.testEvent(appUpsertRetryOk)

		srv.lease = lease
		srv.retryUpsert = false
	}

	return nil
}

func (c *Controller) keepAliveDatabaseServer(handle *upstreamHandle, now time.Time, name resourceKey) error {
	srv, ok := handle.databaseServers[name]
	if !ok {
		handle.dbKeepAliveDelay.Remove(name)
		return trace.Errorf("desync between db server hb registry and keepalive delay (this is a bug)")
	}
	if srv.lease != nil {
		lease := *srv.lease
		lease.Expires = now.Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(dbKeepAliveErr)

			srv.keepAliveErrs++
			handle.databaseServers[name] = srv
			shouldRemove := srv.keepAliveErrs > c.maxKeepAliveErrs
			slog.WarnContext(c.closeContext, "Failed to keep alive database server",
				"server_id", handle.Hello().ServerID,
				"error", err,
				"error_count", srv.keepAliveErrs,
				"should_remove", shouldRemove,
			)

			if shouldRemove {
				c.testEvent(dbKeepAliveDel)
				c.onDisconnectFunc(constants.KeepAliveDatabase, 1)
				if c.dbHBVariableDuration != nil {
					c.dbHBVariableDuration.Dec()
				}
				delete(handle.databaseServers, name)
				handle.dbKeepAliveDelay.Remove(name)
			}
		} else {
			srv.keepAliveErrs = 0
			c.testEvent(dbKeepAliveOk)
		}
	} else if srv.retryUpsert {
		srv.resource.SetExpiry(time.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertDatabaseServer(c.closeContext, srv.resource)
		if err != nil {
			c.testEvent(dbUpsertRetryErr)
			slog.WarnContext(c.closeContext, "Failed to upsert database server on retry",
				"server_id", handle.Hello().ServerID,
				"error", err,
			)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don't bother
			// attempting a third time.
			return trace.Errorf("failed to upsert database server on retry: %v", err)
		}
		c.testEvent(dbUpsertRetryOk)

		srv.lease = lease
		srv.retryUpsert = false
	}

	return nil
}

func (c *Controller) keepAliveKubernetesServer(handle *upstreamHandle, now time.Time, name resourceKey) error {
	srv, ok := handle.kubernetesServers[name]
	if !ok {
		handle.kubeKeepAliveDelay.Remove(name)
		return trace.Errorf("desync between kube server hb registry and keepalive delay (this is a bug)")
	}

	if srv.lease != nil {
		lease := *srv.lease
		lease.Expires = now.Add(c.serverTTL).UTC()
		if err := c.auth.KeepAliveServer(c.closeContext, lease); err != nil {
			c.testEvent(kubeKeepAliveErr)

			srv.keepAliveErrs++
			handle.kubernetesServers[name] = srv
			shouldRemove := srv.keepAliveErrs > c.maxKeepAliveErrs
			slog.WarnContext(c.closeContext, "Failed to keep alive kubernetes server",
				"server_id", handle.Hello().ServerID,
				"error", err,
				"error_count", srv.keepAliveErrs,
				"should_remove", shouldRemove,
			)

			if shouldRemove {
				c.testEvent(kubeKeepAliveDel)
				c.onDisconnectFunc(constants.KeepAliveKube, 1)
				if c.kubeHBVariableDuration != nil {
					c.kubeHBVariableDuration.Dec()
				}
				delete(handle.kubernetesServers, name)
				handle.kubeKeepAliveDelay.Remove(name)
			}
		} else {
			srv.keepAliveErrs = 0
			c.testEvent(kubeKeepAliveOk)
		}
	} else if srv.retryUpsert {
		srv.resource.SetExpiry(time.Now().Add(c.serverTTL).UTC())
		lease, err := c.auth.UpsertKubernetesServer(c.closeContext, srv.resource)
		if err != nil {
			c.testEvent(kubeUpsertRetryErr)
			slog.WarnContext(c.closeContext, "Failed to upsert kubernetes server on retry.",
				"server_id", handle.Hello().ServerID,
				"error", err,
			)
			// since this is retry-specific logic, an error here means that upsert failed twice in
			// a row. Missing upserts is more problematic than missing keepalives so we don'resource bother
			// attempting a third time.
			return trace.Errorf("failed to upsert kubernetes server on retry: %v", err)
		}
		c.testEvent(kubeUpsertRetryOk)

		srv.lease = lease
		srv.retryUpsert = false
	}

	return nil
}

func (c *Controller) keepAliveSSHServer(handle *upstreamHandle, now time.Time) error {
	if handle.sshServer == nil {
		return nil
	}

	handle.sshServer.resource.SetExpiry(now.Add(c.serverTTL).UTC())
	if _, err := c.auth.UpsertNode(c.closeContext, handle.sshServer.resource); err == nil {
		if handle.sshServer.retryUpsert {
			c.testEvent(sshUpsertRetryOk)
		} else {
			c.testEvent(sshKeepAliveOk)
		}
		handle.sshServer.keepAliveErrs = 0
		handle.sshServer.retryUpsert = false
	} else {
		if handle.sshServer.retryUpsert {
			c.testEvent(sshUpsertRetryErr)
			slog.WarnContext(c.closeContext, "Failed to upsert SSH server on retry",
				"server_id", handle.Hello().ServerID,
				"error", err,
			)
			// retryUpsert is set when we get a new resource and we fail to
			// upsert it; if we're here it means that we have failed to upsert
			// it _again_, so we have fallen quite far behind
			return trace.Wrap(err, "failed to upsert SSH server on retry")
		}

		c.testEvent(sshKeepAliveErr)
		handle.sshServer.keepAliveErrs++
		closing := handle.sshServer.keepAliveErrs > c.maxKeepAliveErrs
		slog.WarnContext(c.closeContext, "Failed to upsert SSH server on keepalive",
			"server_id", handle.Hello().ServerID,
			"error", err,
			"count", handle.sshServer.keepAliveErrs,
			"closing", closing,
		)

		if closing {
			return trace.Wrap(err, "failed to keep alive SSH server")
		}
	}

	return nil
}

func (c *Controller) createKeepAliveDelay(variableDuration *interval.VariableDuration) *delay.Delay {
	return delay.New(delay.Params{
		FirstInterval:    retryutils.HalfJitter(c.serverKeepAlive),
		FixedInterval:    c.serverKeepAlive,
		VariableInterval: variableDuration,
		Jitter:           retryutils.SeventhJitter,
	})
}

func (c *Controller) createKeepAliveMultiDelay(variableDuration *interval.VariableDuration) *delay.Multi[resourceKey] {
	return delay.NewMulti[resourceKey](delay.MultiParams{
		FixedInterval:    c.serverKeepAlive,
		VariableInterval: variableDuration,
		FirstJitter:      retryutils.HalfJitter,
		Jitter:           retryutils.SeventhJitter,
	})
}

// Close terminates all control streams registered with this controller. Control streams
// registered after Close() is called are closed immediately.
func (c *Controller) Close() error {
	c.cancel()
	return nil
}
