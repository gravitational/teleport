/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package internal holds the topology-agnostic machinery shared by every
// Teleport cache: the fetch/watch/apply lifecycle, read-health publication,
// and event fanout. It is deliberately an internal package — consumers
// interact with the exported topology caches in lib/cache, never with the
// engine directly.
package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ResourceKind identifies a cached resource kind/subkind pair. It is the
// registry key for collection handlers.
type ResourceKind struct {
	Kind    string
	SubKind string
}

func (r ResourceKind) String() string {
	if r.SubKind == "" {
		return r.Kind
	}
	return r.Kind + "/" + r.SubKind
}

// ResourceKindFromResource derives the registry key for an event resource.
func ResourceKindFromResource(res types.Resource) ResourceKind {
	switch res.GetKind() {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return ResourceKind{
			Kind:    res.GetKind(),
			SubKind: res.GetSubKind(),
		}
	}
	return ResourceKind{
		Kind: res.GetKind(),
	}
}

// Handler is implemented by cache collections: it seeds initial data and
// applies event-stream updates for one resource kind.
type Handler interface {
	// Fetch fetches resources and returns a function which will apply said
	// resources to the cache. Fetch *must* not mutate cache state outside of
	// the apply function. The provided cacheOK flag indicates whether this
	// collection will be included in the cache generation that is being
	// prepared. If cacheOK is false, Fetch shouldn't fetch any resources, but
	// the apply function that it returns must still delete resources from the
	// backend.
	Fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// OnDeletes removes the target resources from the cache.
	OnDeletes(rs []types.Resource) error
	// OnPuts updates the target resources in the cache.
	OnPuts(rs []types.Resource) error
}

// Config configures an [Engine]. All values are assumed validated by the
// caller (lib/cache.Config.CheckAndSetDefaults).
type Config struct {
	// Target identifies the cache topology in errors, logs and metrics
	// (e.g. "auth", "proxy").
	Target string
	// Logger emits log messages.
	Logger *slog.Logger
	// Watches is the list of resource kinds replicated by this cache.
	Watches []types.WatchKind
	// Handlers is the registry of collection handlers, keyed by kind.
	Handlers map[ResourceKind]Handler
	// Events provides the upstream event watchers.
	Events types.Events
	// FanoutShards is the number of low-volume event fanout shards.
	FanoutShards int
	// MaxRetryPeriod is the maximum period between cache retries on failures.
	MaxRetryPeriod time.Duration
	// WatcherInitTimeout is the maximum acceptable delay for an OpInit after
	// a watcher has been started.
	WatcherInitTimeout time.Duration
	// CacheInitTimeout is the maximum amount of time that Start will block,
	// waiting for initialization.
	CacheInitTimeout time.Duration
	// EnableRelativeExpiry schedules periodic RelativeExpiry runs.
	EnableRelativeExpiry bool
	// RelativeExpiryCheckInterval determines how often RelativeExpiry runs.
	RelativeExpiryCheckInterval time.Duration
	// RelativeExpiry is invoked on the relative-expiry interval; it is
	// supplied by the topology cache since expiry inspects cached resources.
	RelativeExpiry func(ctx context.Context) error
	// EventsC is a channel for event notifications, used in tests.
	EventsC chan Event
	// Clock can be set to control time.
	Clock clockwork.Clock
	// Component is a component used in logs.
	Component string
	// MetricComponent is a component used in metrics.
	MetricComponent string
	// QueueSize is the desired watcher queue size.
	QueueSize int
	// NeverOK is used in tests to create a cache that never becomes healthy.
	NeverOK bool
	// Tracer is used to create spans.
	Tracer oteltrace.Tracer
	// Registerer is used to register prometheus metrics.
	Registerer prometheus.Registerer
	// DisablePartialHealth disables the default mode in which the cache can
	// become healthy even if some of the requested resource kinds aren't
	// supported by the event source.
	DisablePartialHealth bool
}

