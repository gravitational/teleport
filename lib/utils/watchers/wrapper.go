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

package watchers

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// WatcherSource is a source of types.Watcher instances.
type WatcherSource interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// Retrier implements a retry/backoff strategy.
type Retrier interface {
	// Reset resets the state of the retrier (for example, zero the number of
	// attempts).
	Reset()
	// NextDelay returns the delay for the next attempt.
	// The first attempt is expected to be immediate (ie, no delay).
	NextDelay() time.Duration
}

// WrapperConfig holds creation parameters for Wrapper.
type WrapperConfig struct {
	// Clock used by the Wrapper.
	// Defaults to a real clock.
	Clock clockwork.Clock
	// Logger used by the Wrapper.
	// Defaults to slog.Default().
	Logger *slog.Logger
	// Source used to create Watchers.
	// Required.
	Source WatcherSource
	// EventsChannelSize is the size of the Wrapper's events channel.
	// Defaults to zero/unbuffered.
	EventsChannelSize int
	// Retrier is the retry/backoff strategy for re-creating failed/closed watchers.
	// Defaults to an exponential backoff strategy with an initial delay of 1m.
	Retrier Retrier

	// Watch is the spec of events to watch.
	// Required.
	Watch *types.Watch
}

// Wrapper is a wrapper over [types.Watcher] that automatically handles Watcher
// initialization, disconnection and failures.
//
// Wrapper automatically reconnects to the upstream watcher, spacing attempts
// according to a user-supplied Retrier.
//
// Users of Wrapper can concern themselves simply with handling events.
// See [Wrapper.Run] and [Wrapper.Events].
type Wrapper struct {
	clock    clockwork.Clock
	logger   *slog.Logger
	source   WatcherSource
	watch    *types.Watch
	retrier  Retrier
	events   chan types.Event
	healthyC chan struct{} // Communicates one-shot healthy state. Used for testing.

	// Run state.
	running        atomic.Bool
	logNextSuccess bool
}

// NewWrapper creates a new Watcher wrapper using the given config.
func NewWrapper(cfg WrapperConfig) (*Wrapper, error) {
	switch {
	case cfg.Source == nil:
		return nil, trace.BadParameter("source required")
	case cfg.EventsChannelSize < 0:
		return nil, trace.BadParameter("cfg.EventsChannelSize must be zero or positive")
	case cfg.Watch == nil:
		return nil, trace.BadParameter("watch specification required")
	}

	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	watchShallowCopy := *cfg.Watch

	retrier := cfg.Retrier
	if retrier == nil {
		retrier = newDefaultRetrier()
	}

	w := &Wrapper{
		clock:    clock,
		logger:   logger,
		source:   cfg.Source,
		watch:    &watchShallowCopy,
		retrier:  retrier,
		events:   make(chan types.Event, cfg.EventsChannelSize),
		healthyC: make(chan struct{}, 1),
	}

	return w, nil
}

// Events returns the Wrapper events channel.
//
// Callers should consume the channel in a timely manner, and/or specify an
// appropriate buffer size via config.
//
// The OpInit event is swallowed by Wrapper, so it won't be seen in the returned
// channel.
func (w *Wrapper) Events() <-chan types.Event {
	return w.events
}

// Run executes the Wrapper watch loop. It runs indefinitely, reconnecting to
// the upstream Watcher on failures.
//
// Run only stops if ctx is closed. Returns the context error.
//
// A Wrapper may only Run once.
func (w *Wrapper) Run(ctx context.Context) error {
	if !w.running.CompareAndSwap(false, true) {
		return trace.Wrap(errors.New("method Run already called"))
	}

	var timer <-chan time.Time
	for {
		d := w.retrier.NextDelay()
		timer = w.clock.After(d)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer:
			// OK, continue.
		}

		watcher, err := w.source.NewWatcher(ctx, *w.watch)
		if err != nil {
			w.logger.WarnContext(ctx,
				"Failed to create new Watcher. Service is unable to watch events until a connection is reestablished.",
				"error", err,
			)
			w.logNextSuccess = true
			continue
		}
		if w.logNextSuccess {
			w.logger.InfoContext(ctx, "Watcher connection restored")
			w.logNextSuccess = false
		} else {
			w.logger.DebugContext(ctx, "Watcher connection established")
		}

		abortErr := w.runEventsLoop(ctx, watcher)
		if err := watcher.Close(); err != nil {
			w.logger.DebugContext(ctx, "Error closing Watcher", "error", err)
		}
		if abortErr != nil {
			return trace.Wrap(abortErr)
		}

		w.logger.DebugContext(ctx, "Watcher connection aborted, attempting to reestablish after backoff")
	}
}

// runEventsLoop runs the watcher event loop. Runs until the ctx closes, the
// watcher closes, or an irrecoverable error is encountered (eg, an incorrect
// first event).
//
// Returns nil if the Watcher should reconnect, non-nil if it should abort.
func (w *Wrapper) runEventsLoop(ctx context.Context, watcher types.Watcher) (abortErr error) {
	// The first event MUST be an OpInit event, as dictated by the secret
	// rules of watchers. If it's not then we must fail.
	//
	// * lib/services/watcher.go:336
	// * https://github.com/gravitational/teleport/blob/1f0ca9e4ae66a47f39d10c40f35e55d5ac5e15ac/lib/services/watcher.go#L336-L338
	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-watcher.Done():
		return nil // Reconnect.

	case e := <-watcher.Events():
		if e.Type != types.OpInit {
			w.logger.WarnContext(ctx,
				"Watcher received non-init event as first event, aborting",
				"op", e.Type,
			)
			w.logNextSuccess = true
			return nil // Reconnect.
		}
	}
	w.logger.DebugContext(ctx, "Watcher init event received")

	// Reset backoff after the init event. This our mark for "success".
	w.retrier.Reset()
	w.markHealthyForTesting()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-watcher.Done():
			return nil // Reconnect.

		case e := <-watcher.Events():
			eLog := w.logger.With("op", e.Type)
			if e.Resource != nil {
				eLog = eLog.With(
					"kind", e.Resource.GetKind(),
					"sub_kind", e.Resource.GetSubKind(),
					"name", e.Resource.GetName(),
					"revision", e.Resource.GetRevision(),
				)
			}
			eLog.DebugContext(ctx, "Received watcher event")

			// Forward event to w.events.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case w.events <- e:
			}
		}
	}
}

func (w *Wrapper) markHealthyForTesting() {
	select {
	case w.healthyC <- struct{}{}:
	default:
	}
}

type exponentialRetrier struct {
	d       retryutils.Driver
	attempt uint
}

func (e *exponentialRetrier) Reset() {
	e.attempt = 0
}

func (e *exponentialRetrier) NextDelay() time.Duration {
	a := e.attempt
	d := e.d.Duration(int64(a))
	e.attempt++
	return d
}

func newDefaultRetrier() Retrier {
	const initialDelay = 1 * time.Minute
	return &exponentialRetrier{
		d: retryutils.NewExponentialDriver(initialDelay),
	}
}
