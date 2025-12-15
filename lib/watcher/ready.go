// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package libwatcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

type watcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// WaitForReadyConfig contains configuration for waiting for an event stream to be ready.
type WaitForReadyConfig struct {
	// Watcher is the watcher to use for creating event watchers.
	Watcher watcher
	// Watch is the watch configuration.
	Watch types.Watch
	// Logger is the logger to use for logging.
	Logger *slog.Logger
	// Clock is the clock to use for timing operations.
	Clock clockwork.Clock
	// RetryConfig is the retry configuration. If nil, a default configuration will be used.
	RetryConfig *retryutils.RetryV2Config
}

func (c *WaitForReadyConfig) checkAndSetDefaults() error {
	if c.Watcher == nil {
		return trace.BadParameter("missing Watcher")
	}
	if c.Logger == nil {
		return trace.BadParameter("missing Logger")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.RetryConfig == nil {
		c.RetryConfig = &retryutils.RetryV2Config{
			First:  0,
			Driver: retryutils.NewLinearDriver(5 * time.Second),
			Max:    time.Minute,
			Jitter: retryutils.DefaultJitter,
			Clock:  c.Clock,
		}
	}
	if c.Watch.Name == "" {
		c.Watch = types.Watch{
			Name: "wait-for-ready",
			Kinds: []types.WatchKind{
				{Kind: types.KindClusterName},
			},
		}
	}
	return nil
}

// WaitForReady waits for an event stream to be ready by creating a watcher
// and waiting for an OpInit event. It retries on failure until successful or the
// context is canceled.
//
// An event stream can be either a Cache or Backend. This function is very
// useful when a service flow depends on the cache or backend being ready before proceeding.
// For instance, you don't want to fetch large collections from the backend while the cache
// is still syncing, as this would create unnecessary load on the backend.
func WaitForReady(ctx context.Context, cfg WaitForReadyConfig) error {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	start := cfg.Clock.Now()
	retry, err := retryutils.NewRetryV2(*cfg.RetryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	var attempt int
	for {
		attempt++
		// By default, return immediately on the first attempt without waiting.
		if err := waitForRetry(ctx, retry); err != nil {
			return trace.Wrap(err)
		}
		watcher, err := cfg.Watcher.NewWatcher(ctx, cfg.Watch)
		if err != nil {
			cfg.Logger.ErrorContext(ctx, "Failed to create watcher", "name", cfg.Watch.Name, "attempt", attempt, "error", err)
			continue
		}
		if err := waitForInitEvent(ctx, watcher, cfg, attempt, start); err != nil {
			continue
		}
		return nil
	}
}

// waitForRetry waits for the retry delay or returns if context is canceled.
func waitForRetry(ctx context.Context, retry *retryutils.RetryV2) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-retry.After():
		retry.Inc()
		return nil
	}
}

// waitForInitEvent waits for the OpInit event from the watcher. Returns nil on success,
// or an error if the watcher closes or an unexpected event is received.
func waitForInitEvent(ctx context.Context, w types.Watcher, cfg WaitForReadyConfig, attempt int, start time.Time) error {
	defer w.Close()
	select {
	case evt := <-w.Events():
		if evt.Type != types.OpInit {
			cfg.Logger.ErrorContext(ctx, "expected init event, got something else (this is a bug)", "name", cfg.Watch.Name, "attempt", attempt, "event_type", evt.Type)
			return trace.BadParameter("unexpected event type: %v", evt.Type)
		}
		cfg.Logger.DebugContext(ctx, "event stream initialized", "name", cfg.Watch.Name, "duration", cfg.Clock.Since(start).String())
		return nil

	case <-w.Done():
		cfg.Logger.ErrorContext(ctx, "watcher closed while waiting for init",
			"name", cfg.Watch.Name,
			"attempt", attempt,
			"error", w.Error())
		return trace.Wrap(w.Error())

	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