// Engine drives the shared cache lifecycle: it establishes and maintains the
// upstream watch, seeds collections via their handlers, applies event batches,
// publishes read-health state, and fans events out to subscribers.
type Engine struct {
	Config

	// readStatus is the atomically-published read-health state of the cache:
	// whether the cache is valid for reads, and which kinds were confirmed by
	// the server for the current generation. Both are published together so
	// that a read observes a consistent pair with a single atomic load.
	//
	// Reads intentionally do not exclude resets: collection stores are
	// snapshot-based, so a read that began against the previous generation
	// keeps observing a complete, internally consistent view even while an
	// apply replaces store contents. If readStatus reports not-ok, reads are
	// forwarded directly to the backend instead.
	readStatus atomic.Pointer[readState]

	// generation is a counter that is incremented each time a healthy
	// state is established.  A generation of zero means that a healthy
	// state was never established.  Note that a generation of zero does
	// not preclude `ok` being true in the case that we have loaded a
	// previously healthy state from the backend.
	generation atomic.Uint64

	// initOnce protects initC and initErr.
	initOnce sync.Once
	// initC is closed on the first attempt to initialize the
	// cache, whether or not it is successful.  Once initC
	// has returned, initErr is safe to read.
	initC chan struct{}
	// initErr is set if the first attempt to initialize the cache
	// fails.
	initErr error

	// firstTimeInitC is closed on the first successful initialization of the cache
	firstTimeInitC    chan struct{}
	firstTimeInitOnce sync.Once

	// ctx is a cache exit context
	ctx context.Context
	// cancel triggers exit context closure
	cancel context.CancelFunc

	eventsFanout          *services.FanoutV2
	lowVolumeEventsFanout *utils.RoundRobin[*services.FanoutV2]

	// closed indicates that the cache has been closed
	closed atomic.Bool
}

// NewEngine creates an unstarted engine and registers its metrics.
func NewEngine(cfg Config) (*Engine, error) {
	if err := metrics.RegisterCollectors(cfg.Registerer,
		cacheEventsReceived,
		cacheStaleEventsReceived,
		cacheHealth,
		cacheLastReset,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fanout := services.NewFanoutV2(services.FanoutV2Config{})
	lowVolumeFanouts := make([]*services.FanoutV2, 0, cfg.FanoutShards)
	for range cfg.FanoutShards {
		lowVolumeFanouts = append(lowVolumeFanouts, services.NewFanoutV2(services.FanoutV2Config{}))
	}

	e := &Engine{
		Config:                cfg,
		ctx:                   ctx,
		cancel:                cancel,
		initC:                 make(chan struct{}),
		firstTimeInitC:        make(chan struct{}),
		eventsFanout:          fanout,
		lowVolumeEventsFanout: utils.NewRoundRobin(lowVolumeFanouts),
	}
	e.readStatus.Store(&readState{})
	return e, nil
}

// ExitContext returns the engine's exit context: it is canceled when the
// engine closes, and parents any background work tied to the cache lifetime.
func (c *Engine) ExitContext() context.Context {
	return c.ctx
}

// Closed reports whether the engine has been closed.
func (c *Engine) Closed() bool {
	return c.closed.Load()
}

// KindConfirmed reports whether the cache is accessible for reads of the
// given kind in the current generation.
func (c *Engine) KindConfirmed(k ResourceKind) bool {
	return c.readStatus.Load().kindConfirmed(k)
}

// ReadOK reports whether the cache is currently in a valid state for reads.
func (c *Engine) ReadOK() bool {
	return c.readStatus.Load().healthy()
}

// SetReadOK flips the health bit while preserving the confirmed kinds of the
// current generation. It is a test helper for simulating an unhealthy cache;
// unlike setReadStatus it ignores NeverOK.
func (c *Engine) SetReadOK(ok bool) {
	prev := c.readStatus.Load()
	next := &readState{ok: ok}
	if prev != nil {
		next.confirmedKinds = prev.confirmedKinds
	}
	c.readStatus.Store(next)
}

var (
	cacheEventsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricCacheEventsReceived,
			Help:      "Number of events received by a Teleport service cache. Teleport's Auth Service, Proxy Service, and other services cache incoming events related to their service.",
		},
		[]string{teleport.TagCacheComponent},
	)
	cacheStaleEventsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricStaleCacheEventsReceived,
			Help:      "Number of stale events received by a Teleport service cache. A high percentage of stale events can indicate a degraded backend.",
		},
		[]string{teleport.TagCacheComponent},
	)

	cacheHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "cache",
			Name:      "health",
			Help:      "Whether the cache for a particular Teleport service is healthy.",
		},
		[]string{teleport.TagCacheComponent},
	)

	cacheLastReset = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "cache",
			Name:      "last_reset_seconds",
			Help:      "The unix time in seconds that the last cache reset was performed.",
		},
		[]string{teleport.TagCacheComponent},
	)
)

