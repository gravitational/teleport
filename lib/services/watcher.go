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

package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// smallFanoutCapacity is the default capacity used for the circular event buffer allocated by
	// resource watchers that implement event fanout.
	smallFanoutCapacity = 128

	// eventBufferMaxSize is the maximum size of the event buffer used by resource watchers to
	// batch events that arrive in quick succession. In practice the event buffer should never
	// grow this large unless we're dealing with a truly massive teleport cluster.
	eventBufferMaxSize = 2048
)

// resourceCollector is a generic interface for maintaining an up-to-date view
// of a resource set being monitored. Used in conjunction with resourceWatcher.
type resourceCollector interface {
	// resourceKinds specifies the resource kind to watch.
	resourceKinds() []types.WatchKind
	// getResourcesAndUpdateCurrent is called when the resources should be
	// (re-)fetched directly.
	getResourcesAndUpdateCurrent(context.Context) error
	// processEventsAndUpdateCurrent is called when a watcher events are received. The event buffer
	// may be reused so implementers must not retain it, but implementers may mutate the buffer
	// in place during the call, e.g. in order to filter out undesired events before passing them
	// to a subsideary bulk-processor such as a fanout.
	processEventsAndUpdateCurrent(context.Context, []types.Event)
	// notifyStale is called when the maximum acceptable staleness (if specified)
	// is exceeded.
	notifyStale()
	// initializationChan is used to check if the initial state sync has
	// been completed.
	initializationChan() <-chan struct{}
}

func watchKindsString(kinds []types.WatchKind) string {
	var sb strings.Builder
	for i, k := range kinds {
		if i != 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(k.Kind)
		if k.SubKind != "" {
			sb.WriteString("/")
			sb.WriteString(k.SubKind)
		}
	}
	return sb.String()
}

// ResourceWatcherConfig configures resource watcher.
type ResourceWatcherConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock
	// Client is used to create new watchers
	Client types.Events
	// Logger emits log messages.
	Logger *slog.Logger
	// ResetC is a channel to notify of internal watcher reset (used in tests).
	ResetC chan time.Duration
	// Component is a component used in logs.
	Component string
	// MaxRetryPeriod is the maximum retry period on failed watchers.
	MaxRetryPeriod time.Duration
	// MaxStaleness is a maximum acceptable staleness for the locally maintained
	// resources, zero implies no staleness detection.
	MaxStaleness time.Duration
	// QueueSize is an optional queue size
	QueueSize int
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ResourceWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Component == "" {
		return trace.BadParameter("missing parameter Component")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxRetryPeriod == 0 {
		cfg.MaxRetryPeriod = defaults.MaxWatcherBackoff
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if cfg.ResetC == nil {
		cfg.ResetC = make(chan time.Duration, 1)
	}
	return nil
}

// newResourceWatcher returns a new instance of resourceWatcher.
// It is the caller's responsibility to verify the inputs' validity
// incl. cfg.CheckAndSetDefaults.
func newResourceWatcher(ctx context.Context, collector resourceCollector, cfg ResourceWatcherConfig) (*resourceWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.Logger = cfg.Logger.With("resource_kinds", watchKindsString(collector.resourceKinds()))
	ctx, cancel := context.WithCancel(ctx)
	p := &resourceWatcher{
		ResourceWatcherConfig: cfg,
		collector:             collector,
		ctx:                   ctx,
		cancel:                cancel,
		retry:                 retry,
		LoopC:                 make(chan struct{}),
		StaleC:                make(chan struct{}),
	}
	go p.runWatchLoop()
	return p, nil
}

// resourceWatcher monitors additions, updates and deletions
// to a set of resources.
type resourceWatcher struct {
	// failureStartedAt records when the current sync failures were first
	// detected, zero if there are no failures present.
	failureStartedAt time.Time
	collector        resourceCollector
	// ctx is a context controlling the lifetime of this resourceWatcher
	// instance.
	ctx context.Context
	// retry is used to manage backoff logic for watchers.
	retry  retryutils.Retry
	cancel context.CancelFunc
	// LoopC is a channel to check whether the watch loop is running
	// (used in tests).
	LoopC chan struct{}
	// StaleC is a channel that can trigger the condition of resource staleness
	// (used in tests).
	StaleC chan struct{}
	ResourceWatcherConfig
}

// Done returns a channel that signals resource watcher closure.
func (p *resourceWatcher) Done() <-chan struct{} {
	return p.ctx.Done()
}

// Close closes the resource watcher and cancels all the functions.
func (p *resourceWatcher) Close() {
	p.cancel()
}

// IsInitialized is a non-blocking way to check if resource watcher is already
// initialized.
func (p *resourceWatcher) IsInitialized() bool {
	select {
	case <-p.collector.initializationChan():
		return true
	default:
		return false
	}
}

// WaitInitialization blocks until resource watcher is fully initialized with
// the resources presented in auth server.
func (p *resourceWatcher) WaitInitialization() error {
	// wait for resourceWatcher to complete initialization.
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.collector.initializationChan():
			return nil
		case <-t.C:
			p.Logger.DebugContext(p.ctx, "ResourceWatcher is not yet initialized.")
		case <-p.ctx.Done():
			return trace.BadParameter("ResourceWatcher %s failed to initialize.", watchKindsString(p.collector.resourceKinds()))
		}
	}
}

// hasStaleView returns true when the local view has failed to be updated
// for longer than the MaxStaleness bound.
func (p *resourceWatcher) hasStaleView() bool {
	// Used for testing stale lock views.
	select {
	case <-p.StaleC:
		return true
	default:
	}

	if p.MaxStaleness == 0 || p.failureStartedAt.IsZero() {
		return false
	}
	return p.Clock.Since(p.failureStartedAt) > p.MaxStaleness
}

