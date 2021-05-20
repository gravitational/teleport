/*
Copyright 2019 Gravitational, Inc.

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

package services

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewProxyWatcherFunc creates a new instance of proxy watcher,
// used in tests
type NewProxyWatcherFunc func() (*ProxyWatcher, error)

// NewProxyWatcher returns a new instance of changeset
func NewProxyWatcher(cfg ProxyWatcherConfig) (*ProxyWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: cfg.RetryPeriod / 10,
		Max:  cfg.RetryPeriod,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	p := &ProxyWatcher{
		ctx:                ctx,
		cancel:             cancel,
		RWMutex:            &sync.RWMutex{},
		retry:              retry,
		resetC:             make(chan struct{}),
		ProxyWatcherConfig: cfg,
		FieldLogger:        cfg.Entry,
	}
	go p.watchProxies()
	return p, nil
}

// ProxyWatcher is a resource built on top of the events,
// it monitors the additions and deletions to the proxies
type ProxyWatcher struct {
	*sync.RWMutex
	log.FieldLogger
	ProxyWatcherConfig

	resetC chan struct{}

	// retry is used to manage backoff logic for watches
	retry utils.Retry

	// current is a list of the current servers
	// as reported by the watcher
	current []types.Server

	ctx    context.Context
	cancel context.CancelFunc
}

// ProxyWatcherConfig configures proxy watcher
type ProxyWatcherConfig struct {
	// Context is a parent context
	// controlling the lifecycle of the changeset
	Context context.Context
	// Component is a component used in logs
	Component string
	// RetryPeriod is a retry period on failed watchers
	RetryPeriod time.Duration
	// ReloadPeriod is a failed period on failed watches
	ReloadPeriod time.Duration
	// Client is used by changeset to monitor proxy updates
	Client ProxyWatcherClient
	// Entry is a logging entry
	Entry log.FieldLogger
	// ProxiesC is a channel that will be used
	// by the watcher to push updated list,
	// it will always receive a fresh list on the start
	// and the subsequent list of new values
	// whenever an addition or deletion to the list is detected
	ProxiesC chan []types.Server
}

// CheckAndSetDefaults checks parameters and sets default values
func (cfg *ProxyWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Context == nil {
		return trace.BadParameter("missing parameter Context")
	}
	if cfg.Component == "" {
		return trace.BadParameter("missing parameter Component")
	}
	if cfg.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if cfg.ProxiesC == nil {
		return trace.BadParameter("missing parameter ProxiesC")
	}
	if cfg.Entry == nil {
		cfg.Entry = log.StandardLogger()
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	}
	if cfg.ReloadPeriod == 0 {
		cfg.ReloadPeriod = defaults.LowResPollingPeriod
	}
	return nil
}

// GetCurrent returns a list of currently active proxies
func (p *ProxyWatcher) GetCurrent() []types.Server {
	p.RLock()
	defer p.RUnlock()
	return p.current
}

// setCurrent sets currently active proxy list
func (p *ProxyWatcher) setCurrent(proxies []types.Server) {
	p.Lock()
	defer p.Unlock()

	// Log the updated list of proxies. Useful for debugging to verify proxies
	// are being added and removed from the cluster.
	names := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		names = append(names, proxy.GetName())
	}
	p.Debugf("List of known proxies updated: %q.", names)

	p.current = proxies
}

// ProxyWatcherClient is used by changeset to fetch a list
// of proxies and subscribe to updates
type ProxyWatcherClient interface {
	types.ProxyGetter
	types.Events
}

// Reset returns a channel which notifies of internal
// watcher resets (used in tests).
func (p *ProxyWatcher) Reset() <-chan struct{} {
	return p.resetC
}

// Done returns a channel that signals
// proxy watcher closure
func (p *ProxyWatcher) Done() <-chan struct{} {
	return p.ctx.Done()
}

// Close closes proxy watcher and cancels all the functions
func (p *ProxyWatcher) Close() error {
	p.cancel()
	return nil
}

// watchProxies watches new proxies added and removed to the cluster
// and when this happens, notifies all connected agents
// about the proxy set change via discovery requests
func (p *ProxyWatcher) watchProxies() {
	for {
		// Reload period is here to protect against
		// unknown cache going out of sync problems
		// that we did not predict.
		err := p.watch()
		if err != nil {
			p.Warningf("Re-init the watcher on error: %v.", trace.Unwrap(err))
		}
		p.Debugf("Reloading %v.", p.retry)
		select {
		case p.resetC <- struct{}{}:
		default:
		}
		select {
		case <-p.retry.After():
			p.retry.Inc()
		case <-p.ctx.Done():
			p.Debugf("Closed, returning from update loop.")
			return
		}
	}
}

// watch sets up the watch on proxies
func (p *ProxyWatcher) watch() error {
	watcher, err := p.Client.NewWatcher(p.Context, types.Watch{
		Name: p.Component,
		Kinds: []types.WatchKind{
			{
				Kind: types.KindProxy,
			},
		},
		MetricComponent: p.Component,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	reloadC := time.After(p.ReloadPeriod)
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
	case <-reloadC:
		p.Debugf("Triggering scheduled reload.")
		return nil
	case <-p.ctx.Done():
		return trace.ConnectionProblem(p.Context.Err(), "context is closing")
	case event := <-watcher.Events():
		if event.Type != backend.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}
	proxies, err := p.Client.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(proxies) == 0 {
		// at least 1 proxy aught to exist; yield back to the outer
		// retry loop and try again.
		return trace.NotFound("empty proxy list")
	}
	proxySet := make(map[string]types.Server, len(proxies))
	for i := range proxies {
		proxySet[proxies[i].GetName()] = proxies[i]
	}
	p.retry.Reset()
	p.setCurrent(proxies)
	select {
	case p.ProxiesC <- proxies:
	case <-p.ctx.Done():
		return trace.ConnectionProblem(p.Context.Err(), "context is closing")
	}
	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed")
		case <-reloadC:
			p.Debugf("Triggering scheduled reload.")
			return nil
		case <-p.ctx.Done():
			return trace.ConnectionProblem(p.Context.Err(), "context is closing")
		case event := <-watcher.Events():
			updated := p.processEvent(event, proxySet)
			if updated {
				proxies = setToList(proxySet)
				p.setCurrent(proxies)
				select {
				case p.ProxiesC <- proxies:
				case <-p.Context.Done():
					return trace.ConnectionProblem(p.Context.Err(), "context is closing")
				}
			}
		}
	}
}

// processEvent updates proxy map and returns true if the proxies list have been modified -
// the proxy has been either added or deleted
func (p *ProxyWatcher) processEvent(event types.Event, proxies map[string]types.Server) bool {
	if event.Resource.GetKind() != types.KindProxy {
		p.Warningf("Unexpected event: %v.")
		return false
	}
	switch event.Type {
	case backend.OpDelete:
		delete(proxies, event.Resource.GetName())
		// Always return true if the proxy has been deleted to trigger
		// broadcast cleanup.
		return true
	case backend.OpPut:
		_, existed := proxies[event.Resource.GetName()]
		resource, ok := event.Resource.(types.Server)
		if !ok {
			p.Warningf("unexpected type %T", event.Resource)
			return false
		}
		proxies[event.Resource.GetName()] = resource
		return !existed
	default:
		p.Warningf("Skipping unsupported event type %v.", event.Type)
		return false
	}
}

func setToList(in map[string]types.Server) []types.Server {
	out := make([]types.Server, 0, len(in))
	for key := range in {
		out = append(out, in[key])
	}
	return out
}