// highVolumeResources is the set of cached resources that tend to produce high
// event volumes (e.g. heartbeat resources). high volume events, and the watchers that
// care about them, are separated into a dedicated event fanout system in order to
// reduce the amount of load on watchers that only care about cluster state resources.
// peripheral agents that scale linearly with cluster resources (e.g. nodes) should never
// watch events of this kind.
var highVolumeResources = map[string]struct{}{
	types.KindNode:                  {},
	types.KindAppServer:             {},
	types.KindDatabaseServer:        {},
	types.KindDatabaseService:       {},
	types.KindWindowsDesktopService: {},
	types.KindKubeServer:            {},
	types.KindDatabaseObject:        {},
	types.KindGitServer:             {},
}

func IsHighVolumeResource(kind string) bool {
	_, ok := highVolumeResources[kind]
	return ok
}

func (c *Engine) setInitError(err error) {
	c.initOnce.Do(func() {
		c.initErr = err
		close(c.initC)
	})

	if err == nil {
		c.firstTimeInitOnce.Do(func() {
			close(c.firstTimeInitC)
		})
		cacheHealth.WithLabelValues(c.Target).Set(1.0)
	} else {
		cacheHealth.WithLabelValues(c.Target).Set(0.0)
	}
}

// FirstInit returns a channel that is closed when the cache successfully initializes for the first time.
func (c *Engine) FirstInit() <-chan struct{} {
	return c.firstTimeInitC
}

// readState is the read-health state of the cache, published atomically via
// Engine.readStatus so that the health bit and the confirmed kinds are always
// observed together.
type readState struct {
	// ok indicates whether the cache is in a valid state for reads.
	// If ok is false, reads are forwarded directly to the backend.
	ok bool
	// confirmedKinds is a map of kinds confirmed by the server to be included
	// in the current generation by resource Kind/SubKind.
	confirmedKinds map[ResourceKind]types.WatchKind
}

// healthy reports whether the cache is overall accessible for reads.
func (s *readState) healthy() bool {
	return s != nil && s.ok
}

// kindConfirmed reports whether the cache is accessible for reads of the
// given kind in the current generation.
func (s *readState) kindConfirmed(k ResourceKind) bool {
	if !s.healthy() {
		return false
	}
	_, ok := s.confirmedKinds[k]
	return ok
}

// SetReadStatus publishes whether the cache is overall accessible for reads,
// along with the resource kinds accessible in the current generation.
func (c *Engine) SetReadStatus(ok bool, confirmedKinds map[ResourceKind]types.WatchKind) {
	if c.NeverOK {
		// we are running inside of a test where the cache
		// needs to pretend that it never becomes healthy.
		return
	}
	c.readStatus.Store(&readState{ok: ok, confirmedKinds: confirmedKinds})
}

// Event is event used in tests
type Event struct {
	// Type is event type
	Type string
	// Event is event processed
	// by the event cycle
	Event types.Event
}

const (
	// EventProcessed is emitted whenever event is processed
	EventProcessed = "event_processed"
	// WatcherStarted is emitted when a new event watcher is started
	WatcherStarted = "watcher_started"
	// WatcherFailed is emitted when event watcher has failed
	WatcherFailed = "watcher_failed"
	// Reloading is emitted when an error occurred watching events
	// and the cache is waiting to create a new watcher
	Reloading = "reloading_cache"
	// RelativeExpiry notifies that relative expiry operations have
	// been run.
	RelativeExpiry = "relative_expiry"
)

