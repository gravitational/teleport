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

package local

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
)

// ClusterExternalAuditStorageWatcherConfig contains configuration options for a ClusterExternalAuditWatcher.
type ClusterExternalAuditStorageWatcherConfig struct {
	// Backend is the storage backend used to create watchers.
	Backend backend.Backend
	// Logger is a logger.
	Logger *slog.Logger
	// Clock is used to control time.
	Clock clockwork.Clock
	// OnChange is the action to take when the cluster ExternalAuditStorage
	// changes.
	OnChange func()
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ClusterExternalAuditStorageWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "ExternalAuditStorage.watcher")
	}
	if cfg.Clock == nil {
		cfg.Clock = cfg.Backend.Clock()
	}
	if cfg.OnChange == nil {
		return trace.BadParameter("missing parameter OnChange")
	}
	return nil
}

// ClusterExternalAuditWatcher is a light weight backend watcher for the cluster external audit resource.
type ClusterExternalAuditWatcher struct {
	backend   backend.Backend
	logger    *slog.Logger
	clock     clockwork.Clock
	onChange  func()
	retry     retryutils.Retry
	running   chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	done      chan struct{}
}

// NewClusterExternalAuditWatcher creates a new cluster external audit resource watcher.
// The watcher will close once the given ctx is closed.
func NewClusterExternalAuditWatcher(ctx context.Context, cfg ClusterExternalAuditStorageWatcherConfig) (*ClusterExternalAuditWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  defaults.HighResPollingPeriod,
		Driver: retryutils.NewExponentialDriver(defaults.HighResPollingPeriod),
		Max:    defaults.LowResPollingPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	w := &ClusterExternalAuditWatcher{
		backend:  cfg.Backend,
		logger:   cfg.Logger,
		clock:    cfg.Clock,
		onChange: cfg.OnChange,
		retry:    retry,
		running:  make(chan struct{}),
		closed:   make(chan struct{}),
		done:     make(chan struct{}),
	}

	go w.runWatchLoop(ctx)

	return w, nil
}

// WaitInit waits for the watch loop to initialize.
func (w *ClusterExternalAuditWatcher) WaitInit(ctx context.Context) error {
	select {
	case <-w.running:
		return nil
	case <-w.done:
		return trace.Errorf("watcher closed")
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// close stops the watcher and waits for the watch loop to exit
func (w *ClusterExternalAuditWatcher) close() {
	w.closeOnce.Do(func() { close(w.closed) })
	<-w.done
}

func (w *ClusterExternalAuditWatcher) runWatchLoop(ctx context.Context) {
	defer close(w.done)
	for {
		err := w.watch(ctx)

		startedWaiting := w.clock.Now()
		select {
		case t := <-w.retry.After():
			w.logger.WarnContext(ctx, "Restarting watch on error",
				"backoff", t.Sub(startedWaiting),
				"error", err,
			)
			w.retry.Inc()
		case <-ctx.Done():
			return
		case <-w.closed:
			return
		}
	}
}

func (w *ClusterExternalAuditWatcher) watch(ctx context.Context) error {
	watcher, err := w.newWatcher(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	for {
		select {
		case <-watcher.Events():
			w.logger.InfoContext(ctx, "Detected change to cluster ExternalAuditStorage config")
			w.onChange()
		case w.running <- struct{}{}:
		case <-watcher.Done():
			return trace.Errorf("watcher closed")
		case <-ctx.Done():
			return ctx.Err()
		case <-w.closed:
			return nil
		}
	}
}

func (w *ClusterExternalAuditWatcher) newWatcher(ctx context.Context) (backend.Watcher, error) {
	watcher, err := w.backend.NewWatcher(ctx, backend.Watch{
		Name:     types.KindExternalAuditStorage,
		Prefixes: []backend.Key{clusterExternalAuditStorageBackendKey},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case <-watcher.Done():
		return nil, errors.New("watcher closed")
	case <-w.closed:
		return nil, errors.New("watcher closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	w.retry.Reset()
	return watcher, nil
}
