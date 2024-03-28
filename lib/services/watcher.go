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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// smallFanoutCapacity is the default capacity used for the circular event buffer allocated by
	// resource watchers that implement event fanout.
	smallFanoutCapacity = 128
)

// resourceCollector is a generic interface for maintaining an up-to-date view
// of a resource set being monitored. Used in conjunction with resourceWatcher.
type resourceCollector interface {
	// resourceKinds specifies the resource kind to watch.
	resourceKinds() []types.WatchKind
	// getResourcesAndUpdateCurrent is called when the resources should be
	// (re-)fetched directly.
	getResourcesAndUpdateCurrent(context.Context) error
	// processEventAndUpdateCurrent is called when a watcher event is received.
	processEventAndUpdateCurrent(context.Context, types.Event)
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
	// Component is a component used in logs.
	Component string
	// Log is a logger.
	Log logrus.FieldLogger
	// MaxRetryPeriod is the maximum retry period on failed watchers.
	MaxRetryPeriod time.Duration
	// Clock is used to control time.
	Clock clockwork.Clock
	// Client is used to create new watchers.
	Client types.Events
	// MaxStaleness is a maximum acceptable staleness for the locally maintained
	// resources, zero implies no staleness detection.
	MaxStaleness time.Duration
	// ResetC is a channel to notify of internal watcher reset (used in tests).
	ResetC chan time.Duration
	// QueueSize is an optional queue size
	QueueSize int
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ResourceWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Component == "" {
		return trace.BadParameter("missing parameter Component")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
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
		First:  utils.FullJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.Log = cfg.Log.WithField("resource-kind", watchKindsString(collector.resourceKinds()))
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
	ResourceWatcherConfig
	collector resourceCollector

	// ctx is a context controlling the lifetime of this resourceWatcher
	// instance.
	ctx    context.Context
	cancel context.CancelFunc

	// retry is used to manage backoff logic for watchers.
	retry retryutils.Retry

	// failureStartedAt records when the current sync failures were first
	// detected, zero if there are no failures present.
	failureStartedAt time.Time

	// LoopC is a channel to check whether the watch loop is running
	// (used in tests).
	LoopC chan struct{}

	// StaleC is a channel that can trigger the condition of resource staleness
	// (used in tests).
	StaleC chan struct{}
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
			p.Log.Debug("ResourceWatcher is not yet initialized.")
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
		p.Log.Debug("Starting watch.")
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
			p.Log.Warningf("Maximum staleness of %v exceeded, failure started at %v.", p.MaxStaleness, p.failureStartedAt)
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
			p.Log.Debugf("Attempting to restart watch after waiting %v.", t.Sub(startedWaiting))
			p.retry.Inc()
		case <-p.ctx.Done():
			p.Log.Debug("Closed, returning from watch loop.")
			return
		case <-p.StaleC:
			// Used for testing that the watch routine is waiting for the
			// next restart attempt. We don't want to wait for the full
			// retry period in tests so we trigger the restart immediately.
			p.Log.Debug("Stale view, continue watch loop.")
		}
		if err != nil {
			p.Log.Warningf("Restart watch on error: %v.", err)
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

	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-p.ctx.Done():
			return trace.ConnectionProblem(p.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			p.collector.processEventAndUpdateCurrent(p.ctx, event)
		case p.LoopC <- struct{}{}:
			// Used in tests to detect the watch loop is running.
		case <-p.StaleC:
			return trace.ConnectionProblem(nil, "stale view")
		}
	}
}

// ProxyWatcherConfig is a ProxyWatcher configuration.
type ProxyWatcherConfig struct {
	ResourceWatcherConfig
	// ProxyGetter is used to directly fetch the list of active proxies.
	ProxyGetter
	// ProxyDiffer is used to decide whether a put operation on an existing proxy should
	// trigger a event.
	ProxyDiffer func(old, new types.Server) bool
	// ProxiesC is a channel used to report the current proxy set. It receives
	// a fresh list at startup and subsequently a list of all known proxies
	// whenever an addition or deletion is detected.
	ProxiesC chan []types.Server
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ProxyWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.ProxyGetter == nil {
		getter, ok := cfg.Client.(ProxyGetter)
		if !ok {
			return trace.BadParameter("missing parameter ProxyGetter and Client not usable as ProxyGetter")
		}
		cfg.ProxyGetter = getter
	}
	if cfg.ProxiesC == nil {
		cfg.ProxiesC = make(chan []types.Server)
	}
	return nil
}