// Start the cache. Should only be called once.
func (c *Engine) Start() error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(c.MaxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(c.MaxRetryPeriod / 16),
		Max:    c.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  c.Clock,
	})
	if err != nil {
		c.Close()
		return trace.Wrap(err)
	}

	go c.update(c.ctx, retry)

	select {
	case <-c.initC:
		if c.initErr == nil {
			c.Logger.InfoContext(c.ctx, "Cache first init succeeded")
		} else {
			c.Logger.WarnContext(c.ctx, "Cache first init failed, continuing re-init attempts in background", "error", c.initErr)
		}
	case <-c.ctx.Done():
		c.Close()
		return trace.Wrap(c.ctx.Err(), "context closed during cache init")
	case <-time.After(c.Config.CacheInitTimeout):
		c.Logger.WarnContext(c.ctx, "Cache init is taking too long, will continue in background")
	}
	return nil
}

// NewStream is equivalent to NewWatcher except that it represents the event
// stream as a stream.Stream rather than a channel. Watcher style event handling
// is generally more common, but this API may be preferable for usecases where
// *many* event streams need to be allocated as it is slightly more resource-efficient.
func (c *Engine) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	ctx, span := c.Tracer.Start(ctx, "cache/NewStream")
	defer span.End()

	validKinds, highVolume, err := c.validateWatchRequest(watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watch.Kinds = validKinds
	if highVolume {
		// watch request includes high volume resources, register with the
		// full fanout instance.
		return c.eventsFanout.NewStream(ctx, watch), nil
	}
	// watch request does not contain high volume resources, register with
	// the low volume fanout instance (improves performance at scale).
	return c.lowVolumeEventsFanout.Next().NewStream(ctx, watch), nil
}

// NewWatcher returns a new event watcher. In case of a cache
// this watcher will return events as seen by the cache,
// not the backend. This feature allows auth server
// to handle subscribers connected to the in-memory caches
// instead of reading from the backend.
func (c *Engine) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/NewWatcher")
	defer span.End()

	validKinds, highVolume, err := c.validateWatchRequest(watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watch.Kinds = validKinds
	if highVolume {
		// watch request includes high volume resources, register with the
		// full fanout instance.
		return c.eventsFanout.NewWatcher(ctx, watch)
	}
	// watch request does not contain high volume resources, register with
	// the low volume fanout instance (improves performance at scale).
	return c.lowVolumeEventsFanout.Next().NewWatcher(ctx, watch)
}

func (c *Engine) validateWatchRequest(watch types.Watch) (kinds []types.WatchKind, highVolume bool, err error) {
	state := c.readStatus.Load()
	cacheOK := state.healthy()
	var confirmedKinds map[ResourceKind]types.WatchKind
	if state != nil {
		confirmedKinds = state.confirmedKinds
	}

	validKinds := make([]types.WatchKind, 0, len(watch.Kinds))
	var containsHighVolumeResource bool
Outer:
	for _, requested := range watch.Kinds {
		if IsHighVolumeResource(requested.Kind) {
			containsHighVolumeResource = true
		}
		if cacheOK {
			// if cache has been initialized, we already know which kinds are confirmed by the event source
			// and can validate the kinds requested for fanout against that.
			key := ResourceKind{Kind: requested.Kind, SubKind: requested.SubKind}
			if confirmed, ok := confirmedKinds[key]; !ok || !confirmed.Contains(requested) {
				if watch.AllowPartialSuccess {
					continue
				}
				return nil, false, trace.BadParameter("cache %q does not support watching resource %q", c.Config.Target, requested.Kind)
			}
			validKinds = append(validKinds, requested)
		} else {
			// otherwise, we can only perform preliminary validation against the kinds that cache has been configured for,
			// and the returned fanout watcher might fail later when cache receives and propagates its OpInit event.
			for _, configured := range c.Config.Watches {
				if requested.Kind == configured.Kind && requested.SubKind == configured.SubKind && configured.Contains(requested) {
					validKinds = append(validKinds, requested)
					continue Outer
				}
			}
			if watch.AllowPartialSuccess {
				continue
			}
			return nil, false, trace.BadParameter("cache %q does not support watching resource %q", c.Config.Target, requested.Kind)
		}
	}

	if len(validKinds) == 0 {
		return nil, false, trace.BadParameter("cache %q does not support any of the requested resources", c.Config.Target)
	}

	return validKinds, containsHighVolumeResource, nil
}

