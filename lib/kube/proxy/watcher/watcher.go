// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package watcher

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/readonly"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const eventBufferMaxSize = 2048
const eventBufferInitialSize = 32

// KubernetesServerGetter defines interface for fetching kubernetes server resources.
type KubernetesServerGetter interface {
	// GetKubernetesServers returns all kubernetes server resources.
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)
}

type KubeServerWatcherGetter interface {
	KubernetesServerGetter
	types.Events
}

// ProxyKubeServerWatcherConfig is the configuration struct for [ProxyKubeServerWatcher].
type ProxyKubeServerWatcherConfig struct {
	Logger *slog.Logger

	// Component is a component used in logs.
	Component string

	// MaxRetryPeriod is the maximum retry duration for watcher backoff.
	MaxRetryPeriod time.Duration

	// AccessPoint is the primary source of kube servers.
	AccessPoint KubeServerWatcherGetter

	// PrimaryTimeout is the duration after which the primary access point is considered unhealthy.
	PrimaryTimeout time.Duration

	// FallbackGetter is used to fetch kube servers when the primary access point is considered unhealthy.
	FallbackGetter KubernetesServerGetter

	// FallbackInterval is the minimum duration between attempts to fetch kube servers from the fallback getter when the primary access point is unhealthy.
	FallbackInterval time.Duration
}

// CheckAndSetDefaults checks the configuration and sets default values.
func (cfg *ProxyKubeServerWatcherConfig) CheckAndSetDefaults() error {
	const defaultPrimaryTimeout = time.Minute
	const defaultFallbackInterval = time.Minute

	// Check component defensively to prevent creating this watcher for anything but the proxy.
	switch cfg.Component {
	case teleport.ComponentProxy,
		teleport.ComponentKube,
		teleport.Component(teleport.ComponentProxy, teleport.ComponentProxyKube):
	default:
		return trace.BadParameter("unsupported component %q", cfg.Component)
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if cfg.FallbackGetter == nil {
		return trace.BadParameter("missing FallbackGetter")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default().With(teleport.ComponentKey, teleport.Component(cfg.Component, "kube_server_watcher"))
	}
	if cfg.MaxRetryPeriod <= 0 {
		cfg.MaxRetryPeriod = defaults.MaxWatcherBackoff
	}
	if cfg.PrimaryTimeout <= 0 {
		cfg.PrimaryTimeout = defaultPrimaryTimeout
	}
	if cfg.FallbackInterval <= 0 {
		cfg.FallbackInterval = defaultFallbackInterval
	}

	return nil
}

// ProxyKubeServerWatcher provides a watcher for kube servers that can tolerate local cache staleness.
// This is used by the proxy to watch for kube servers and should never be used in Agents, faulty cache
// on agents would lead to an excessive amount of fallback calls to the auth server and cause performance issues.
// This watcher should only be used when the cache staleness is in the critical connection path to keep access
// functional in a degraded state. This is currently strongly typed to kube servers but could be made more generic if needed in the future.
type ProxyKubeServerWatcher struct {
	ProxyKubeServerWatcherConfig

	// ctx is a context controlling the lifetime of this resourceWatcher
	// instance.
	ctx context.Context

	// retry is used to manage backoff logic for watchers.
	retry retryutils.Retry

	// cancel is used to cancel the context controlling the lifetime of this instance.
	cancel context.CancelFunc

	// hot is used to indicate that the watcher is hot and can serve kube servers fed from event watcher.
	hot atomic.Bool

	// initC is closed when the watcher is initialized successfully for the first time.
	initC    chan struct{}
	initOnce sync.Once

	// lastFullFetchAttempt is the time when we last attempted to fetch kube servers from the fallback getter.
	lastFullFetchAttempt time.Time

	// rw protects below fields
	rw sync.RWMutex

	// current holds a map of the currently known servers.
	current map[serverKey]types.KubeServer

	// primaryFailureAt is the time when the primary access point was first observed to be failing.
	primaryFailureAt time.Time
}

// serverKey maps [types.KubeServer] into a local cache.
type serverKey struct {
	Name, HostID string
}

// NewProxyKubeServerWatcher creates a new instance of [ProxyKubeServerWatcher]
func NewProxyKubeServerWatcher(ctx context.Context, cfg ProxyKubeServerWatcherConfig) (*ProxyKubeServerWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "checking and setting defaults")
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating retry")
	}

	cfg.Logger = cfg.Logger.With("resource_kinds", types.KindKubeServer)
	ctx, cancel := context.WithCancel(ctx)

	w := &ProxyKubeServerWatcher{
		ProxyKubeServerWatcherConfig: cfg,

		cancel: cancel,
		ctx:    ctx,
		retry:  retry,
		initC:  make(chan struct{}),
	}

	go w.runWatchLoop()

	return w, nil
}