// NewProxyWatcher returns a new instance of ProxyWatcher.
func NewProxyWatcher(ctx context.Context, cfg ProxyWatcherConfig) (*ProxyWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &proxyCollector{
		ProxyWatcherConfig: cfg,
		initializationC:    make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ProxyWatcher{watcher, collector}, nil
}

// ProxyWatcher is built on top of resourceWatcher to monitor additions
// and deletions to the set of proxies.
type ProxyWatcher struct {
	*resourceWatcher
	*proxyCollector
}

// proxyCollector accompanies resourceWatcher when monitoring proxies.
type proxyCollector struct {
	ProxyWatcherConfig
	// current holds a map of the currently known proxies (keyed by server name,
	// RWMutex protected).
	current         map[string]types.Server
	rw              sync.RWMutex
	initializationC chan struct{}
	once            sync.Once
}

// GetCurrent returns the currently stored proxies.
func (p *proxyCollector) GetCurrent() []types.Server {
	p.rw.RLock()
	defer p.rw.RUnlock()
	return serverMapValues(p.current)
}

// resourceKinds specifies the resource kind to watch.
func (p *proxyCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindProxy}}
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (p *proxyCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	proxies, err := p.ProxyGetter.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	newCurrent := make(map[string]types.Server, len(proxies))
	for _, proxy := range proxies {
		newCurrent[proxy.GetName()] = proxy
	}
	p.rw.Lock()
	defer p.rw.Unlock()
	p.current = newCurrent
	// only emit an empty proxy list if the collector has already been initialized
	// to prevent an empty slice being sent out on creation of the watcher
	if len(proxies) > 0 || (len(proxies) == 0 && p.isInitialized()) {
		p.broadcastUpdate(ctx)
	}
	p.defineCollectorAsInitialized()
	return nil
}