func (c *Engine) update(ctx context.Context, retry retryutils.Retry) {
	defer func() {
		c.Logger.DebugContext(ctx, "Cache is closing, returning from update loop")
		// ensure that close operations have been run
		c.Close()
	}()
	timer := time.NewTimer(c.Config.WatcherInitTimeout)
	for {
		err := c.fetchAndWatch(ctx, retry, timer)
		c.setInitError(err)
		if c.isClosing() {
			return
		}
		if err != nil {
			c.Logger.WarnContext(ctx, "Re-init the cache on error", "error", err)
		}

		// events cache should be closed as well
		c.Logger.DebugContext(ctx, "Reloading cache")

		c.notify(ctx, Event{Type: Reloading, Event: types.Event{
			Resource: &types.ResourceHeader{
				Kind: retry.Duration().String(),
			},
		}})

		startedWaiting := c.Clock.Now()
		select {
		case t := <-retry.After():
			c.Logger.DebugContext(ctx, "Initiating new watch after backoff", "backoff_time", t.Sub(startedWaiting))
			retry.Inc()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Engine) notify(ctx context.Context, event Event) {
	if c.EventsC == nil {
		return
	}
	select {
	case c.EventsC <- event:
		return
	case <-ctx.Done():
		return
	}
}

// fetchAndWatch keeps cache up to date by replaying
// events and syncing local cache storage.
//
// Here are some thoughts on consistency in face of errors:
//
// 1. Every client is connected to the database fan-out
// system. This system creates a buffered channel for every
// client and tracks the channel overflow. Thanks to channels every client gets its
// own unique iterator over the event stream. If client loses connection
// or fails to keep up with the stream, the server will terminate
// the channel and client will have to re-initialize.
//
// 2. Replays of stale events. Etcd provides a strong
// mechanism to track the versions of the storage - revisions
// of every operation that are uniquely numbered and monotonically
// and consistently ordered thanks to Raft. Unfortunately, DynamoDB
// does not provide such a mechanism for its event system, so
// some tradeoffs have to be made:
//
//	a. We assume that events are ordered in regards to the
//	individual key operations which is the guarantees both Etcd and DynamoDB
//	provide.
//	b. Thanks to the init event sent by the server on a successful connect,
//	and guarantees 1 and 2a, client assumes that once it connects and receives an event,
//	it will not miss any events, however it can receive stale events.
//	Event could be stale, if it relates to a change that happened before
//	the version read by client from the database, for example,
//	given the event stream: 1. Update a=1 2. Delete a 3. Put a = 2
//	Client could have subscribed before event 1 happened,
//	read the value a=2 and then received events 1 and 2 and 3.
//	The cache will replay all events 1, 2 and 3 and end up in the correct
//	state 3. If we had a consistent revision number, we could
//	have skipped 1 and 2, but in the absence of such mechanism in Dynamo
//	we assume that this cache will eventually end up in a correct state
//	potentially lagging behind the state of the database.
func (c *Engine) fetchAndWatch(ctx context.Context, retry retryutils.Retry, timer *time.Timer) error {
	cacheLastReset.WithLabelValues(c.Target).SetToCurrentTime()
	requestKinds := c.Config.Watches
	watcher, err := c.Events.NewWatcher(c.ctx, types.Watch{
		Name:                c.Component,
		Kinds:               requestKinds,
		QueueSize:           c.QueueSize,
		MetricComponent:     c.MetricComponent,
		AllowPartialSuccess: !c.DisablePartialHealth,
	})
	if err != nil {
		c.notify(c.ctx, Event{Type: WatcherFailed})
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// ensure that the timer is stopped and drained
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
	// set timer to watcher init timeout
	timer.Reset(c.Config.WatcherInitTimeout)

	var confirmedKinds []types.WatchKind

	// before fetch, make sure watcher is synced by receiving init event,
	// to avoid the scenario:
	// 1. Cache process:   w = NewWatcher()
	// 2. Cache process:   c.fetch()
	// 3. Backend process: addItem()
	// 4. Cache process:   <- w.Events()
	//
	// If there is a way that NewWatcher() on line 1 could
	// return without subscription established first,
	// Code line 3 could execute and line 4 could miss event,
	// wrapping up without of sync replica.
	// To avoid this, before doing fetch,
	// cache process makes sure the connection is established
	// by receiving init event first.
	select {
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
	case <-c.ctx.Done():
		return trace.ConnectionProblem(c.ctx.Err(), "context is closing")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
		if watchStatus, ok := event.Resource.(types.WatchStatus); ok {
			confirmedKinds = watchStatus.GetKinds()
		} else {
			// this event was generated by an old Auth service that doesn't support partial success mode,
			// which means that we can assume all requested kinds to be confirmed.
			confirmedKinds = requestKinds
		}
	case <-timer.C:
		return trace.ConnectionProblem(nil, "timeout waiting for watcher init")
	}

	fetchAndApplyStart := time.Now()

	confirmedKindsMap := make(map[ResourceKind]types.WatchKind, len(confirmedKinds))
	for _, kind := range confirmedKinds {
		confirmedKindsMap[ResourceKind{Kind: kind.Kind, SubKind: kind.SubKind}] = kind
	}
	if len(confirmedKinds) < len(requestKinds) {
		rejectedKinds := make([]string, 0, len(requestKinds)-len(confirmedKinds))
		for _, kind := range requestKinds {
			key := ResourceKind{Kind: kind.Kind, SubKind: kind.SubKind}
			if _, ok := confirmedKindsMap[key]; !ok {
				rejectedKinds = append(rejectedKinds, key.String())
			}
		}
		c.Logger.WarnContext(ctx, "Some resource kinds unsupported by the server cannot be cached",
			"rejected", rejectedKinds,
		)
	}

	apply, err := c.fetch(ctx, confirmedKindsMap)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply will mutate cache, and possibly leave it in an invalid state
	// if an error occurs, so ensure that cache is not read.
	c.SetReadStatus(false, nil)
	err = apply(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply was successful; cache is now readable.
	c.generation.Add(1)
	c.SetReadStatus(true, confirmedKindsMap)
	c.setInitError(nil)

	// watchers have been queuing up since the last time
	// the cache was in a healthy state; broadcast OpInit.
	// It is very important that OpInit is not broadcast until
	// after we've placed the cache into a readable state.  This ensures
	// that any derivative caches do not perform their fetch operations
	// until this cache has finished its apply operations.
	c.eventsFanout.SetInit(confirmedKinds)
	c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.SetInit(confirmedKinds)
	})
	defer c.eventsFanout.Reset()
	defer c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.Reset()
	})

	retry.Reset()

	// Only enable relative node expiry for the auth cache.
	relativeExpiryInterval := interval.NewNoop()
	if c.EnableRelativeExpiry {
		relativeExpiryInterval = interval.New(interval.Config{
			Duration:      c.Config.RelativeExpiryCheckInterval,
			FirstDuration: retryutils.HalfJitter(c.Config.RelativeExpiryCheckInterval),
			Jitter:        retryutils.SeventhJitter,
		})
	}
	defer relativeExpiryInterval.Stop()

	c.notify(c.ctx, Event{Type: WatcherStarted})

	fetchAndApplyDuration := time.Since(fetchAndApplyStart)
	if fetchAndApplyDuration > time.Second*20 {
		c.Logger.WarnContext(ctx, "slow fetch and apply",
			"cache_target", c.Config.Target,
			"duration", fetchAndApplyDuration.String(),
		)
	} else {
		c.Logger.Log(ctx, logutils.TraceLevel, "fetch and apply",
			"cache_target", c.Config.Target,
			"duration", fetchAndApplyDuration.String(),
		)
	}

	var lastStalenessWarning time.Time
	var staleEventCount int
	batch := make([]types.Event, 0, maxEventBatchSize)
	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-c.ctx.Done():
			return trace.ConnectionProblem(c.ctx.Err(), "context is closing")
		case <-relativeExpiryInterval.Next():
			if err := c.RelativeExpiry(ctx); err != nil {
				return trace.Wrap(err)
			}
			c.notify(ctx, Event{Type: RelativeExpiry})
		case event := <-watcher.Events():
			// opportunistically drain any other already-pending events so
			// that bursts are applied as batched store commits instead of
			// one copy-on-write commit per event.
			batch = append(batch[:0], event)
		drain:
			for len(batch) < maxEventBatchSize {
				select {
				case next := <-watcher.Events():
					batch = append(batch, next)
				default:
					break drain
				}
			}

			// check for expired resources in OpPut events and log them periodically. stale OpPut events
			// may be an indicator of poor performance, and can lead to confusing and inconsistent state
			// as the cache may prune items that aught to exist.
			//
			// NOTE: The inconsistent state mentioned above is a symptom of a deeper issue with the cache
			// design.  The cache should not expire individual items.  It should instead rely on OpDelete events
			// from backend expires. As soon as the cache has expired at least one item, it is no longer
			// a faithful representation of a real backend state, since it is 'anticipating' a change in
			// backend state that may or may not have actually happened.  Instead, it aught to serve the
			// most recent internally-consistent "view" of the backend, and individual consumers should
			// determine if the resources they are handling are sufficiently fresh.  Resource-level expiry
			// is a convenience/cleanup feature and aught not be relied upon for meaningful logic anyhow.
			// If we need to protect against a stale cache, we aught to invalidate the cache in its entirety, rather
			// than pruning the resources that we think *might* have been removed from the real backend.
			// TODO(fspmarshall): ^^^
			//
			cacheEventsReceived.WithLabelValues(c.Target).Add(float64(len(batch)))
			for _, event := range batch {
				if event.Type == types.OpPut && !event.Resource.Expiry().IsZero() {
					if now := c.Clock.Now(); now.After(event.Resource.Expiry()) {
						cacheStaleEventsReceived.WithLabelValues(c.Target).Inc()
						staleEventCount++
						if now.After(lastStalenessWarning.Add(time.Minute)) {
							kind := event.Resource.GetKind()
							if sk := event.Resource.GetSubKind(); sk != "" {
								kind = fmt.Sprintf("%s/%s", kind, sk)
							}
							c.Logger.WarnContext(ctx, "Encountered stale event(s), may indicate degraded backend or event system performance",
								"stale_event_count", staleEventCount,
								"last_kind", kind,
							)
							lastStalenessWarning = now
							staleEventCount = 0
						}
					}
				}
			}

			if err := c.processEventBatch(ctx, batch); err != nil {
				return trace.Wrap(err)
			}
			for _, event := range batch {
				c.notify(c.ctx, Event{Event: event, Type: EventProcessed})
			}
		}
	}
}