// runWatchLoop runs a watch loop.
func (p *resourceWatcher) runWatchLoop() {
	for {
		p.Logger.DebugContext(p.ctx, "Starting watch.")
		err := p.watch()

		select {
		case <-p.ctx.Done():
			return
		default:
		}

		if err != nil && p.failureStartedAt.IsZero() {
			// Note that failureStartedAt is zeroed in the watch routine immediately
			// after the local resource set has been successfully updated.
			p.failureStartedAt = p.Clock.Now()
		}
		if p.hasStaleView() {
			p.Logger.WarnContext(p.ctx, "Maximum staleness of period exceeded.", "max_staleness", p.MaxStaleness, "failure_started", p.failureStartedAt)
			p.collector.notifyStale()
		}

		// Used for testing that the watch routine has exited and is about
		// to be restarted.
		select {
		case p.ResetC <- p.retry.Duration():
		default:
		}

		startedWaiting := p.Clock.Now()
		select {
		case t := <-p.retry.After():
			p.Logger.DebugContext(p.ctx, "Attempting to restart watch after waiting", "waited", t.Sub(startedWaiting))
			p.retry.Inc()
		case <-p.ctx.Done():
			p.Logger.DebugContext(p.ctx, "Closed, returning from watch loop.")
			return
		case <-p.StaleC:
			// Used for testing that the watch routine is waiting for the
			// next restart attempt. We don't want to wait for the full
			// retry period in tests so we trigger the restart immediately.
			p.Logger.DebugContext(p.ctx, "Stale view, continue watch loop.")
		}
		if err != nil {
			p.Logger.WarnContext(p.ctx, "Restart watch on error", "error", err)
		}
	}
}

// watch monitors new resource updates, maintains a local view and broadcasts
// notifications to connected agents.
func (p *resourceWatcher) watch() error {
	watch := types.Watch{
		Name:            p.Component,
		MetricComponent: p.Component,
		Kinds:           p.collector.resourceKinds(),
	}

	if p.QueueSize > 0 {
		watch.QueueSize = p.QueueSize
	}
	watcher, err := p.Client.NewWatcher(p.ctx, watch)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

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
	// wrapping up with out of sync replica.
	// To avoid this, before doing fetch,
	// cache process makes sure the connection is established
	// by receiving init event first.
	select {
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
	case <-p.ctx.Done():
		return trace.ConnectionProblem(p.ctx.Err(), "context is closing")
	case <-p.StaleC:
		return trace.ConnectionProblem(nil, "stale view")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	if err := p.collector.getResourcesAndUpdateCurrent(p.ctx); err != nil {
		return trace.Wrap(err)
	}
	p.retry.Reset()
	p.failureStartedAt = time.Time{}

	// start out with a modestly sized event buffer
	eventBuf := make([]types.Event, 0, 16)
	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-p.ctx.Done():
			return trace.ConnectionProblem(p.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			// resource collectors want to process events in batches
			// when possible in order to reduce contention on their locks.
			// we therefore optimistically try to gather a large number of
			// events without blocking.
			eventBuf = append(eventBuf, event)
		CollectEvents:
			for len(eventBuf) < eventBufferMaxSize {
				select {
				case additionalEvent := <-watcher.Events():
					eventBuf = append(eventBuf, additionalEvent)
				default:
					break CollectEvents
				}
			}
			p.collector.processEventsAndUpdateCurrent(p.ctx, eventBuf)
			clear(eventBuf)
			eventBuf = eventBuf[:0]
		case p.LoopC <- struct{}{}:
			// Used in tests to detect the watch loop is running.
		case <-p.StaleC:
			return trace.ConnectionProblem(nil, "stale view")
		}
	}
}

// ProxyWatcherConfig is a ProxyWatcher configuration.
type ProxyWatcherConfig struct {
	// ProxyGetter is used to directly fetch the list of active proxies.
	ProxyGetter
	// ProxyDiffer is used to decide whether a put operation on an existing proxy should
	// trigger a event.
	ProxyDiffer func(old, new types.Server) bool
	// ProxiesC is a channel used to report the current proxy set. It receives
	// a fresh list at startup and subsequently a list of all known proxy
	// whenever an addition or deletion is detected.
	ProxiesC chan []types.Server
	ResourceWatcherConfig
}

// NewProxyWatcher returns a new instance of GenericWatcher that is configured
// to watch for changes.
func NewProxyWatcher(ctx context.Context, cfg ProxyWatcherConfig) (*GenericWatcher[types.Server, readonly.Server], error) {
	if cfg.ProxyGetter == nil {
		return nil, trace.BadParameter("ProxyGetter must be provided")
	}

	if cfg.ProxyDiffer == nil {
		cfg.ProxyDiffer = func(old, new types.Server) bool { return true }
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.Server, readonly.Server]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindProxy,
		ResourceKey:           types.Server.GetName,
		ResourceGetter: func(ctx context.Context) ([]types.Server, error) {
			return cfg.ProxyGetter.GetProxies()
		},
		ResourcesC:                          cfg.ProxiesC,
		ResourceDiffer:                      cfg.ProxyDiffer,
		RequireResourcesForInitialBroadcast: true,
		CloneFunc:                           types.Server.DeepCopy,
	})
	return w, trace.Wrap(err)
}

// DatabaseWatcherConfig is a DatabaseWatcher configuration.
type DatabaseWatcherConfig struct {
	// DatabaseGetter is responsible for fetching database resources.
	DatabaseGetter
	// DatabasesC receives up-to-date list of all database resources.
	DatabasesC chan []types.Database
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
}

// NewDatabaseWatcher returns a new instance of DatabaseWatcher.
func NewDatabaseWatcher(ctx context.Context, cfg DatabaseWatcherConfig) (*GenericWatcher[types.Database, readonly.Database], error) {
	if cfg.DatabaseGetter == nil {
		return nil, trace.BadParameter("DatabaseGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.Database, readonly.Database]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindDatabase,
		ResourceKey:           types.Database.GetName,
		ResourceGetter: func(ctx context.Context) ([]types.Database, error) {
			return cfg.DatabaseGetter.GetDatabases(ctx)
		},
		ResourcesC: cfg.DatabasesC,
		CloneFunc: func(resource types.Database) types.Database {
			return resource.Copy()
		},
	})
	return w, trace.Wrap(err)
}

// AppWatcherConfig is an AppWatcher configuration.
type AppWatcherConfig struct {
	// AppGetter is responsible for fetching application resources.
	AppGetter
	// AppsC receives up-to-date list of all application resources.
	AppsC chan []types.Application
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
}