// armTimeout creates the timeout timer for marking the cache as stale. If a previous
// failure has already been marked and the cache is still hot the timeout is adjusted
// to not exceed [ProxyKubeServerWatcherConfig.PrimaryTimeout].
// Otherwise defaults to [ProxyKubeServerWatcherConfig.PrimaryTimeout].
func (w *ProxyKubeServerWatcher) armTimeout() *time.Timer {
	w.rw.RLock()
	failureAt := w.primaryFailureAt
	w.rw.RUnlock()

	timeout := w.PrimaryTimeout

	if !failureAt.IsZero() && w.hot.Load() {
		elapsed := time.Since(failureAt)
		if elapsed >= w.PrimaryTimeout {
			timeout = 0
		} else {
			timeout = w.PrimaryTimeout - elapsed
		}
	}

	return time.NewTimer(timeout)

}

// fillEventBuf fills the given buffer with events from the watcher channel until the buffer is reaches [eventBufferMaxSize] or the channel is closed.
func fillEventBuf(ctx context.Context, buf []types.Event, w types.Watcher) (out []types.Event) {
	out = buf // does not reset, caller is responsible for clearing the buffer after processing

	for len(out) < eventBufferMaxSize {
		select {
		case <-w.Done():
			return
		case <-ctx.Done():
			return
		case event, ok := <-w.Events():
			if !ok {
				return
			}
			out = append(out, event)
		default:
			return
		}
	}
	return
}

// watch spawns a watcher on [types.KindKubeServer] and if successful, initlizes the local cache nad fetches events.
func (w *ProxyKubeServerWatcher) watch() error {
	watcher, err := w.AccessPoint.NewWatcher(w.ctx, types.Watch{
		Name:            w.Component,
		MetricComponent: w.Component,
		Kinds:           []types.WatchKind{{Kind: types.KindKubeServer}},
	})
	if err != nil {
		return trace.Wrap(err, "creating a watcher")
	}
	defer watcher.Close()

	timer := w.armTimeout()
	defer timer.Stop()

	var initReceived bool
	for !initReceived {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-w.ctx.Done():
			return trace.ConnectionProblem(w.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			if event.Type != types.OpInit {
				return trace.BadParameter("expected init event, got %v instead", event.Type)
			}
			initReceived = true
		case <-timer.C:
			// Do not return timeout here but mark the failure and continue waiting.
			// It's possible the watcher will recover eventually. If using the cache,
			// on error the cache will close the watcher and we handle that separately.
			w.hot.Store(false)
			w.Logger.WarnContext(w.ctx, "slow watcher init")
		}
	}

	if err := w.fetchInitialState(w.ctx); err != nil {
		return trace.Wrap(err, "warming up cache")
	}

	// At this point watcher is successfully initialized and the cache is warmed up, we can reset the retry backoff and start processing events.
	w.retry.Reset()
	eventBuf := make([]types.Event, 0, eventBufferInitialSize)

	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-w.ctx.Done():
			return trace.ConnectionProblem(w.ctx.Err(), "context is closing")
		case event, ok := <-watcher.Events():
			// Safety check
			if !ok {
				return trace.ConnectionProblem(nil, "watcher events channel is closed (this is a bug)")
			}
			eventBuf = append(eventBuf[:0], event)
			eventBuf = fillEventBuf(w.ctx, eventBuf, watcher)
			w.processEvents(w.ctx, eventBuf)
			clear(eventBuf)
		}
	}
}

// kubeServerKey returns the key used to store the given kube server in the watcher's cache.
func kubeServerKey(resource types.KubeServer) serverKey {
	return serverKey{
		Name:   resource.GetName(),
		HostID: resource.GetHostID(),
	}
}

// kubeServerDeleteKey returns the key used to delete the given kube server from the watcher's cache.
func kubeServerDeleteKey(resource types.Resource) serverKey {
	return serverKey{
		Name: resource.GetName(),
		// On delete events, the server description is populated with the host ID.
		HostID: resource.GetMetadata().Description,
	}
}

// getAllKubeServers fetches all kube servers from the given getter and returns them as a map keyed by the watcher's cache keys.
func (w *ProxyKubeServerWatcher) getAllKubeServers(ctx context.Context, getter KubernetesServerGetter) (map[serverKey]types.KubeServer, error) {
	resources, err := getter.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	current := make(map[serverKey]types.KubeServer, len(resources))
	for _, resource := range resources {
		current[kubeServerKey(resource)] = resource
	}
	return current, nil
}

// fetchInitialState fetches all kube servers from the primary access point and marks the watcher as initialized.
func (w *ProxyKubeServerWatcher) fetchInitialState(ctx context.Context) error {
	newCurrent, err := w.getAllKubeServers(ctx, w.AccessPoint)
	if err != nil {
		return trace.Wrap(err, "fetching from primary")
	}

	w.rw.Lock()
	defer w.rw.Unlock()
	w.current = newCurrent
	w.hot.Store(true)
	w.primaryFailureAt = time.Time{}
	w.initOnce.Do(func() {
		close(w.initC)
	})
	return nil
}