// isClosing checks if the cache has begun closing.
func (c *Engine) isClosing() bool {
	if c.closed.Load() {
		// closing due to Close being called
		return true
	}

	select {
	case <-c.ctx.Done():
		// closing due to context cancellation
		return true
	default:
		// not closing
		return false
	}
}

// Close closes all outstanding and active cache operations
func (c *Engine) Close() error {
	c.closed.Store(true)
	c.cancel()
	c.eventsFanout.Close()
	c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.Close()
	})
	return nil
}

// applyFn applies the fetched resources for a
// particular collection
type applyFn func(ctx context.Context) error

// tracedApplyFn wraps an apply function with a span that is
// a child of the provided parent span. Since the context provided
// to the applyFn won't be from fetch, we need to manually link
// the spans.
func tracedApplyFn(parent oteltrace.Span, tracer oteltrace.Tracer, kind ResourceKind, f applyFn) applyFn {
	return func(ctx context.Context) (err error) {
		ctx, span := tracer.Start(
			oteltrace.ContextWithSpan(ctx, parent),
			fmt.Sprintf("cache/apply/%s", kind.String()),
		)
		defer func() { apitracing.EndSpan(span, err) }()

		return f(ctx)
	}
}

// fetchLimit determines the parallelism of the
// fetch operations based on the target. Both the
// auth and proxy caches are permitted to run parallel
// fetches for resources, while all other targets are
// throttled to limit load spiking during a mass
// restart of nodes
func fetchLimit(target string) int {
	if IsControlPlane(target) {
		return 5
	}

	return 1
}

