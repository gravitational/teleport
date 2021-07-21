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
	"github.com/sirupsen/logrus"
)

// resourceCollector is a generic interface for maintaining an up-to-date view
// of a resource set being monitored. Used in conjunction with resourceWatcher.
type resourceCollector interface {
	// WatchKinds specifies the resource kinds to watch.
	WatchKinds() []types.WatchKind

	// getResourcesAndUpdateCurrent is called when the resources should be
	// (re-)fetched directly.
	getResourcesAndUpdateCurrent(context.Context) error
	// processEventAndUpdateCurrent is called when a watcher event is received.
	processEventAndUpdateCurrent(context.Context, types.Event) error
}

// ResourceWatcherConfig configures resource watcher.
type ResourceWatcherConfig struct {
	// ParentContext is a parent context.
	ParentContext context.Context
	// Component is a component used in logs.
	Component string
	// Log is a logger.
	Log logrus.FieldLogger
	// RetryPeriod is a retry period on failed watchers.
	RetryPeriod time.Duration
	// RefetchPeriod is a period after which to explicitly refetch the resources.
	// It is to protect against unexpected cache syncing issues.
	RefetchPeriod time.Duration
	// Client is used to create new watchers.
	Client types.Events
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ResourceWatcherConfig) CheckAndSetDefaults() error {
	if cfg.ParentContext == nil {
		return trace.BadParameter("missing parameter ParentContext")
	}
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

	ctx, cancel := context.WithCancel(cfg.ParentContext)
	p := &resourceWatcher{
		resourceCollector:     collector,
		ResourceWatcherConfig: cfg,
		ctx:                   ctx,
		cancel:                cancel,
		retry:                 retry,
		ResetC:                make(chan struct{}),
	}
	go p.watchResources()
	return p, nil
}

// resourceWatcher monitors additions, updates and deletions
// to a set of resources.
type resourceWatcher struct {
	resourceCollector
	ResourceWatcherConfig

	ctx    context.Context
	cancel context.CancelFunc

	// retry is used to manage backoff logic for watchers.
	retry utils.Retry

	// ResetC is a channel to notify of internal watcher reset (used in tests).
	ResetC chan struct{}
}

// Done returns a channel that signals resource watcher closure.
func (p *resourceWatcher) Done() <-chan struct{} {
	return p.ctx.Done()
}

// Close closes resource watcher and cancels all the functions.
func (p *resourceWatcher) Close() error {
	p.cancel()
	return nil
}

// watchResources runs a watch loop.
func (p *resourceWatcher) watchResources() {
	for {
		p.Log.WithField("retry", p.retry).Debug("Starting watch.")
		err := p.watch()
		if err != nil {
			p.Log.WithError(err).Warning("Restart watch on error.")
		}
		select {
		case p.ResetC <- struct{}{}:
		default:
		}
		select {
		case <-p.retry.After():
			p.retry.Inc()
		case <-p.ctx.Done():
			p.Log.Debug("Closed, returning from watch loop.")
			return
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

	if err := p.getResourcesAndUpdateCurrent(p.ctx); err != nil {
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
			if err := p.processEventAndUpdateCurrent(p.ctx, event); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// ProxyWatcherConfig is a ProxyWatcher configuration.
type ProxyWatcherConfig struct {
	ResourceWatcherConfig
	// ProxyGetter is used to directly fetch the list of active proxies.
	ProxyGetter
	// ProxiesC is a channel used by the watcher to push updates to the proxy
	// set.  It receives a fresh list at startup and subsequently a list of all
	// known proxies whenever an addition or deletion is detected.
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
func NewProxyWatcher(cfg ProxyWatcherConfig) (*ProxyWatcher, error) {
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

// GetCurrent returns the currently stored proxy set.
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
func (p *proxyCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	proxies, err := p.GetProxies()
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
	return trace.Wrap(p.broadcastUpdate(ctx))
}

// processEventAndUpdateCurrent is called when a watcher event is received.
func (p *proxyCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) error {
	if event.Resource == nil || event.Resource.GetKind() != types.KindProxy {
		p.Log.Warningf("Unexpected event: %v.", event)
		return nil
	}

	p.rw.Lock()
	defer p.rw.Unlock()

	switch event.Type {
	case types.OpDelete:
		delete(p.current, event.Resource.GetName())
		// Always broadcast when a proxy is deleted.
		return trace.Wrap(p.broadcastUpdate(ctx))
	case types.OpPut:
		server, ok := event.Resource.(types.Server)
		if !ok {
			p.Log.Warningf("Unexpected type %T.", event.Resource)
			return nil
		}
		_, known := p.current[server.GetName()]
		p.current[server.GetName()] = server
		// Broadcast only creation of new proxies (not known before).
		if !known {
			return trace.Wrap(p.broadcastUpdate(ctx))
		}
		return nil
	default:
		p.Log.Warningf("Skipping unsupported event type %v.", event.Type)
		return nil
	}
}

// broadcastUpdate broadcasts information about updating the proxy set.
func (p *proxyCollector) broadcastUpdate(ctx context.Context) error {
	names := make([]string, 0, len(p.current))
	for k := range p.current {
		names = append(names, k)
	}
	p.Log.Debugf("List of known proxies updated: %q.", names)

	select {
	case p.ProxiesC <- serverMapValues(p.current):
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context is closing")
	}
	return nil
}

func serverMapValues(m map[string]types.Server) []types.Server {
	servers := make([]types.Server, 0, len(m))
	for _, s := range m {
		servers = append(servers, s)
	}
	return servers
}
