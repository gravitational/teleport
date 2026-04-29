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

package services

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/readonly"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
	"rsc.io/ordered"
)

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
// functional in a degrated state. This is currently strongly typed to kube servers but could be made more generic if needed in the future.
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

	// rw protects below fields
	rw sync.RWMutex

	// current holds a map of the currently known servers.
	current map[string]types.KubeServer

	// nextColdFetch is the time of the next allowed fetch of resources from the auth server.
	// Used to single flight calls to the auth server when the cache is cold.
	nextColdFetch time.Time

	// primaryFailureAt is the time when the primary access point was first observed to be failing.
	primaryFailureAt time.Time
}

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
		// Arm a cold fetch immediately, in the case the primary never initilizes and the consumer
		// does not wait for [ProxyKubeServerWatcher.WaitInitialization] to return this delays the
		// cold fetch to after the first timeout period with some jitter applied in case all proxy
		// caches fail the same way.
		nextColdFetch: time.Now().Add(cfg.PrimaryTimeout).Add(retryutils.FullJitter(cfg.FallbackInterval)),
		cancel:        cancel,
		ctx:           ctx,
		retry:         retry,
		initC:         make(chan struct{}),
	}

	go w.runWatchLoop()

	return w, nil
}

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

	select {
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
	case <-w.ctx.Done():
		return trace.ConnectionProblem(w.ctx.Err(), "context is closing")
	case <-time.After(w.PrimaryTimeout):
		return trace.ConnectionProblem(nil, "timed out waiting for initial watch event")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	if err := w.warmUpCache(w.ctx); err != nil {
		return trace.Wrap(err, "warming up cache")
	}

	// At this point watcher is successfully initialized and the cache is warmed up, we can reset the retry backoff and start processing events.
	w.retry.Reset()

	// start out with a modestly sized event buffer
	eventBuf := make([]types.Event, 0, 32)

	batchCollectEvents := func(ctx context.Context, w types.Watcher) {
		// resource collectors want to process events in batches
		// when possible in order to reduce contention on their locks.
		// we therefore optimistically try to gather a larger number of
		// events without blocking.
		for len(eventBuf) < eventBufferMaxSize {
			select {

			case <-w.Done():
				return
			case <-ctx.Done():
				return
			case event, ok := <-w.Events():
				// Safety check
				if !ok {
					return
				}
				eventBuf = append(eventBuf, event)
			default:
				return
			}
		}
	}

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
			eventBuf = append(eventBuf, event)
			batchCollectEvents(w.ctx, watcher)
			w.processEvents(w.ctx, eventBuf)
			clear(eventBuf)
			eventBuf = eventBuf[:0]
		}
	}
}

// kubeServerKey returns the key used to store the given kube server in the watcher's cache.
func kubeServerKey(resource types.KubeServer) string {
	return string(ordered.Encode(resource.GetHostID(), resource.GetName()))
}

// kubeServerDeleteKey returns the key used to delete the given kube server from the watcher's cache.
func kubeServerDeleteKey(resource types.Resource) string {
	// On delete events, the server description is populated with the host ID.
	return string(ordered.Encode(resource.GetMetadata().Description, resource.GetName()))
}

// getAllKubeServers fetches all kube servers from the given getter and returns them as a map keyed by the watcher's cache keys.
func (w *ProxyKubeServerWatcher) getAllKubeServers(ctx context.Context, getter KubernetesServerGetter) (map[string]types.KubeServer, error) {
	resources, err := getter.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	current := make(map[string]types.KubeServer, len(resources))
	for _, resource := range resources {
		current[kubeServerKey(resource)] = resource
	}
	return current, nil
}

// warmUpCache attempts to pre warm the cache after the watcher is initialized.
func (w *ProxyKubeServerWatcher) warmUpCache(ctx context.Context) error {
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
// the next cold fetch time has been reached. This is used to single flight calls to the auth server when the cache is cold.
func (w *ProxyKubeServerWatcher) maybeFetchFromUpstream(ctx context.Context) error {
	if w.hot.Load() {
		// fast path watcher is hot, no need to fetch from upstream
		return nil
	}

	w.rw.Lock()
	nextFetch := w.nextColdFetch
	w.rw.Unlock()

	if time.Now().Before(nextFetch) {
		return nil
	}

	newCurrent, err := w.getAllKubeServers(ctx, w.FallbackGetter)
	w.rw.Lock()
	defer w.rw.Unlock()

	// Arm the next cold fetch.
	w.nextColdFetch = time.Now().Add(retryutils.SeventhJitter(w.FallbackInterval))
	if err != nil {
		return trace.Wrap(err, "fetching from fallback")
	}

	// Check again in case the warm up succeeded while we were waiting for the lock.
	if w.hot.Load() {
		return nil
	}

	w.current = newCurrent
	return nil
}

// handleWatchError handles errors from the watch loop.
func (w *ProxyKubeServerWatcher) handleWatchError(err error) {
	w.rw.Lock()
	defer w.rw.Unlock()
	now := time.Now()
	if w.primaryFailureAt.IsZero() {
		w.primaryFailureAt = now
	} else if w.hot.Load() && time.Since(w.primaryFailureAt) > w.PrimaryTimeout {
		w.Logger.WarnContext(w.ctx, "Primary access point is unhealthy, falling back to auth server for kube servers.", "error", err, "failure_at", w.primaryFailureAt.String())
		w.nextColdFetch = now.Add(retryutils.FullJitter(w.FallbackInterval))
		w.hot.Store(false)
	}
}

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
	for {
		select {
		case <-w.initC:
			return nil
		case <-time.After(initTickerPeriod):
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

	toReadyOnly := func(resource types.KubeServer) readonly.KubeServer {
		return resource
	}

	var out []types.KubeServer
	for _, resource := range w.current {
		if filter(toReadyOnly(resource)) {
			out = append(out, types.KubeServer.Copy(resource))
		}
	}

	return out, nil
}