// IsControlPlane checks if the cache target is a control-plane element.
func IsControlPlane(target string) bool {
	switch target {
	case "auth", "proxy":
		return true
	}

	return false
}

func (c *Engine) fetch(ctx context.Context, confirmedKinds map[ResourceKind]types.WatchKind) (fn applyFn, err error) {
	ctx, fetchSpan := c.Tracer.Start(ctx, "cache/fetch", oteltrace.WithAttributes(attribute.String("target", c.Target)))
	defer func() { apitracing.EndSpan(fetchSpan, err) }()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(fetchLimit(c.Target))
	applyfns := make([]applyFn, len(c.Handlers))
	i := 0

	for kind, handler := range c.Handlers {
		ii := i
		i++

		g.Go(func() (err error) {
			ctx, span := c.Tracer.Start(
				ctx,
				"cache/fetch/"+kind.String(),
				oteltrace.WithAttributes(attribute.String("target", c.Target)),
			)
			defer func() { apitracing.EndSpan(span, err) }()

			_, cacheOK := confirmedKinds[ResourceKind{Kind: kind.Kind, SubKind: kind.SubKind}]
			applyfn, err := handler.Fetch(ctx, cacheOK)
			if err != nil {
				return trace.Wrap(err, "failed to fetch resource: %q", kind)
			}

			applyfns[ii] = tracedApplyFn(fetchSpan, c.Tracer, kind, applyfn)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		for _, applyfn := range applyfns {
			if err := applyfn(ctx); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

// maxEventBatchSize bounds how many pending events the event loop drains
// into a single processing batch.
const maxEventBatchSize = 1024

// ProcessEvent hands a single event off to the appropriate collection for
// processing. See [Engine.processEventBatch].
func (c *Engine) ProcessEvent(ctx context.Context, event types.Event) error {
	return trace.Wrap(c.processEventBatch(ctx, []types.Event{event}))
}

// processEventBatch hands events off to the appropriate collections for
// processing. Runs of consecutive events of the same kind and operation are
// applied as a single store commit to amortize the copy-on-write write cost.
// Any resources which were not registered are ignored. After a run is
// applied, its events are emitted via the fanouts in order, so a fanout
// subscriber never observes an event whose effect is not yet visible in the
// cache.
func (c *Engine) processEventBatch(ctx context.Context, events []types.Event) error {
	for i := 0; i < len(events); {
		event := events[i]
		kind := ResourceKindFromResource(event.Resource)

		// find the run of consecutive events sharing this event's kind and
		// operation.
		j := i + 1
		for j < len(events) && events[j].Type == event.Type && ResourceKindFromResource(events[j].Resource) == kind {
			j++
		}
		run := events[i:j]
		i = j

		if handler, ok := c.Handlers[kind]; ok {
			switch event.Type {
			case types.OpDelete:
				resources := make([]types.Resource, 0, len(run))
				for _, e := range run {
					resources = append(resources, e.Resource)
				}
				if err := handler.OnDeletes(resources); err != nil {
					if !trace.IsNotFound(err) {
						c.Logger.WarnContext(ctx, "Failed to delete resource", "error", err)
						return trace.Wrap(err)
					}
				}
			case types.OpPut:
				resources := make([]types.Resource, 0, len(run))
				for _, e := range run {
					resources = append(resources, e.Resource)
				}
				if err := handler.OnPuts(resources); err != nil {
					return trace.Wrap(err)
				}
			default:
				c.Logger.WarnContext(ctx, "Skipping unsupported event type", "event", event.Type)
			}
		}

		for _, e := range run {
			c.eventsFanout.Emit(e)
			if !IsHighVolumeResource(kind.Kind) {
				c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
					f.Emit(e)
				})
			}
		}
	}

	return nil
}
