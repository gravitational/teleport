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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/readonly"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const eventBufferSize = 128

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

	// initC is closed when the watcher is initialized successfully for the first time.
	initC    chan struct{}
	initOnce sync.Once

	// rw protects below fields
	rw sync.RWMutex

	// current holds a map of the currently known servers.
	current map[serverKey]types.KubeServer

	// nextFallbackFetch is the next point in time when the watcher should attempt to fetch from the fallback getter
	// if the primary access point is observed to be failing.
	nextFallbackFetch time.Time

	// singleFlighter is used to suppress multiple simultaneous fetches from the fallback
	// getter when the primary access point is observed to be failing.
	singleFlighter singleflight.Group
}

// serverKey maps [types.KubeServer] into a local cache.
type serverKey struct {
	Name, HostID string
}

// NewProxyKubeServerWatcher creates a new instance of [ProxyKubeServerWatcher]
// ctx is the lifetime of the watcher.
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

func (w *ProxyKubeServerWatcher) isHealthyLocked() bool {
	return w.nextFallbackFetch.IsZero()
}

func (w *ProxyKubeServerWatcher) shouldFetchFromFallback(now time.Time) bool {
	w.rw.Lock()
	defer w.rw.Unlock()

	if w.isHealthyLocked() {
		return false
	}

	if now.Before(w.nextFallbackFetch) {
		return false
	}

	// Note this includes time to acquire the lock but the interval should be much greater than that.
	w.nextFallbackFetch = now.Add(retryutils.SeventhJitter(w.FallbackInterval))
	return true
}

// markUnhealthyOnInitTimeout marks the watcher as unhealthy if the initial watcher creation and init process takes longer than the configured primary timeout.
func (w *ProxyKubeServerWatcher) markUnhealthyOnInitTimeout() {
	now := time.Now()

	w.rw.Lock()
	defer w.rw.Unlock()

	if w.nextFallbackFetch.IsZero() {
		// Watcher is considered unhealthy after the initial timeout, given we know the timeout has elapsed, set
		// to now for immidate fetching from fallback on next check.
		w.nextFallbackFetch = now
	}
}

// markUnhealthyOnWatchError handles errors from the watch loop.
func (w *ProxyKubeServerWatcher) markUnhealthyOnWatchError(err error) {
	now := time.Now()
	w.rw.Lock()
	defer w.rw.Unlock()
	if w.nextFallbackFetch.IsZero() {
		w.nextFallbackFetch = now.Add(retryutils.SeventhJitter(w.PrimaryTimeout))
		w.Logger.WarnContext(w.ctx, "Kube Server Watcher has failed.", "error", err, "next_fallback_fetch", w.nextFallbackFetch.Format(time.RFC3339))
	}
}

// fillEventBuf fills the given buffer with events from the watcher channel until the buffer reaches the given maxSize, the context is canceled or the watcher is closed.
func fillEventBuf(ctx context.Context, buf []types.Event, w types.Watcher, maxSize int) (out []types.Event) {
	out = buf // does not reset, caller is responsible for clearing the buffer after processing

	for len(out) < maxSize {
		select {
		case <-w.Done():
			return
		case <-ctx.Done():
			return
		case event := <-w.Events():
			out = append(out, event)
		default:
			return
		}
	}
	return
}