// NewAppWatcher returns a new instance of AppWatcher.
func NewAppWatcher(ctx context.Context, cfg AppWatcherConfig) (*GenericWatcher[types.Application, readonly.Application], error) {
	if cfg.AppGetter == nil {
		return nil, trace.BadParameter("AppGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.Application, readonly.Application]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindApp,
		ResourceKey:           types.Application.GetName,
		ResourceGetter: func(ctx context.Context) ([]types.Application, error) {
			return cfg.AppGetter.GetApps(ctx)
		},
		ResourcesC: cfg.AppsC,
		CloneFunc: func(resource types.Application) types.Application {
			return resource.Copy()
		},
	})

	return w, trace.Wrap(err)
}

// KubeServerWatcherConfig is an KubeServerWatcher configuration.
type KubeServerWatcherConfig struct {
	// KubernetesServerGetter is responsible for fetching kube_server resources.
	KubernetesServerGetter
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
}

// NewKubeServerWatcher returns a new instance of KubeServerWatcher.
func NewKubeServerWatcher(ctx context.Context, cfg KubeServerWatcherConfig) (*GenericWatcher[types.KubeServer, readonly.KubeServer], error) {
	if cfg.KubernetesServerGetter == nil {
		return nil, trace.BadParameter("KubernetesServerGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.KubeServer, readonly.KubeServer]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindKubeServer,
		ResourceGetter: func(ctx context.Context) ([]types.KubeServer, error) {
			return cfg.KubernetesServerGetter.GetKubernetesServers(ctx)
		},
		ResourceKey: func(resource types.KubeServer) string {
			return resource.GetHostID() + resource.GetName()
		},
		DisableUpdateBroadcast: true,
		CloneFunc:              types.KubeServer.Copy,
	})
	return w, trace.Wrap(err)
}

// KubeClusterWatcherConfig is an KubeClusterWatcher configuration.
type KubeClusterWatcherConfig struct {
	// KubernetesGetter is responsible for fetching kube_cluster resources.
	KubernetesClusterGetter
	// KubeClustersC receives up-to-date list of all kube_cluster resources.
	KubeClustersC chan []types.KubeCluster
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
}

// NewKubeClusterWatcher returns a new instance of KubeClusterWatcher.
func NewKubeClusterWatcher(ctx context.Context, cfg KubeClusterWatcherConfig) (*GenericWatcher[types.KubeCluster, readonly.KubeCluster], error) {
	if cfg.KubernetesClusterGetter == nil {
		return nil, trace.BadParameter("KubernetesClusterGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.KubeCluster, readonly.KubeCluster]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindKubernetesCluster,
		ResourceGetter: func(ctx context.Context) ([]types.KubeCluster, error) {
			return cfg.KubernetesClusterGetter.GetKubernetesClusters(ctx)
		},
		ResourceKey: types.KubeCluster.GetName,
		ResourcesC:  cfg.KubeClustersC,
		CloneFunc: func(resource types.KubeCluster) types.KubeCluster {
			return resource.Copy()
		},
	})
	return w, trace.Wrap(err)
}

type DynamicWindowsDesktopGetter interface {
	ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error)
}

// DynamicWindowsDesktopWatcherConfig is a DynamicWindowsDesktopWatcher configuration.
type DynamicWindowsDesktopWatcherConfig struct {
	// DynamicWindowsDesktopGetter is responsible for fetching DynamicWindowsDesktop resources.
	DynamicWindowsDesktopGetter
	// DynamicWindowsDesktopsC receives up-to-date list of all DynamicWindowsDesktop resources.
	DynamicWindowsDesktopsC chan []types.DynamicWindowsDesktop
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
}

// NewDynamicWindowsDesktopWatcher returns a new instance of DynamicWindowsDesktopWatcher.
func NewDynamicWindowsDesktopWatcher(ctx context.Context, cfg DynamicWindowsDesktopWatcherConfig) (*GenericWatcher[types.DynamicWindowsDesktop, readonly.DynamicWindowsDesktop], error) {
	if cfg.DynamicWindowsDesktopGetter == nil {
		return nil, trace.BadParameter("KubernetesClusterGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.DynamicWindowsDesktop, readonly.DynamicWindowsDesktop]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindDynamicWindowsDesktop,
		ResourceGetter: func(ctx context.Context) ([]types.DynamicWindowsDesktop, error) {
			var desktops []types.DynamicWindowsDesktop
			next := ""
			for {
				d, token, err := cfg.DynamicWindowsDesktopGetter.ListDynamicWindowsDesktops(ctx, defaults.MaxIterationLimit, next)
				if err != nil {
					return nil, err
				}
				desktops = append(desktops, d...)
				if token == "" {
					break
				}
				next = token
			}
			return desktops, nil
		},
		ResourceKey: types.DynamicWindowsDesktop.GetName,
		ResourcesC:  cfg.DynamicWindowsDesktopsC,
		CloneFunc: func(resource types.DynamicWindowsDesktop) types.DynamicWindowsDesktop {
			return resource.Copy()
		},
	})
	return w, trace.Wrap(err)
}

