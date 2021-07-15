/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or collectoried.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// resourceCollector is a generic interface for maintaining an up-to-date view
// of a resource set being monitored. Used in conjunction with resourceWatcher.
type resourceCollector interface {
	// WatchKinds specifies the resource kinds to watch.
	WatchKinds() []types.WatchKind
	// getResourcesAndUpdateCurrent is called when the resources should be
	// (re-)fetched directly.
	getResourcesAndUpdateCurrent() error
	// processEventAndUpdateCurrent is called when a watcher event is received.
	processEventAndUpdateCurrent(types.Event)
}

// ResourceWatcherConfig configures resource watcher.
type ResourceWatcherConfig struct {
	ctx    context.Context
	cancel context.CancelFunc
	// Component is a component used in logs.
	Component string
	// Log is a logger.
	Log logrus.FieldLogger
	// RetryPeriod is a retry period on failed watchers.
	RetryPeriod time.Duration
	// RefetchPeriod is a period after which to explicitly refetch the resources.
	// It is to protect against unexpected cache syncing issues.
	RefetchPeriod time.Duration
	// Clock is used to control time.
	Clock clockwork.Clock
	// Client is used to create new watchers.
	Client types.Events
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ResourceWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Component == "" {
		return trace.BadParameter("missing parameter Component")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	}
	if cfg.RefetchPeriod == 0 {
		cfg.RefetchPeriod = defaults.LowResPollingPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// newResourceWatcher returns a new instance of resourceWatcher.
// It is the caller's responsibility to verify the inputs' validity
// incl. cfg.CheckAndSetDefaults.
func newResourceWatcher(collector resourceCollector, cfg ResourceWatcherConfig) (*resourceWatcher, error) {
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: cfg.RetryPeriod / 10,
		Max:  cfg.RetryPeriod,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Log = cfg.Log.WithField("watch-kinds", collector.WatchKinds())
	p := &resourceWatcher{
		resourceCollector:     collector,
		ResourceWatcherConfig: cfg,
		retry:                 retry,
		ResetC:                make(chan struct{}),
	}
	return p, nil
}

// resourceWatcher monitors additions, updates and deletions
// to a set of resources.
type resourceWatcher struct {
	resourceCollector
	ResourceWatcherConfig

	// retry is used to manage backoff logic for watchers.
	retry utils.Retry

	// ResetC is a channel to notify of internal watcher reset (used in tests).
	ResetC chan struct{}
}

// Done returns a channel that signals resource watcher closure.
func (p *resourceWatcher) Done() <-chan struct{} {
	return p.ResourceWatcherConfig.ctx.Done()
}

// Close closes the resource watcher and cancels all the functions.
func (p *resourceWatcher) Close() {
	p.ResourceWatcherConfig.cancel()
}

// RunWatchLoop runs a watch loop.
func (p *resourceWatcher) RunWatchLoop() {
	for {
		p.Log.WithField("retry", p.retry).Debug("Starting watch.")
		err := p.watch()
		select {
		case <-p.retry.After():
			p.retry.Inc()
		case <-p.ctx.Done():
			p.Log.Debug("Closed, returning from watch loop.")
			return
		}
		select {
		case p.ResetC <- struct{}{}:
		default:
		}
		if err != nil {
			p.Log.WithError(err).Warning("Restart watch on error.")
		}
	}
}

// watch monitors new resource updates, maintains a local view and broadcasts
// notifications to connected agents.
func (p *resourceWatcher) watch() error {
	watcher, err := p.Client.NewWatcher(p.ctx, types.Watch{
		Name:            p.Component,
		MetricComponent: p.Component,
		Kinds:           p.WatchKinds(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	refetchC := time.After(p.RefetchPeriod)

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
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed")
	case <-refetchC:
		p.Log.Debug("Triggering scheduled refetch.")
		return nil
	case <-p.ctx.Done():
		return trace.ConnectionProblem(p.ctx.Err(), "context is closing")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	if err := p.getResourcesAndUpdateCurrent(); err != nil {
		return trace.Wrap(err)
	}
	p.retry.Reset()

	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed")
		case <-refetchC:
			p.Log.Debug("Triggering scheduled refetch.")
			return nil
		case <-p.ctx.Done():
			return trace.ConnectionProblem(p.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			p.processEventAndUpdateCurrent(event)
		}
	}
}

// ProxyWatcherConfig is a ProxyWatcher configuration.
type ProxyWatcherConfig struct {
	ResourceWatcherConfig
	// ProxyGetter is used to directly fetch the list of active proxies.
	ProxyGetter
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
	cfg.ctx, cfg.cancel = context.WithCancel(ctx)
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &proxyCollector{
		ProxyWatcherConfig: cfg,
	}
	watcher, err := newResourceWatcher(collector, cfg.ResourceWatcherConfig)
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
	current map[string]types.Server
	rw      sync.RWMutex
}

// GetCurrent returns the currently stored proxies.
func (p *proxyCollector) GetCurrent() []types.Server {
	p.rw.RLock()
	defer p.rw.RUnlock()
	return serverMapValues(p.current)
}

// WatchKinds specifies the resource kinds to watch.
func (p *proxyCollector) WatchKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindProxy,
		},
	}
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (p *proxyCollector) getResourcesAndUpdateCurrent() error {
	proxies, err := p.ProxyGetter.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(proxies) == 0 {
		// At least one proxy ought to exist.
		return trace.NotFound("empty proxy list")
	}
	newCurrent := make(map[string]types.Server, len(proxies))
	for _, proxy := range proxies {
		newCurrent[proxy.GetName()] = proxy
	}
	p.rw.Lock()
	defer p.rw.Unlock()
	p.current = newCurrent
	p.broadcastUpdate()
	return nil
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *proxyCollector) processEventAndUpdateCurrent(event types.Event) {
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
		p.broadcastUpdate()
	case types.OpPut:
		server, ok := event.Resource.(types.Server)
		if !ok {
			p.Log.Warningf("Unexpected type %T.", event.Resource)
			return
		}
		_, known := p.current[server.GetName()]
		p.current[server.GetName()] = server
		// Broadcast only creation of new proxies (not known before).
		if !known {
			p.broadcastUpdate()
		}
	default:
		p.Log.Warningf("Skipping unsupported event type %s.", event.Type)
	}
}

// broadcastUpdate broadcasts information about updating the proxy set.
func (p *proxyCollector) broadcastUpdate() {
	names := make([]string, 0, len(p.current))
	for k := range p.current {
		names = append(names, k)
	}
	p.Log.Debugf("List of known proxies updated: %q.", names)

	select {
	case p.ProxiesC <- serverMapValues(p.current):
	case <-p.ctx.Done():
	}
}

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
	cfg.ctx, cfg.cancel = context.WithCancel(ctx)
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	collector := &lockCollector{
		LockWatcherConfig: cfg,
		fanout:            NewFanout(),
	}
	watcher, err := newResourceWatcher(collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	collector.fanout.SetInit()
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
	// currentRW is a mutex protecting current.
	currentRW sync.RWMutex
	// fanout provides support for multiple subscribers to the lock updates.
	fanout *Fanout
}

// Subscribe is used to subscribe to the lock updates.
func (p *lockCollector) Subscribe(ctx context.Context, targets []types.LockTarget) (types.Watcher, error) {
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
			return nil, trace.BadParameter("unexpected event type %s", event.Type)
		}
	case <-time.After(defaults.LowResPollingPeriod):
		return nil, trace.LimitExceeded("lock watcher subscription failed to initialize")
	case <-sub.Done():
		return nil, trace.Wrap(sub.Error())
	}
	return sub, nil
}