func (w *ProxyKubeServerWatcher) createWatcherAndInit() (types.Watcher, error) {
	watcher, err := w.AccessPoint.NewWatcher(w.ctx, types.Watch{
		Name:            w.Component,
		MetricComponent: w.Component,
		Kinds:           []types.WatchKind{{Kind: types.KindKubeServer}},
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating a watcher")
	}

	watcherInitTimeout := time.NewTimer(w.PrimaryTimeout)
	defer watcherInitTimeout.Stop()

	for {
		select {
		case <-watcher.Done():
			return nil, trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-w.ctx.Done():
			return nil, trace.ConnectionProblem(w.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			if event.Type != types.OpInit {
				return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
			}
		case <-watcherInitTimeout.C:
			// Do not return timeout here but mark the failure and continue waiting.
			// It's possible the watcher will recover eventually. If using the cache,
			// on error the cache will close the watcher and we handle that separately.
			w.markUnhealthyOnInitTimeout()
			w.Logger.WarnContext(w.ctx, "Watcher failed to initialize within the expected time. This is an indication of a problem with the primary access point.", "timeout", w.PrimaryTimeout.String())
			continue
		}
		break
	}

	return watcher, nil
}

// watch spawns a watcher on [types.KindKubeServer] and if successful, initializes the local cache and fetches events.
func (w *ProxyKubeServerWatcher) watch() error {
	watcher, err := w.createWatcherAndInit()
	if err != nil {
		return trace.Wrap(err, "creating watcher and waiting for init")
	}
	defer watcher.Close()

	newCurrent, err := w.getAllKubeServers(w.ctx, w.AccessPoint)
	if err != nil {
		return trace.Wrap(err, "fetching from primary")
	}

	var seen int
	queuedEvents := len(watcher.Events())
	eventBuf := make([]types.Event, 0, eventBufferSize)

	// Catch up on queued events that happened while we were fetching the initial state, these are applied to the
	// new state we just fetched, so we end up with a consistent view of the world at the point in time when the watcher was created.
	for seen < queuedEvents {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-w.ctx.Done():
			return trace.ConnectionProblem(w.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			eventBuf = append(eventBuf[:0], event)
			eventBuf = fillEventBuf(w.ctx, eventBuf, watcher, eventBufferSize)
			// no lock, events applied to local copy of the new state.
			w.applyEventsLocked(w.ctx, newCurrent, eventBuf)
			seen += len(eventBuf)
			clear(eventBuf)
		}
	}

	w.rw.Lock()
	w.nextFallbackFetch = time.Time{} // mark as healthy
	w.current = newCurrent
	w.rw.Unlock()

	w.initOnce.Do(func() {
		close(w.initC)
	})

	// At this point watcher is successfully initialized and the cache is warmed up, we can reset the retry backoff and start processing events.
	w.retry.Reset()

	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-w.ctx.Done():
			return trace.ConnectionProblem(w.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			eventBuf = append(eventBuf[:0], event)
			eventBuf = fillEventBuf(w.ctx, eventBuf, watcher, eventBufferSize)
			w.rw.Lock()
			w.applyEventsLocked(w.ctx, w.current, eventBuf)
			w.rw.Unlock()
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

// maybeFetchFromUpstream attempts to fetch the kube servers from the auth server if the cache is cold and
// the next cold fetch time has been reached. This is used to throttle calls to the auth server when the cache is cold.
func (w *ProxyKubeServerWatcher) maybeFetchFromUpstream(ctx context.Context) error {
	now := time.Now()

	if !w.shouldFetchFromFallback(now) {
		return nil
	}

	ch := w.singleFlighter.DoChan("collection", func() (any, error) {
		newCurrent, err := w.getAllKubeServers(w.ctx, w.FallbackGetter)
		if err != nil {
			return nil, trace.Wrap(err, "fetching from fallback")
		}

		w.rw.Lock()
		defer w.rw.Unlock()

		if w.isHealthyLocked() {
			// If the watcher became healhy while we were fetching from the fallback use the primary data.
			return nil, nil
		}

		w.current = newCurrent
		return nil, nil
	})

	select {
	case res := <-ch:
		return trace.Wrap(res.Err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "context is closing")
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
			w.markUnhealthyOnWatchError(err)
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

// applyEventsLocked takes events from the watcher channel and applies them to the given resources map
func (w *ProxyKubeServerWatcher) applyEventsLocked(ctx context.Context, resources map[serverKey]types.KubeServer, events []types.Event) {
	for _, event := range events {
		if event.Resource == nil || event.Resource.GetKind() != types.KindKubeServer {
			w.Logger.WarnContext(ctx, "Received unexpected event", "event", logutils.StringerAttr(event))
			continue
		}

		switch event.Type {
		case types.OpDelete:
			delete(resources, kubeServerDeleteKey(event.Resource))
		case types.OpPut:
			srv, err := types.ConvertResource[types.KubeServer](event.Resource)
			if err != nil {
				w.Logger.WarnContext(ctx, "Failed to convert event resource",
					"resource", event.Resource.GetKind(),
					"error", err,
				)
				continue
			}

			resources[kubeServerKey(srv)] = srv
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
		// and the fallback getter is failing. Log this as a warning.
		w.Logger.WarnContext(ctx, "Unhealthy watcher failed to fetch from upstream", "error", err)
		return nil, trace.Wrap(err)
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