// GenericWatcherConfig is a generic resource watcher configuration.
type GenericWatcherConfig[T any, R any] struct {
	// ResourceGetter is used to directly fetch the current set of resources.
	ResourceGetter func(context.Context) ([]T, error)
	// ResourceDiffer is used to decide whether a put operation on an existing ResourceGetter should
	// trigger an event.
	ResourceDiffer func(old, new T) bool
	// ResourceKey defines how the resources should be keyed.
	ResourceKey func(resource T) string
	// ResourcesC is a channel used to report the current resourxe set. It receives
	// a fresh list at startup and subsequently a list of all known resourxes
	// whenever an addition or deletion is detected.
	ResourcesC chan []T
	// CloneFunc defines how a resource is cloned. All resources provided via
	// the broadcast mechanism, or retrieved via [GenericWatcer.CurrentResources]
	// or [GenericWatcher.CurrentResourcesWithFilter] will be cloned by this
	// mechanism before being provided to callers.
	CloneFunc func(resource T) T
	ResourceWatcherConfig
	// ResourceKind specifies the kind of resource the watcher is monitoring.
	ResourceKind string
	// RequireResourcesForInitialBroadcast indicates whether an update should be
	// performed if the initial set of resources is empty.
	RequireResourcesForInitialBroadcast bool
	// DisableUpdateBroadcast turns off emitting updates on changes. When this
	// mode is opted into, users must invoke [GenericWatcher.CurrentResources] or
	// [GenericWatcher.CurrentResourcesWithFilter] manually to retrieve the active
	// resource set.
	DisableUpdateBroadcast bool
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *GenericWatcherConfig[T, R]) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if cfg.ResourceGetter == nil {
		return trace.BadParameter("ResourceGetter not provided to generic resource watcher")
	}

	if cfg.ResourceKind == "" {
		return trace.BadParameter("ResourceKind not provided to generic resource watcher")
	}

	if cfg.ResourceKey == nil {
		return trace.BadParameter("ResourceKey not provided to generic resource watcher")
	}

	if cfg.ResourceDiffer == nil {
		cfg.ResourceDiffer = func(T, T) bool { return true }
	}

	if cfg.ResourcesC == nil {
		cfg.ResourcesC = make(chan []T)
	}
	return nil
}

// NewGenericResourceWatcher returns a new instance of resource watcher.
func NewGenericResourceWatcher[T any, R any](ctx context.Context, cfg GenericWatcherConfig[T, R]) (*GenericWatcher[T, R], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     3 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	collector := &genericCollector[T, R]{
		GenericWatcherConfig: cfg,
		initializationC:      make(chan struct{}),
		cache:                cache,
	}
	collector.stale.Store(true)
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &GenericWatcher[T, R]{watcher, collector}, nil
}

// GenericWatcher is built on top of resourceWatcher to monitor additions
// and deletions to the set of resources.
type GenericWatcher[T any, R any] struct {
	*resourceWatcher
	*genericCollector[T, R]
}

// ResourceCount returns the current number of resources known to the watcher.
func (g *GenericWatcher[T, R]) ResourceCount() int {
	g.rw.RLock()
	defer g.rw.RUnlock()
	return len(g.current)
}

// CurrentResources returns a copy of the resources known to the watcher.
func (g *GenericWatcher[T, R]) CurrentResources(ctx context.Context) ([]T, error) {
	if err := g.refreshStaleResources(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	g.rw.RLock()
	defer g.rw.RUnlock()

	return resourcesToSlice(g.current, g.CloneFunc), nil
}

// CurrentResourcesWithFilter returns a copy of the resources known to the watcher
// that match the provided filter.
func (g *GenericWatcher[T, R]) CurrentResourcesWithFilter(ctx context.Context, filter func(R) bool) ([]T, error) {
	if err := g.refreshStaleResources(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	g.rw.RLock()
	defer g.rw.RUnlock()

	r := func(a any) R {
		return a.(R)
	}

	var out []T
	for _, resource := range g.current {
		if filter(r(resource)) {
			out = append(out, g.CloneFunc(resource))
		}
	}

	return out, nil
}

// genericCollector accompanies resourceWatcher when monitoring proxies.
type genericCollector[T any, R any] struct {
	GenericWatcherConfig[T, R]
	// current holds a map of the currently known resources (keyed by server name,
	// RWMutex protected).
	current         map[string]T
	initializationC chan struct{}
	// cache is a helper for temporarily storing the results of CurrentResources.
	// It's used to limit the number of calls to the backend.
	cache *utils.FnCache
	rw    sync.RWMutex
	once  sync.Once
	// stale is used to indicate that the watcher is stale and needs to be
	// refreshed.
	stale atomic.Bool
}

// resourceKinds specifies the resource kind to watch.
func (g *genericCollector[T, R]) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: g.ResourceKind}}
}