// GetLockInForce returns a matching lock in force, nil if not found.
func (p *lockCollector) GetLockInForce(targets []types.LockTarget) types.Lock {
	p.currentRW.RLock()
	defer p.currentRW.RUnlock()

	for _, lock := range p.current {
		if !lock.IsInForce(p.Clock) {
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

// WatchKinds specifies the resource kinds to watch.
func (p *lockCollector) WatchKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindLock,
		},
	}
}

// getResourcesAndUpdateCurrent is called when the resources should be
// (re-)fetched directly.
func (p *lockCollector) getResourcesAndUpdateCurrent() error {
	locks, err := p.LockGetter.GetLocks(p.ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newCurrent := map[string]types.Lock{}
	for _, lock := range locks {
		if !lock.IsInForce(p.Clock) {
			continue
		}
		newCurrent[lock.GetName()] = lock
	}

	p.currentRW.Lock()
	defer p.currentRW.Unlock()
	p.current = newCurrent
	for _, lock := range p.current {
		p.fanout.Emit(types.Event{Type: types.OpPut, Resource: lock})
	}
	return nil
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *lockCollector) processEventAndUpdateCurrent(event types.Event) {
	if event.Resource == nil || event.Resource.GetKind() != types.KindLock {
		p.Log.Warningf("Unexpected event: %v.", event)
		return
	}

	p.currentRW.Lock()
	defer p.currentRW.Unlock()

	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		// Lock deletion need not be broadcast further.
	case types.OpPut:
		lock, ok := event.Resource.(types.Lock)
		if !ok {
			p.Log.Warningf("Unexpected resource type %T.", event.Resource)
			return
		}
		if lock.IsInForce(p.Clock) {
			p.current[lock.GetName()] = lock
			p.fanout.Emit(event)
		} else {
			delete(p.current, lock.GetName())
		}
	default:
		p.Log.Warningf("Skipping unsupported event type %s.", event.Type)
	}
}

func lockTargetsToWatchKinds(targets []types.LockTarget) ([]types.WatchKind, error) {
	watchKinds := make([]types.WatchKind, 0, len(targets))
	for _, target := range targets {
		filter, err := target.IntoMap()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		watchKinds = append(watchKinds, types.WatchKind{
			Kind:   types.KindLock,
			Filter: filter,
		})
	}
	return watchKinds, nil
}