// maybeFetchFromUpstream attempts to fetch the kube servers from the auth server if the cache is cold and
// the next cold fetch time has been reached. This is used to throttle calls to the auth server when the cache is cold.
func (w *ProxyKubeServerWatcher) maybeFetchFromUpstream(ctx context.Context) error {
	if w.hot.Load() {
		// fast path watcher is hot, no need to fetch from upstream
		return nil
	}

	now := time.Now()

	w.rw.Lock()
	if now.Before(w.lastFullFetchAttempt.Add(w.FallbackInterval)) {
		w.rw.Unlock()
		return nil
	}
	w.lastFullFetchAttempt = now
	w.rw.Unlock()

	newCurrent, err := w.getAllKubeServers(ctx, w.FallbackGetter)
	if err != nil {
		return trace.Wrap(err, "fetching from fallback")
	}

	w.rw.Lock()
	defer w.rw.Unlock()

	if w.hot.Load() {
		// Double check cache is not hot while we waited on the lock and/or fetch.
		return nil
	}

	w.current = newCurrent
	return err
}

// handleWatchError handles errors from the watch loop.
func (w *ProxyKubeServerWatcher) handleWatchError(err error) {
	now := time.Now()

	w.rw.Lock()
	defer w.rw.Unlock()
	if w.primaryFailureAt.IsZero() {
		w.primaryFailureAt = now
	} else if w.hot.Load() && time.Since(w.primaryFailureAt) >= w.PrimaryTimeout {
		w.Logger.WarnContext(w.ctx, "Primary access point is unhealthy, falling back to auth server for kube servers.", "error", err, "failure_at", w.primaryFailureAt.String())
		w.hot.Store(false)
	}
}

// runWatchLoop is the main event loop of the watcher.
func (w *ProxyKubeServerWatcher) runWatchLoop() {
	for {
		err := w.watch()
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		if err != nil {
			w.handleWatchError(err)
		}

		startedWaiting := time.Now()
		select {
		case t := <-w.retry.After():
			w.Logger.DebugContext(w.ctx, "Attempting to restart watch after waiting", "waited", t.Sub(startedWaiting).String())
			w.retry.Inc()
		case <-w.ctx.Done():
			w.Logger.DebugContext(w.ctx, "Closed, returning from watch loop.")
			return
		}
		if err != nil {
			w.Logger.WarnContext(w.ctx, "Restart watch on error", "error", err)
		}
	}
}

// processEvents takes events from the watcher channel and applies them to the local cache.
func (w *ProxyKubeServerWatcher) processEvents(ctx context.Context, events []types.Event) {
	w.rw.Lock()
	defer w.rw.Unlock()

	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindKubeServer {
			w.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}

		switch event.Type {
		case types.OpDelete:
			delete(w.current, kubeServerDeleteKey(event.Resource))
		case types.OpPut:
			srv, err := types.ConvertResource[types.KubeServer](event.Resource)
			if err != nil {
				w.Logger.WarnContext(ctx, "Failed to convert event resource",
					"resource", event.Resource.GetKind(),
					"error", err,
				)
				continue
			}

			w.current[kubeServerKey(srv)] = srv
		default:
			w.Logger.WarnContext(ctx, "Skipping unsupported event type", "event_type", event.Type)
		}
	}
}

// Done returns a channel that signals resource watcher closure.
func (w *ProxyKubeServerWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// Close closes the resource watcher and cancels all the functions.
func (w *ProxyKubeServerWatcher) Close() {
	w.cancel()
}

// IsInitialized is a non-blocking way to check if the watcher is initialized.
func (w *ProxyKubeServerWatcher) IsInitialized() bool {
	select {
	case <-w.initC:
		return true
	default:
		return false
	}
}

// WaitInitialization blocks until watcher is initialized.
func (w *ProxyKubeServerWatcher) WaitInitialization() error {
	const initTickerPeriod = 5 * time.Second

	ticker := time.NewTicker(initTickerPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-w.initC:
			return nil
		case <-ticker.C:
			w.Logger.DebugContext(w.ctx, "ProxyKubeServerWatcher is not yet initialized.")
		case <-w.ctx.Done():
			return trace.ConnectionProblem(nil, "Failed to initialize, context closing")
		}
	}
}

// CurrentResourcesWithFilter returns a copy of the resources known to the watcher
// that match the provided filter.
func (w *ProxyKubeServerWatcher) CurrentResourcesWithFilter(ctx context.Context, filter func(readonly.KubeServer) bool) ([]types.KubeServer, error) {
	if err := w.maybeFetchFromUpstream(ctx); err != nil {
		// This is an indication things are in a very bad state, it means the primary acccess point is not responsive
		// and the fallback getter is failing. Log this as a warning since it means the data returned by this function is likely stale.
		// Keep going in the attempt to keep the proxy routing to kube servers functional.
		w.Logger.WarnContext(ctx, "Unhealthy watcher failed to fetch from upstream", "error", err)
	}

	w.rw.RLock()
	defer w.rw.RUnlock()

	var out []types.KubeServer
	for _, resource := range w.current {
		if filter(readonly.KubeServer(resource)) {
			out = append(out, types.KubeServer.Copy(resource))
		}
	}

	return out, nil
}
