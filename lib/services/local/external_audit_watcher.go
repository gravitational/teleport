/*
Copyright 2023 Gravitational, Inc.

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

package local

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

var ErrClusterExternalCloudAuditWatcherClosed = errors.New("cluster external cloud audit watcher closed")

// ClusterExternalCloudAuditWatcherConfig contains configuration options for a ClusterExternalAuditWatcher.
type ClusterExternalCloudAuditWatcherConfig struct {
	// Backend is the storage backend used to create watchers.
	Backend backend.Backend
	// Log is a logger.
	Log logrus.FieldLogger
	// Clock is used to control time.
	Clock clockwork.Clock
	// MaxRetryPeriod is the maximum retry period on failed watchers.
	MaxRetryPeriod time.Duration

	OnChange func()
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ClusterExternalCloudAuditWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
		cfg.Log.WithField("resource-kind", types.KindExternalCloudAudit)
	}
	if cfg.MaxRetryPeriod == 0 {
		// On watcher failure, we eagerly retry in order to avoid login delays.
		cfg.MaxRetryPeriod = defaults.HighResPollingPeriod
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
	ClusterExternalCloudAuditWatcherConfig
	retry retryutils.Retry
	sync.Mutex
	closed  chan struct{}
	running chan struct{}
}

// NewClusterExternalAuditWatcher creates a new cluster external audit resource watcher.
// The watcher will close once the given ctx is closed.
func NewClusterExternalAuditWatcher(ctx context.Context, cfg ClusterExternalCloudAuditWatcherConfig) (*ClusterExternalAuditWatcher, error) {
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

	w := &ClusterExternalAuditWatcher{
		ClusterExternalCloudAuditWatcherConfig: cfg,
		retry:                                  retry,
		closed:                                 make(chan struct{}),
		running:                                make(chan struct{}),
	}

	go w.runWatchLoop(ctx)

	return w, nil
}

// WaitInit waits for the watch loop to initialize.
func (w *ClusterExternalAuditWatcher) WaitInit(ctx context.Context) error {
	select {
	case <-w.running:
	case <-ctx.Done():
	}
	return trace.Wrap(ctx.Err())
}

// Done returns a channel that's closed when the watcher is closed.
func (w *ClusterExternalAuditWatcher) Done() <-chan struct{} {
	return w.closed
}

func (w *ClusterExternalAuditWatcher) close() {
	w.Lock()
	defer w.Unlock()
	close(w.closed)
}

func (w *ClusterExternalAuditWatcher) runWatchLoop(ctx context.Context) {
	defer w.close()
	for {
		err := w.watch(ctx)

		startedWaiting := w.Clock.Now()
		select {
		case t := <-w.retry.After():
			w.Log.Warningf("BYOB Restarting watch on error after waiting %v. Error: %v.", t.Sub(startedWaiting), err)
			w.retry.Inc()
		case <-ctx.Done():
			w.Log.WithError(ctx.Err()).Debugf("BYOB Context closed with err. Returning from watch loop.")
			return
		case <-w.closed:
			w.Log.Debug("BYOB Watcher closed. Returning from watch loop.")
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
		case event := <-watcher.Events():
			// TODO(tobiaszheller): every cluster external audit change should trriggger reload
			w.Log.Infof("BYOB reloading byob. %v", event.Type)
			w.OnChange()
		case <-watcher.Done():
			return errors.New("watcher closed")
		case <-ctx.Done():
			return ctx.Err()
		case w.running <- struct{}{}:
		}
	}
}

func (w *ClusterExternalAuditWatcher) newWatcher(ctx context.Context) (backend.Watcher, error) {
	watcher, err := w.Backend.NewWatcher(ctx, backend.Watch{
		Name: types.KindExternalCloudAudit,
		// TODO(tobiaszheller): what's benefix of that metric, and check cardinality.
		MetricComponent: types.KindExternalCloudAudit,
		Prefixes:        [][]byte{clusterExternalCloudAuditBackendKey},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case <-watcher.Done():
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