func (p *proxyCollector) defineCollectorAsInitialized() {
	p.once.Do(func() {
		// mark watcher as initialized.
		close(p.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *proxyCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindProxy {
		p.Log.Warningf("Unexpected event: %v.", event)
		return
	}

	p.rw.Lock()
	defer p.rw.Unlock()

	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		// Always broadcast when a proxy is deleted.
		p.broadcastUpdate(ctx)
	case types.OpPut:
		server, ok := event.Resource.(types.Server)
		if !ok {
			p.Log.Warningf("Unexpected type %T.", event.Resource)
			return
		}
		current, exists := p.current[server.GetName()]
		p.current[server.GetName()] = server
		if !exists || (p.ProxyDiffer != nil && p.ProxyDiffer(current, server)) {
			p.broadcastUpdate(ctx)
		}
	default:
		p.Log.Warningf("Skipping unsupported event type %s.", event.Type)
	}
}

// broadcastUpdate broadcasts information about updating the proxy set.
func (p *proxyCollector) broadcastUpdate(ctx context.Context) {
	names := make([]string, 0, len(p.current))
	for k := range p.current {
		names = append(names, k)
	}
	p.Log.Debugf("List of known proxies updated: %q.", names)

	select {
	case p.ProxiesC <- serverMapValues(p.current):
	case <-ctx.Done():
	}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (p *proxyCollector) initializationChan() <-chan struct{} {
	return p.initializationC
}

func (p *proxyCollector) isInitialized() bool {
	select {
	case <-p.initializationC:
		return true
	default:
		return false
	}
}

func (p *proxyCollector) notifyStale() {}

func serverMapValues(serverMap map[string]types.Server) []types.Server {
	servers := make([]types.Server, 0, len(serverMap))
	for _, server := range serverMap {
		servers = append(servers, server)
	}
	return servers
}

// LockWatcherConfig is a LockWatcher configuration.
type LockWatcherConfig struct {
	ResourceWatcherConfig
	LockGetter
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
	// isStale indicates whether the local lock view (current) is stale.
	isStale bool
	// currentRW is a mutex protecting both current and isStale.
	currentRW sync.RWMutex
	// fanout provides support for multiple subscribers to the lock updates.
	fanout *FanoutV2
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once
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

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *lockCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindLock {
		p.Log.Warningf("Unexpected event: %v.", event)
		return
	}

	p.currentRW.Lock()
	defer p.currentRW.Unlock()
	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		p.fanout.Emit(event)
	case types.OpPut:
		lock, ok := event.Resource.(types.Lock)
		if !ok {
			p.Log.Warningf("Unexpected resource type %T.", event.Resource)
			return
		}
		if lock.IsInForce(p.Clock.Now()) {
			p.current[lock.GetName()] = lock
			p.fanout.Emit(event)
		} else {
			delete(p.current, lock.GetName())
		}
	default:
		p.Log.Warningf("Skipping unsupported event type %s.", event.Type)
	}
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

// DatabaseWatcherConfig is a DatabaseWatcher configuration.
type DatabaseWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// DatabaseGetter is responsible for fetching database resources.
	DatabaseGetter
	// DatabasesC receives up-to-date list of all database resources.
	DatabasesC chan types.Databases
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *DatabaseWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.DatabaseGetter == nil {
		getter, ok := cfg.Client.(DatabaseGetter)
		if !ok {
			return trace.BadParameter("missing parameter DatabaseGetter and Client not usable as DatabaseGetter")
		}
		cfg.DatabaseGetter = getter
	}
	if cfg.DatabasesC == nil {
		cfg.DatabasesC = make(chan types.Databases)
	}
	return nil
}

// NewDatabaseWatcher returns a new instance of DatabaseWatcher.
func NewDatabaseWatcher(ctx context.Context, cfg DatabaseWatcherConfig) (*DatabaseWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &databaseCollector{
		DatabaseWatcherConfig: cfg,
		initializationC:       make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DatabaseWatcher{watcher, collector}, nil
}

// DatabaseWatcher is built on top of resourceWatcher to monitor database resources.
type DatabaseWatcher struct {
	*resourceWatcher
	*databaseCollector
}

// databaseCollector accompanies resourceWatcher when monitoring database resources.
type databaseCollector struct {
	// DatabaseWatcherConfig is the watcher configuration.
	DatabaseWatcherConfig
	// current holds a map of the currently known database resources.
	current map[string]types.Database
	// lock protects the "current" map.
	lock sync.RWMutex
	// initializationC is used to check that the
	initializationC chan struct{}
	once            sync.Once
}

// resourceKinds specifies the resource kind to watch.
func (p *databaseCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindDatabase}}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (p *databaseCollector) initializationChan() <-chan struct{} {
	return p.initializationC
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (p *databaseCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	databases, err := p.DatabaseGetter.GetDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := make(map[string]types.Database, len(databases))
	for _, database := range databases {
		newCurrent[database.GetName()] = database
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.current = newCurrent
	p.defineCollectorAsInitialized()

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case p.DatabasesC <- databases:
	}

	return nil
}

func (p *databaseCollector) defineCollectorAsInitialized() {
	p.once.Do(func() {
		// mark watcher as initialized.
		close(p.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *databaseCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindDatabase {
		p.Log.Warnf("Unexpected event: %v.", event)
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		select {
		case <-ctx.Done():
		case p.DatabasesC <- resourcesToSlice(p.current):
		}
	case types.OpPut:
		database, ok := event.Resource.(types.Database)
		if !ok {
			p.Log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}
		p.current[database.GetName()] = database
		select {
		case <-ctx.Done():
		case p.DatabasesC <- resourcesToSlice(p.current):
		}

	default:
		p.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (*databaseCollector) notifyStale() {}

// AppWatcherConfig is an AppWatcher configuration.
type AppWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// AppGetter is responsible for fetching application resources.
	AppGetter
	// AppsC receives up-to-date list of all application resources.
	AppsC chan types.Apps
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *AppWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.AppGetter == nil {
		getter, ok := cfg.Client.(AppGetter)
		if !ok {
			return trace.BadParameter("missing parameter AppGetter and Client not usable as AppGetter")
		}
		cfg.AppGetter = getter
	}
	if cfg.AppsC == nil {
		cfg.AppsC = make(chan types.Apps)
	}
	return nil
}

// NewAppWatcher returns a new instance of AppWatcher.
func NewAppWatcher(ctx context.Context, cfg AppWatcherConfig) (*AppWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &appCollector{
		AppWatcherConfig: cfg,
		initializationC:  make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AppWatcher{watcher, collector}, nil
}

// AppWatcher is built on top of resourceWatcher to monitor application resources.
type AppWatcher struct {
	*resourceWatcher
	*appCollector
}

// appCollector accompanies resourceWatcher when monitoring application resources.
type appCollector struct {
	// AppWatcherConfig is the watcher configuration.
	AppWatcherConfig
	// current holds a map of the currently known application resources.
	current map[string]types.Application
	// lock protects the "current" map.
	lock sync.RWMutex
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once
}

// resourceKinds specifies the resource kind to watch.
func (p *appCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindApp}}
}

// isInitialized is used to check that the cache has done its initial
// sync
func (p *appCollector) initializationChan() <-chan struct{} {
	return p.initializationC
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (p *appCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	apps, err := p.AppGetter.GetApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := make(map[string]types.Application, len(apps))
	for _, app := range apps {
		newCurrent[app.GetName()] = app
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.current = newCurrent
	p.defineCollectorAsInitialized()
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case p.AppsC <- apps:
	}
	return nil
}

func (p *appCollector) defineCollectorAsInitialized() {
	p.once.Do(func() {
		// mark watcher as initialized.
		close(p.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *appCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindApp {
		p.Log.Warnf("Unexpected event: %v.", event)
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		p.AppsC <- resourcesToSlice(p.current)

		select {
		case <-ctx.Done():
		case p.AppsC <- resourcesToSlice(p.current):
		}

	case types.OpPut:
		app, ok := event.Resource.(types.Application)
		if !ok {
			p.Log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}
		p.current[app.GetName()] = app

		select {
		case <-ctx.Done():
		case p.AppsC <- resourcesToSlice(p.current):
		}
	default:
		p.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (*appCollector) notifyStale() {}

func resourcesToSlice[T any](resources map[string]T) (slice []T) {
	for _, resource := range resources {
		slice = append(slice, resource)
	}
	return slice
}

// KubeClusterWatcherConfig is an KubeClusterWatcher configuration.
type KubeClusterWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// KubernetesGetter is responsible for fetching kube_cluster resources.
	KubernetesClusterGetter
	// KubeClustersC receives up-to-date list of all kube_cluster resources.
	KubeClustersC chan types.KubeClusters
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *KubeClusterWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.KubernetesClusterGetter == nil {
		getter, ok := cfg.Client.(KubernetesClusterGetter)
		if !ok {
			return trace.BadParameter("missing parameter KubernetesGetter and Client not usable as KubernetesGetter")
		}
		cfg.KubernetesClusterGetter = getter
	}
	if cfg.KubeClustersC == nil {
		cfg.KubeClustersC = make(chan types.KubeClusters)
	}
	return nil
}

// NewKubeClusterWatcher returns a new instance of KubeClusterWatcher.
func NewKubeClusterWatcher(ctx context.Context, cfg KubeClusterWatcherConfig) (*KubeClusterWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &kubeCollector{
		KubeClusterWatcherConfig: cfg,
		initializationC:          make(chan struct{}),
	}
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KubeClusterWatcher{watcher, collector}, nil
}

// KubeClusterWatcher is built on top of resourceWatcher to monitor kube_cluster resources.
type KubeClusterWatcher struct {
	*resourceWatcher
	*kubeCollector
}

// kubeCollector accompanies resourceWatcher when monitoring kube_cluster resources.
type kubeCollector struct {
	// KubeClusterWatcherConfig is the watcher configuration.
	KubeClusterWatcherConfig
	// current holds a map of the currently known kube_cluster resources.
	current map[string]types.KubeCluster
	// lock protects the "current" map.
	lock sync.RWMutex
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once
}

// isInitialized is used to check that the cache has done its initial
// sync
func (k *kubeCollector) initializationChan() <-chan struct{} {
	return k.initializationC
}

// resourceKinds specifies the resource kind to watch.
func (k *kubeCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindKubernetesCluster}}
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (k *kubeCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	clusters, err := k.KubernetesClusterGetter.GetKubernetesClusters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := make(map[string]types.KubeCluster, len(clusters))
	for _, cluster := range clusters {
		newCurrent[cluster.GetName()] = cluster
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	k.current = newCurrent

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case k.KubeClustersC <- clusters:
	}

	k.defineCollectorAsInitialized()

	return nil
}

func (k *kubeCollector) defineCollectorAsInitialized() {
	k.once.Do(func() {
		// mark watcher as initialized.
		close(k.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (k *kubeCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindKubernetesCluster {
		k.Log.Warnf("Unexpected event: %v.", event)
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		delete(k.current, event.Resource.GetName())
		k.KubeClustersC <- resourcesToSlice(k.current)

		select {
		case <-ctx.Done():
		case k.KubeClustersC <- resourcesToSlice(k.current):
		}

	case types.OpPut:
		cluster, ok := event.Resource.(types.KubeCluster)
		if !ok {
			k.Log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}
		k.current[cluster.GetName()] = cluster

		select {
		case <-ctx.Done():
		case k.KubeClustersC <- resourcesToSlice(k.current):
		}
	default:
		k.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (*kubeCollector) notifyStale() {}

// KubeServerWatcherConfig is an KubeServerWatcher configuration.
type KubeServerWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// KubernetesServerGetter is responsible for fetching kube_server resources.
	KubernetesServerGetter
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *KubeServerWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.KubernetesServerGetter == nil {
		getter, ok := cfg.Client.(KubernetesServerGetter)
		if !ok {
			return trace.BadParameter("missing parameter KubernetesServerGetter and Client not usable as KubernetesServerGetter")
		}
		cfg.KubernetesServerGetter = getter
	}
	return nil
}

// NewKubeServerWatcher returns a new instance of KubeServerWatcher.
func NewKubeServerWatcher(ctx context.Context, cfg KubeServerWatcherConfig) (*KubeServerWatcher, error) {
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
	collector := &kubeServerCollector{
		KubeServerWatcherConfig: cfg,
		initializationC:         make(chan struct{}),
		cache:                   cache,
	}
	// start the collector as staled.
	collector.stale.Store(true)
	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KubeServerWatcher{watcher, collector}, nil
}

// KubeServerWatcher is built on top of resourceWatcher to monitor kube_server resources.
type KubeServerWatcher struct {
	*resourceWatcher
	*kubeServerCollector
}

// GetKubeServersByClusterName returns a list of kubernetes servers for the specified cluster.
func (k *KubeServerWatcher) GetKubeServersByClusterName(ctx context.Context, clusterName string) ([]types.KubeServer, error) {
	k.refreshStaleKubeServers(ctx)

	k.lock.RLock()
	defer k.lock.RUnlock()
	var servers []types.KubeServer
	for _, server := range k.current {
		if server.GetCluster().GetName() == clusterName {
			servers = append(servers, server.Copy())
		}
	}
	if len(servers) == 0 {
		return nil, trace.NotFound("no kubernetes servers found for cluster %q", clusterName)
	}

	return servers, nil
}

// GetKubernetesServers returns a list of kubernetes servers for all clusters.
func (k *KubeServerWatcher) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	k.refreshStaleKubeServers(ctx)

	k.lock.RLock()
	defer k.lock.RUnlock()
	servers := make([]types.KubeServer, 0, len(k.current))
	for _, server := range k.current {
		servers = append(servers, server.Copy())
	}
	return servers, nil
}

// kubeServerCollector accompanies resourceWatcher when monitoring kube_server resources.
type kubeServerCollector struct {
	// KubeServerWatcherConfig is the watcher configuration.
	KubeServerWatcherConfig
	// current holds a map of the currently known kube_server resources.
	current map[kubeServersKey]types.KubeServer
	// lock protects the "current" map.
	lock sync.RWMutex
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once
	// stale is used to indicate that the watcher is stale and needs to be
	// refreshed.
	stale atomic.Bool
	// cache is a helper for temporarily storing the results of GetKubernetesServers.
	// It's used to limit the amount of calls to the backend.
	cache *utils.FnCache
}

// kubeServersKey is used to uniquely identify a kube_server resource.
type kubeServersKey struct {
	hostID       string
	resourceName string
}

// isInitialized is used to check that the cache has done its initial
// sync
func (k *kubeServerCollector) initializationChan() <-chan struct{} {
	return k.initializationC
}

// resourceKinds specifies the resource kind to watch.
func (k *kubeServerCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindKubeServer}}
}

// getResourcesAndUpdateCurrent refreshes the list of current resources.
func (k *kubeServerCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newCurrent, err := k.getResources(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	k.lock.Lock()
	k.current = newCurrent
	k.lock.Unlock()

	k.stale.Store(false)

	k.defineCollectorAsInitialized()
	return nil
}

// getResourcesAndUpdateCurrent gets the list of current resources.
func (k *kubeServerCollector) getResources(ctx context.Context) (map[kubeServersKey]types.KubeServer, error) {
	servers, err := k.KubernetesServerGetter.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	current := make(map[kubeServersKey]types.KubeServer, len(servers))
	for _, server := range servers {
		key := kubeServersKey{
			hostID:       server.GetHostID(),
			resourceName: server.GetName(),
		}
		current[key] = server
	}
	return current, nil
}

func (k *kubeServerCollector) defineCollectorAsInitialized() {
	k.once.Do(func() {
		// mark watcher as initialized.
		close(k.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (k *kubeServerCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindKubeServer {
		k.Log.Warnf("Unexpected event: %v.", event)
		return
	}

	server, ok := event.Resource.(types.KubeServer)
	if !ok {
		k.Log.Warnf("Unexpected resource type %T.", event.Resource)
		return
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	switch event.Type {
	case types.OpDelete:
		key := kubeServersKey{
			// On delete events, the server description is populated with the host ID.
			hostID:       server.GetMetadata().Description,
			resourceName: server.GetName(),
		}
		delete(k.current, key)
	case types.OpPut:
		key := kubeServersKey{
			hostID:       server.GetHostID(),
			resourceName: server.GetName(),
		}
		k.current[key] = server
	default:
		k.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (k *kubeServerCollector) notifyStale() {
	k.stale.Store(true)
}

// refreshStaleKubeServers attempts to reload kube servers from the cache if
// the collector is stale. This ensures that no matter the health of
// the collector callers will be returned the most up to date node
// set as possible.
func (k *kubeServerCollector) refreshStaleKubeServers(ctx context.Context) error {
	if !k.stale.Load() {
		return nil
	}

	_, err := utils.FnCacheGet(ctx, k.cache, "kube_servers", func(ctx context.Context) (any, error) {
		current, err := k.getResources(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// There is a chance that the watcher reinitialized while
		// getting kube servers happened above. Check if we are still stale
		if k.stale.CompareAndSwap(true, false) {
			k.lock.Lock()
			k.current = current
			k.lock.Unlock()
		}

		return nil, nil
	})

	return trace.Wrap(err)
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
	CertAuthorityWatcherConfig
	fanout *FanoutV2

	// lock protects concurrent access to cas
	lock sync.RWMutex
	// cas maps ca type -> cluster -> ca
	cas map[types.CertAuthType]map[string]types.CertAuthority
	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once
	filter          types.CertAuthorityFilter
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

// processEventAndUpdateCurrent is called when a watcher event is received.
func (c *caCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindCertAuthority {
		c.Log.Warnf("Unexpected event: %v.", event)
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		caType := types.CertAuthType(event.Resource.GetSubKind())
		if !c.watchingType(caType) {
			return
		}

		delete(c.cas[caType], event.Resource.GetName())
		c.fanout.Emit(event)
	case types.OpPut:
		ca, ok := event.Resource.(types.CertAuthority)
		if !ok {
			c.Log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}

		if !c.watchingType(ca.GetType()) {
			return
		}

		authority, ok := c.cas[ca.GetType()][ca.GetName()]
		if ok && CertAuthoritiesEquivalent(authority, ca) {
			return
		}

		c.cas[ca.GetType()][ca.GetName()] = ca
		c.fanout.Emit(event)
	default:
		c.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
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
	ResourceWatcherConfig
	// NodesGetter is used to directly fetch the list of active nodes.
	NodesGetter
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *NodeWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.ResourceWatcherConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.NodesGetter == nil {
		getter, ok := cfg.Client.(NodesGetter)
		if !ok {
			return trace.BadParameter("missing parameter NodesGetter and Client not usable as NodesGetter")
		}
		cfg.NodesGetter = getter
	}
	return nil
}

// NewNodeWatcher returns a new instance of NodeWatcher.
func NewNodeWatcher(ctx context.Context, cfg NodeWatcherConfig) (*NodeWatcher, error) {
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

	collector := &nodeCollector{
		NodeWatcherConfig: cfg,
		current:           map[string]types.Server{},
		initializationC:   make(chan struct{}),
		cache:             cache,
		stale:             true,
	}

	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &NodeWatcher{resourceWatcher: watcher, nodeCollector: collector}, nil
}

// NodeWatcher is built on top of resourceWatcher to monitor additions
// and deletions to the set of nodes.
type NodeWatcher struct {
	*resourceWatcher
	*nodeCollector
}

// nodeCollector accompanies resourceWatcher when monitoring nodes.
type nodeCollector struct {
	NodeWatcherConfig

	// initializationC is used to check whether the initial sync has completed
	initializationC chan struct{}
	once            sync.Once

	cache *utils.FnCache

	rw sync.RWMutex
	// current holds a map of the currently known nodes keyed by server name
	current map[string]types.Server
	stale   bool
}

// Node is a readonly subset of the types.Server interface which
// users may filter by in GetNodes.
type Node interface {
	// ResourceWithLabels provides common resource headers
	types.ResourceWithLabels
	// GetTeleportVersion returns the teleport version the server is running on
	GetTeleportVersion() string
	// GetAddr return server address
	GetAddr() string
	// GetPublicAddrs returns all public addresses where this server can be reached.
	GetPublicAddrs() []string
	// GetHostname returns server hostname
	GetHostname() string
	// GetNamespace returns server namespace
	GetNamespace() string
	// GetCmdLabels gets command labels
	GetCmdLabels() map[string]types.CommandLabel
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() types.Rotation
	// GetUseTunnel gets if a reverse tunnel should be used to connect to this node.
	GetUseTunnel() bool
	// GetProxyIDs returns a list of proxy ids this server is connected to.
	GetProxyIDs() []string
	// IsEICE returns whether the Node is an EICE instance.
	// Must be `openssh-ec2-ice` subkind and have the AccountID and InstanceID information (AWS Metadata or Labels).
	IsEICE() bool
}

// GetNodes allows callers to retrieve a subset of nodes that match the filter provided. The
// returned servers are a copy and can be safely modified. It is intentionally hard to retrieve
// the full set of nodes to reduce the number of copies needed since the number of nodes can get
// quite large and doing so can be expensive.
func (n *nodeCollector) GetNodes(ctx context.Context, fn func(n Node) bool) []types.Server {
	// Attempt to freshen our data first.
	n.refreshStaleNodes(ctx)

	n.rw.RLock()
	defer n.rw.RUnlock()

	var matched []types.Server
	for _, server := range n.current {
		if fn(server) {
			matched = append(matched, server.DeepCopy())
		}
	}

	return matched
}

// refreshStaleNodes attempts to reload nodes from the NodeGetter if
// the collecter is stale. This ensures that no matter the health of
// the collecter callers will be returned the most up to date node
// set as possible.
func (n *nodeCollector) refreshStaleNodes(ctx context.Context) error {
	n.rw.RLock()
	if !n.stale {
		n.rw.RUnlock()
		return nil
	}
	n.rw.RUnlock()

	_, err := utils.FnCacheGet(ctx, n.cache, "nodes", func(ctx context.Context) (any, error) {
		current, err := n.getNodes(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		n.rw.Lock()
		defer n.rw.Unlock()

		// There is a chance that the watcher reinitialized while
		// getting nodes happened above. Check if we are still stale
		// now that the lock is held to ensure that the refresh is
		// still necessary.
		if !n.stale {
			return nil, nil
		}

		n.current = current
		return nil, trace.Wrap(err)
	})

	return trace.Wrap(err)
}

func (n *nodeCollector) NodeCount() int {
	n.rw.RLock()
	defer n.rw.RUnlock()
	return len(n.current)
}

// resourceKinds specifies the resource kind to watch.
func (n *nodeCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{{Kind: types.KindNode}}
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (n *nodeCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newCurrent, err := n.getNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer n.defineCollectorAsInitialized()

	if len(newCurrent) == 0 {
		return nil
	}

	n.rw.Lock()
	defer n.rw.Unlock()
	n.current = newCurrent
	n.stale = false
	return nil
}

func (n *nodeCollector) getNodes(ctx context.Context) (map[string]types.Server, error) {
	nodes, err := n.NodesGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(nodes) == 0 {
		return map[string]types.Server{}, nil
	}

	current := make(map[string]types.Server, len(nodes))
	for _, node := range nodes {
		current[node.GetName()] = node
	}

	return current, nil
}

func (n *nodeCollector) defineCollectorAsInitialized() {
	n.once.Do(func() {
		// mark watcher as initialized.
		close(n.initializationC)
	})
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (n *nodeCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindNode {
		n.Log.Warningf("Unexpected event: %v.", event)
		return
	}

	switch event.Type {
	case types.OpDelete:
		n.rw.Lock()
		delete(n.current, event.Resource.GetName())
		n.rw.Unlock()
	case types.OpPut:
		server, ok := event.Resource.(types.Server)
		if !ok {
			n.Log.Warningf("Unexpected type %T.", event.Resource)
			return
		}

		n.rw.Lock()
		n.current[server.GetName()] = server
		n.rw.Unlock()
	default:
		n.Log.Warningf("Skipping unsupported event type %s.", event.Type)
	}
}

func (n *nodeCollector) initializationChan() <-chan struct{} {
	return n.initializationC
}

func (n *nodeCollector) notifyStale() {
	n.rw.Lock()
	defer n.rw.Unlock()
	n.stale = true
}

// AccessRequestWatcherConfig is a AccessRequestWatcher configuration.
type AccessRequestWatcherConfig struct {
	// ResourceWatcherConfig is the resource watcher configuration.
	ResourceWatcherConfig
	// AccessRequestGetter is responsible for fetching access request resources.
	AccessRequestGetter
	// Filter is the filter to use to monitor access requests.
	Filter types.AccessRequestFilter
	// AccessRequestsC receives up-to-date list of all access request resources.
	AccessRequestsC chan types.AccessRequests
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
	// lock protects the "current" map.
	lock sync.RWMutex
	// initializationC is used to check that the watcher has been initialized properly.
	initializationC chan struct{}
	once            sync.Once
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

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *accessRequestCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindAccessRequest {
		p.Log.Warnf("Unexpected event: %v.", event)
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		select {
		case <-ctx.Done():
		case p.AccessRequestsC <- resourcesToSlice(p.current):
		}
	case types.OpPut:
		accessRequest, ok := event.Resource.(types.AccessRequest)
		if !ok {
			p.Log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}
		p.current[accessRequest.GetName()] = accessRequest
		select {
		case <-ctx.Done():
		case p.AccessRequestsC <- resourcesToSlice(p.current):
		}

	default:
		p.Log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (*accessRequestCollector) notifyStale() {}

// OktaAssignmentWatcherConfig is a OktaAssignmentWatcher configuration.
type OktaAssignmentWatcherConfig struct {
	// RWCfg is the resource watcher configuration.
	RWCfg ResourceWatcherConfig
	// OktaAssignments is responsible for fetching Okta assignments.
	OktaAssignments OktaAssignmentsGetter
	// PageSize is the number of Okta assignments to list at a time.
	PageSize int
	// OktaAssignmentsC receives up-to-date list of all Okta assignment resources.
	OktaAssignmentsC chan types.OktaAssignments
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
		log:             cfg.RWCfg.Log,
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
	log logrus.FieldLogger
	// OktaAssignmentWatcherConfig is the watcher configuration.
	cfg OktaAssignmentWatcherConfig
	// mu guards "current"
	mu sync.RWMutex
	// current holds a map of the currently known Okta assignment resources.
	current map[string]types.OktaAssignment
	// initializationC is used to check that the watcher has been initialized properly.
	initializationC chan struct{}
	once            sync.Once
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

// processEventAndUpdateCurrent is called when a watcher event is received.
func (c *oktaAssignmentCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindOktaAssignment {
		c.log.Warnf("Unexpected event: %v.", event)
		return
	}
	switch event.Type {
	case types.OpDelete:
		c.mu.Lock()
		delete(c.current, event.Resource.GetName())
		resources := resourcesToSlice(c.current)
		c.mu.Unlock()

		select {
		case <-ctx.Done():
		case c.cfg.OktaAssignmentsC <- resources:
		}
	case types.OpPut:
		oktaAssignment, ok := event.Resource.(types.OktaAssignment)
		if !ok {
			c.log.Warnf("Unexpected resource type %T.", event.Resource)
			return
		}
		c.mu.Lock()
		c.current[oktaAssignment.GetName()] = oktaAssignment
		resources := resourcesToSlice(c.current)
		c.mu.Unlock()

		select {
		case <-ctx.Done():
		case c.cfg.OktaAssignmentsC <- resources:
		}

	default:
		c.log.Warnf("Unsupported event type %s.", event.Type)
		return
	}
}

func (*oktaAssignmentCollector) notifyStale() {}