// getResources gets the list of current resources.
func (g *genericCollector[T, R]) getResources(ctx context.Context) (map[string]T, error) {
	resources, err := g.GenericWatcherConfig.ResourceGetter(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	current := make(map[string]T, len(resources))
	for _, resource := range resources {
		current[g.GenericWatcherConfig.ResourceKey(resource)] = resource
	}
	return current, nil
}

func (g *genericCollector[T, R]) refreshStaleResources(ctx context.Context) error {
	if !g.stale.Load() {
		return nil
	}

	_, err := utils.FnCacheGet(ctx, g.cache, g.GenericWatcherConfig.ResourceKind, func(ctx context.Context) (any, error) {
		current, err := g.getResources(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// There is a chance that the watcher reinitialized while
		// getting resources happened above. Check if we are still stale
		if g.stale.CompareAndSwap(true, false) {
			g.rw.Lock()
			g.current = current
			g.rw.Unlock()
		}

		return nil, nil
	})

	return trace.Wrap(err)
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (g *genericCollector[T, R]) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newCurrent, err := g.getResources(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	g.rw.Lock()
	defer g.rw.Unlock()
	g.current = newCurrent
	g.stale.Store(false)
	// Only emit an empty set of resources if the watcher is already initialized,
	// or if explicitly opted into by for the watcher.
	if len(newCurrent) > 0 || g.isInitialized() ||
		(!g.RequireResourcesForInitialBroadcast && len(newCurrent) == 0) {
		g.broadcastUpdate(ctx)
	}
	g.defineCollectorAsInitialized()
	return nil
}

func (g *genericCollector[T, R]) defineCollectorAsInitialized() {
	g.once.Do(func() {
		// mark watcher as initialized.
		close(g.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (g *genericCollector[T, R]) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	g.rw.Lock()
	defer g.rw.Unlock()

	var updated bool

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != g.ResourceKind {
			g.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}

		switch event.Type {
		case types.OpDelete:
			// On delete events, the server description is populated with the host ID.
			delete(g.current, event.Resource.GetMetadata().Description+event.Resource.GetName())
			// Always broadcast when a resource is deleted.
			updated = true
		case types.OpPut:
			resource, ok := event.Resource.(T)
			if !ok {
				g.Logger.WarnContext(ctx, "Received unexpected type", "resource", event.Resource.GetKind())
				continue
			}

			key := g.ResourceKey(resource)
			current, exists := g.current[key]
			g.current[key] = resource
			updated = !exists || g.ResourceDiffer(current, resource)
		default:
			g.Logger.WarnContext(ctx, "Skipping unsupported event type", "event_type", event.Type)
		}
	}

	if updated {
		g.broadcastUpdate(ctx)
	}
}

// broadcastUpdate broadcasts information about updating the resource set.
func (g *genericCollector[T, R]) broadcastUpdate(ctx context.Context) {
	if g.DisableUpdateBroadcast {
		return
	}

	names := make([]string, 0, len(g.current))
	for k := range g.current {
		names = append(names, k)
	}
	g.Logger.DebugContext(ctx, "List of known resources updated", "resources", names)

	select {
	case g.ResourcesC <- resourcesToSlice(g.current, g.CloneFunc):
	case <-ctx.Done():
	}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (g *genericCollector[T, R]) initializationChan() <-chan struct{} {
	return g.initializationC
}

func (g *genericCollector[T, R]) isInitialized() bool {
	select {
	case <-g.initializationC:
		return true
	default:
		return false
	}
}

func (g *genericCollector[T, R]) notifyStale() {
	g.stale.Store(true)
}

// LockWatcherConfig is a LockWatcher configuration.
type LockWatcherConfig struct {
	LockGetter
	ResourceWatcherConfig
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *LockWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.MaxStaleness == 0 {
		cfg.MaxStaleness = defaults.LockMaxStaleness
	}
	if cfg.LockGetter == nil {
		getter, ok := cfg.Client.(LockGetter)
		if !ok {
			return trace.BadParameter("missing parameter LockGetter and Client not usable as LockGetter")
		}
		cfg.LockGetter = getter
	}
	return nil
}

// NewLockWatcher returns a new instance of LockWatcher.
func NewLockWatcher(ctx context.Context, cfg LockWatcherConfig) (*LockWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &lockCollector{
		LockWatcherConfig: cfg,
		fanout: NewFanoutV2(FanoutV2Config{
			Capacity: smallFanoutCapacity,
		}),
		initializationC: make(chan struct{}),
	}
	// Resource watcher require the fanout to be initialized before passing in.
	// Otherwise, Emit() may fail due to a race condition mentioned in https://github.com/gravitational/teleport/issues/19289
	collector.fanout.SetInit(collector.resourceKinds())
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &LockWatcher{watcher, collector}, nil
}

// LockWatcher is built on top of resourceWatcher to monitor changes to locks.
type LockWatcher struct {
	*resourceWatcher
	*lockCollector
}

// lockCollector accompanies resourceWatcher when monitoring locks.
type lockCollector struct {
	LockWatcherConfig
	// current holds a map of the currently known locks (keyed by lock name).
	current map[string]types.Lock
	// fanout provides support for multiple subscribers to the lock updates.
	fanout *FanoutV2
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	// currentRW is a mutex protecting both current and isStale.
	currentRW sync.RWMutex
	once      sync.Once
	// isStale indicates whether the local lock view (current) is stale.
	isStale bool
}

// IsStale is used to check whether the lock watcher is stale.
// Used in tests.
func (p *lockCollector) IsStale() bool {
	p.currentRW.RLock()
	defer p.currentRW.RUnlock()
	return p.isStale
}

// Subscribe is used to subscribe to the lock updates.
func (p *lockCollector) Subscribe(ctx context.Context, targets ...types.LockTarget) (types.Watcher, error) {
	watchKinds, err := lockTargetsToWatchKinds(targets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sub, err := p.fanout.NewWatcher(ctx, types.Watch{Kinds: watchKinds})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	select {
	case event := <-sub.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-sub.Done():
		return nil, trace.Wrap(sub.Error())
	}
	return sub, nil
}

// CheckLockInForce returns an AccessDenied error if there is a lock in force
// matching at least one of the targets.
func (p *lockCollector) CheckLockInForce(mode constants.LockingMode, targets ...types.LockTarget) error {
	p.currentRW.RLock()
	defer p.currentRW.RUnlock()
	if p.isStale && mode == constants.LockingModeStrict {
		return StrictLockingModeAccessDenied
	}
	if lock := p.findLockInForceUnderMutex(targets); lock != nil {
		return LockInForceAccessDenied(lock)
	}
	return nil
}

func (p *lockCollector) findLockInForceUnderMutex(targets []types.LockTarget) types.Lock {
	for _, lock := range p.current {
		if !lock.IsInForce(p.Clock.Now()) {
			continue
		}
		if len(targets) == 0 {
			return lock
		}
		for _, target := range targets {
			if target.Match(lock) {
				return lock
			}
		}
	}
	return nil
}

// GetCurrent returns the currently stored locks.
func (p *lockCollector) GetCurrent() []types.Lock {
	p.currentRW.RLock()
	defer p.currentRW.RUnlock()
	return lockMapValues(p.current)
}

// resourceKinds specifies the resource kind to watch.
func (p *lockCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindLock}}
}

// initializationChan is used to check that the cache has done its initial
// sync
func (p *lockCollector) initializationChan() <-chan struct{} {
	return p.initializationC
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (p *lockCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	locks, err := p.LockGetter.GetLocks(ctx, true)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := map[string]types.Lock{}
	for _, lock := range locks {
		newCurrent[lock.GetName()] = lock
	}

	p.currentRW.Lock()
	defer p.currentRW.Unlock()
	p.current = newCurrent
	p.isStale = false
	p.defineCollectorAsInitialized()
	for _, lock := range p.current {
		p.fanout.Emit(types.Event{Type: types.OpPut, Resource: lock})
	}
	return nil
}

func (p *lockCollector) defineCollectorAsInitialized() {
	p.once.Do(func() {
		// mark watcher as initialized.
		close(p.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (p *lockCollector) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	p.currentRW.Lock()
	defer p.currentRW.Unlock()
	eventsToEmit := events[:0]
	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindLock {
			p.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}

		switch event.Type {
		case types.OpDelete:
			delete(p.current, event.Resource.GetName())
			eventsToEmit = append(eventsToEmit, event)
		case types.OpPut:
			lock, ok := event.Resource.(types.Lock)
			if !ok {
				p.Logger.WarnContext(ctx, "Unexpected resource type", "resource", event.Resource.GetKind())
				continue
			}
			if lock.IsInForce(p.Clock.Now()) {
				p.current[lock.GetName()] = lock
				eventsToEmit = append(eventsToEmit, event)
			} else {
				delete(p.current, lock.GetName())
			}
		default:
			p.Logger.WarnContext(ctx, "Skipping unsupported event type", "event_type", event.Type)
		}
	}
	p.fanout.Emit(eventsToEmit...)
}

// notifyStale is called when the maximum acceptable staleness (if specified)
// is exceeded.
func (p *lockCollector) notifyStale() {
	p.currentRW.Lock()
	defer p.currentRW.Unlock()

	p.fanout.Emit(types.Event{Type: types.OpUnreliable})

	// Do not clear p.current here, the most recent lock set may still be used
	// with LockingModeBestEffort.
	p.isStale = true
}

func lockTargetsToWatchKinds(targets []types.LockTarget) ([]types.WatchKind, error) {
	watchKinds := make([]types.WatchKind, 0, len(targets))
	for _, target := range targets {
		if target.IsEmpty() {
			continue
		}
		filter, err := target.IntoMap()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		watchKinds = append(watchKinds, types.WatchKind{
			Kind:   types.KindLock,
			Filter: filter,
		})
	}
	if len(watchKinds) == 0 {
		watchKinds = []types.WatchKind{{Kind: types.KindLock}}
	}
	return watchKinds, nil
}

func lockMapValues(lockMap map[string]types.Lock) []types.Lock {
	locks := make([]types.Lock, 0, len(lockMap))
	for _, lock := range lockMap {
		locks = append(locks, lock)
	}
	return locks
}

func resourcesToSlice[T any](resources map[string]T, cloneFunc func(T) T) (slice []T) {
	for _, resource := range resources {
		slice = append(slice, cloneFunc(resource))
	}
	return slice
}

// CertAuthorityWatcherConfig is a CertAuthorityWatcher configuration.
type CertAuthorityWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// AuthorityGetter is responsible for fetching cert authority resources.
	AuthorityGetter
	// Types restricts which cert authority types are retrieved via the AuthorityGetter.
	Types []types.CertAuthType
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *CertAuthorityWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.AuthorityGetter == nil {
		getter, ok := cfg.Client.(AuthorityGetter)
		if !ok {
			return trace.BadParameter("missing parameter AuthorityGetter and Client not usable as AuthorityGetter")
		}
		cfg.AuthorityGetter = getter
	}
	if len(cfg.Types) == 0 {
		return trace.BadParameter("missing parameter Types")
	}
	return nil
}

// NewCertAuthorityWatcher returns a new instance of CertAuthorityWatcher.
func NewCertAuthorityWatcher(ctx context.Context, cfg CertAuthorityWatcherConfig) (*CertAuthorityWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	collector := &caCollector{
		CertAuthorityWatcherConfig: cfg,
		fanout: NewFanoutV2(FanoutV2Config{
			Capacity: smallFanoutCapacity,
		}),
		cas:             make(map[types.CertAuthType]map[string]types.CertAuthority, len(cfg.Types)),
		filter:          make(types.CertAuthorityFilter, len(cfg.Types)),
		initializationC: make(chan struct{}),
	}

	for _, t := range cfg.Types {
		collector.cas[t] = make(map[string]types.CertAuthority)
		collector.filter[t] = types.Wildcard
	}
	// Resource watcher require the fanout to be initialized before passing in.
	// Otherwise, Emit() may fail due to a race condition mentioned in https://github.com/gravitational/teleport/issues/19289
	collector.fanout.SetInit(collector.resourceKinds())
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CertAuthorityWatcher{watcher, collector}, nil
}

// CertAuthorityWatcher is built on top of resourceWatcher to monitor cert authority resources.
type CertAuthorityWatcher struct {
	*resourceWatcher
	*caCollector
}

// caCollector accompanies resourceWatcher when monitoring cert authority resources.
type caCollector struct {
	fanout *FanoutV2
	cas    map[types.CertAuthType]map[string]types.CertAuthority
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	filter          types.CertAuthorityFilter
	CertAuthorityWatcherConfig
	// lock protects concurrent access to cas
	lock sync.RWMutex
	once sync.Once
}

// Subscribe is used to subscribe to the lock updates.
func (c *caCollector) Subscribe(ctx context.Context, filter types.CertAuthorityFilter) (types.Watcher, error) {
	if len(filter) == 0 {
		filter = c.filter
	}
	watch := types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind:   types.KindCertAuthority,
				Filter: filter.IntoMap(),
			},
		},
	}
	sub, err := c.fanout.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	select {
	case event := <-sub.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-sub.Done():
		return nil, trace.Wrap(sub.Error())
	}
	return sub, nil
}

// resourceKinds specifies the resource kind to watch.
func (c *caCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindCertAuthority, Filter: c.filter.IntoMap()}}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (c *caCollector) initializationChan() <-chan struct{} {
	return c.initializationC
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (c *caCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	var cas []types.CertAuthority

	for _, t := range c.Types {
		authorities, err := c.AuthorityGetter.GetCertAuthorities(ctx, t, false)
		if err != nil {
			return trace.Wrap(err)
		}

		cas = append(cas, authorities...)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	for _, ca := range cas {
		if !c.watchingType(ca.GetType()) {
			continue
		}

		c.cas[ca.GetType()][ca.GetName()] = ca
		c.fanout.Emit(types.Event{Type: types.OpPut, Resource: ca.Clone()})
	}

	c.defineCollectorAsInitialized()

	return nil
}

func (c *caCollector) defineCollectorAsInitialized() {
	c.once.Do(func() {
		// mark watcher as initialized.
		close(c.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (c *caCollector) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.lock.Lock()
	defer c.lock.Unlock()

	eventsToEmit := events[:0]

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindCertAuthority {
			c.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}
		switch event.Type {
		case types.OpDelete:
			caType := types.CertAuthType(event.Resource.GetSubKind())
			if !c.watchingType(caType) {
				continue
			}

			delete(c.cas[caType], event.Resource.GetName())
			eventsToEmit = append(eventsToEmit, event)
		case types.OpPut:
			ca, ok := event.Resource.(types.CertAuthority)
			if !ok {
				c.Logger.WarnContext(ctx, "Received unexpected resource type", "resource", event.Resource.GetKind())
				continue
			}

			if !c.watchingType(ca.GetType()) {
				continue
			}

			authority, ok := c.cas[ca.GetType()][ca.GetName()]
			if ok && CertAuthoritiesEquivalent(authority, ca) {
				continue
			}

			c.cas[ca.GetType()][ca.GetName()] = ca
			eventsToEmit = append(eventsToEmit, event)
		default:
			c.Logger.WarnContext(ctx, "Received unsupported event type", "event_type", event.Type)
		}
	}

	c.fanout.Emit(eventsToEmit...)
}

func (c *caCollector) watchingType(t types.CertAuthType) bool {
	if _, ok := c.cas[t]; ok {
		return true
	}
	return false
}

func (c *caCollector) notifyStale() {}

// NodeWatcherConfig is a NodeWatcher configuration.
type NodeWatcherConfig struct {
	// NodesGetter is used to directly fetch the list of active nodes.
	NodesGetter
	ResourceWatcherConfig
}

// NewNodeWatcher returns a new instance of NodeWatcher.
func NewNodeWatcher(ctx context.Context, cfg NodeWatcherConfig) (*GenericWatcher[types.Server, readonly.Server], error) {
	if cfg.NodesGetter == nil {
		return nil, trace.BadParameter("NodesGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.Server, readonly.Server]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindNode,
		ResourceGetter: func(ctx context.Context) ([]types.Server, error) {
			return cfg.NodesGetter.GetNodes(ctx, apidefaults.Namespace)
		},
		ResourceKey:            types.Server.GetName,
		DisableUpdateBroadcast: true,
		CloneFunc:              types.Server.DeepCopy,
	})
	return w, trace.Wrap(err)
}

// AccessRequestWatcherConfig is a AccessRequestWatcher configuration.
type AccessRequestWatcherConfig struct {
	// AccessRequestGetter is responsible for fetching access request resources.
	AccessRequestGetter
	// AccessRequestsC receives up-to-date list of all access request resources.
	AccessRequestsC chan types.AccessRequests
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// Filter is the filter to use to monitor access requests.
	Filter types.AccessRequestFilter
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *AccessRequestWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.AccessRequestGetter == nil {
		getter, ok := cfg.Client.(AccessRequestGetter)
		if !ok {
			return trace.BadParameter("missing parameter AccessRequestGetter and Client not usable as AccessRequestGetter")
		}
		cfg.AccessRequestGetter = getter
	}
	if cfg.AccessRequestsC == nil {
		cfg.AccessRequestsC = make(chan types.AccessRequests)
	}
	return nil
}

// NewAccessRequestWatcher returns a new instance of AccessRequestWatcher.
func NewAccessRequestWatcher(ctx context.Context, cfg AccessRequestWatcherConfig) (*AccessRequestWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &accessRequestCollector{
		AccessRequestWatcherConfig: cfg,
		initializationC:            make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AccessRequestWatcher{watcher, collector}, nil
}

// AccessRequestWatcher is built on top of resourceWatcher to monitor access request resources.
type AccessRequestWatcher struct {
	*resourceWatcher
	*accessRequestCollector
}

// accessRequestCollector accompanies resourceWatcher when monitoring access request resources.
type accessRequestCollector struct {
	// AccessRequestWatcherConfig is the watcher configuration.
	AccessRequestWatcherConfig
	// current holds a map of the currently known access request resources.
	current map[string]types.AccessRequest
	// initializationC is used to check that the watcher has been initialized properly.
	initializationC chan struct{}
	// lock protects the "current" map.
	lock sync.RWMutex
	once sync.Once
}

// resourceKinds specifies the resource kind to watch.
func (p *accessRequestCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindAccessRequest}}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (p *accessRequestCollector) initializationChan() <-chan struct{} {
	return p.initializationC
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (p *accessRequestCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	accessRequests, err := p.AccessRequestGetter.GetAccessRequests(ctx, p.Filter)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := make(map[string]types.AccessRequest, len(accessRequests))
	for _, accessRequest := range accessRequests {
		newCurrent[accessRequest.GetName()] = accessRequest
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.current = newCurrent
	p.defineCollectorAsInitialized()

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case p.AccessRequestsC <- accessRequests:
	}

	return nil
}

func (p *accessRequestCollector) defineCollectorAsInitialized() {
	p.once.Do(func() {
		// mark watcher as initialized.
		close(p.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (p *accessRequestCollector) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindAccessRequest {
			p.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}
		switch event.Type {
		case types.OpDelete:
			delete(p.current, event.Resource.GetName())
			select {
			case <-ctx.Done():
			case p.AccessRequestsC <- resourcesToSlice(p.current, types.AccessRequest.Copy):
			}
		case types.OpPut:
			accessRequest, ok := event.Resource.(types.AccessRequest)
			if !ok {
				p.Logger.WarnContext(ctx, "Received unexpected resource type", "resource", event.Resource.GetKind())
				continue
			}
			p.current[accessRequest.GetName()] = accessRequest
			select {
			case <-ctx.Done():
			case p.AccessRequestsC <- resourcesToSlice(p.current, types.AccessRequest.Copy):
			}

		default:
			p.Logger.WarnContext(ctx, "Received unsupported event type", "event_type", event.Type)
		}
	}
}

func (*accessRequestCollector) notifyStale() {}

// OktaAssignmentWatcherConfig is a OktaAssignmentWatcher configuration.
type OktaAssignmentWatcherConfig struct {
	// OktaAssignments is responsible for fetching Okta assignments.
	OktaAssignments OktaAssignmentsGetter
	// OktaAssignmentsC receives up-to-date list of all Okta assignment resources.
	OktaAssignmentsC chan types.OktaAssignments
	// RWCfg is the resource watcher configuration.
	RWCfg ResourceWatcherConfig
	// PageSize is the number of Okta assignments to list at a time.
	PageSize int
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *OktaAssignmentWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.RWCfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.OktaAssignments == nil {
		assignments, ok := cfg.RWCfg.Client.(OktaAssignmentsGetter)
		if !ok {
			return trace.BadParameter("missing parameter OktaAssignments and Client not usable as OktaAssignments")
		}
		cfg.OktaAssignments = assignments
	}
	if cfg.OktaAssignmentsC == nil {
		cfg.OktaAssignmentsC = make(chan types.OktaAssignments)
	}
	return nil
}

// NewOktaAssignmentWatcher returns a new instance of OktaAssignmentWatcher. The context here will be used to
// exit early from the resource watcher if needed.
func NewOktaAssignmentWatcher(ctx context.Context, cfg OktaAssignmentWatcherConfig) (*OktaAssignmentWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &oktaAssignmentCollector{
		logger:          cfg.RWCfg.Logger,
		cfg:             cfg,
		initializationC: make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.RWCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &OktaAssignmentWatcher{
		resourceWatcher: watcher,
		collector:       collector,
	}, nil
}

// OktaAssignmentWatcher is built on top of resourceWatcher to monitor Okta assignment resources.
type OktaAssignmentWatcher struct {
	resourceWatcher *resourceWatcher
	collector       *oktaAssignmentCollector
}

// CollectorChan is the channel that collects the Okta assignments.
func (o *OktaAssignmentWatcher) CollectorChan() chan types.OktaAssignments {
	return o.collector.cfg.OktaAssignmentsC
}

// Close closes the underlying resource watcher
func (o *OktaAssignmentWatcher) Close() {
	o.resourceWatcher.Close()
}

// Done returns the channel that signals watcher closer.
func (o *OktaAssignmentWatcher) Done() <-chan struct{} {
	return o.resourceWatcher.Done()
}

// oktaAssignmentCollector accompanies resourceWatcher when monitoring Okta assignment resources.
type oktaAssignmentCollector struct {
	// OktaAssignmentWatcherConfig is the watcher configuration.
	cfg    OktaAssignmentWatcherConfig
	logger *slog.Logger
	// current holds a map of the currently known Okta assignment resources.
	current map[string]types.OktaAssignment
	// initializationC is used to check that the watcher has been initialized properly.
	initializationC chan struct{}
	// mu guards "current"
	mu   sync.RWMutex
	once sync.Once
}

// resourceKinds specifies the resource kind to watch.
func (*oktaAssignmentCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindOktaAssignment}}
}

// initializationChan is used to check if the initial state sync has been completed.
func (c *oktaAssignmentCollector) initializationChan() <-chan struct{} {
	return c.initializationC
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (c *oktaAssignmentCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	var oktaAssignments []types.OktaAssignment
	var nextToken string
	for {
		var oktaAssignmentsPage []types.OktaAssignment
		var err error
		oktaAssignmentsPage, nextToken, err = c.cfg.OktaAssignments.ListOktaAssignments(ctx, c.cfg.PageSize, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		oktaAssignments = append(oktaAssignments, oktaAssignmentsPage...)
		if nextToken == "" {
			break
		}
	}

	newCurrent := make(map[string]types.OktaAssignment, len(oktaAssignments))
	for _, oktaAssignment := range oktaAssignments {
		newCurrent[oktaAssignment.GetName()] = oktaAssignment
	}
	c.mu.Lock()
	c.current = newCurrent
	c.defineCollectorAsInitialized()
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case c.cfg.OktaAssignmentsC <- oktaAssignments:
	}

	return nil
}

func (c *oktaAssignmentCollector) defineCollectorAsInitialized() {
	c.once.Do(func() {
		close(c.initializationC)
	})
}

// processEventsAndUpdateCurrent is called when a watcher event is received.
func (c *oktaAssignmentCollector) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindOktaAssignment {
			c.logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}
		switch event.Type {
		case types.OpDelete:
			delete(c.current, event.Resource.GetName())
			resources := resourcesToSlice(c.current, types.OktaAssignment.Copy)
			select {
			case <-ctx.Done():
			case c.cfg.OktaAssignmentsC <- resources:
			}
		case types.OpPut:
			oktaAssignment, ok := event.Resource.(types.OktaAssignment)
			if !ok {
				c.logger.WarnContext(ctx, "Received unexpected resource type", "resource", event.Resource.GetKind())
				continue
			}
			c.current[oktaAssignment.GetName()] = oktaAssignment
			resources := resourcesToSlice(c.current, types.OktaAssignment.Copy)

			select {
			case <-ctx.Done():
			case c.cfg.OktaAssignmentsC <- resources:
			}

		default:
			c.logger.WarnContext(ctx, "Received unsupported event type", "event_type", event.Type)
		}
	}
}

func (*oktaAssignmentCollector) notifyStale() {}

// GitServerWatcherConfig is the config for Git server watcher.
type GitServerWatcherConfig struct {
	GitServerGetter
	ResourceWatcherConfig

	// EnableUpdateBroadcast turns on emitting updates on changes. Broadcast is
	// opt-in for Git Server watcher.
	EnableUpdateBroadcast bool
}

// NewGitServerWatcher returns a new instance of Git server watcher.
func NewGitServerWatcher(ctx context.Context, cfg GitServerWatcherConfig) (*GenericWatcher[types.Server, readonly.Server], error) {
	if cfg.GitServerGetter == nil {
		return nil, trace.BadParameter("NodesGetter must be provided")
	}

	w, err := NewGenericResourceWatcher(ctx, GenericWatcherConfig[types.Server, readonly.Server]{
		ResourceWatcherConfig: cfg.ResourceWatcherConfig,
		ResourceKind:          types.KindGitServer,
		ResourceGetter: func(ctx context.Context) (all []types.Server, err error) {
			var page []types.Server
			var token string
			for {
				page, token, err = cfg.GitServerGetter.ListGitServers(ctx, apidefaults.DefaultChunkSize, token)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				all = append(all, page...)
				if token == "" {
					break
				}
			}
			return all, nil
		},
		ResourceKey:            types.Server.GetName,
		DisableUpdateBroadcast: !cfg.EnableUpdateBroadcast,
		CloneFunc:              types.Server.DeepCopy,
	})
	return w, trace.Wrap(err)
}
