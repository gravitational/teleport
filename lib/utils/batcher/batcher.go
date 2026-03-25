// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package batcher

import (
	"context"
	"io"
	"iter"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Run processes events from the given channel in batches, calling fn for each batch.
// Events are collected into batches during the time window with maximum threshold number of events.
//
// It is useful in cases of high event volume where events can be aggregated and
// processed together in a single operation. For example, when reacting to role
// change events and check what users have been affected by this change.
// Instead of fetching all the users for each role change event
// you can aggregate roles change events and fetch and process all users only once
// instead of fetching all the users from each role change event.
//
// Example:
//
//	   opts := []batcher.Option{
//	       batcher.WithClock(u.clock),
//	       batcher.WithWindow(time.Second * 10),
//	       batcher.WithThreshold(100),
//	   }
//
//		err := batcher.Run(ctx, eventChan, func(batch []Event) error {
//			log.Printf("Processing %d events", len(batch))
//			return processEvents(batch)
//		}, opts...))
//
// The function returns when the context is canceled, returning ctx.Err(),
// when fn returns an error, returning that error, or when the events channel is closed.
func Run[T any](ctx context.Context, events <-chan T, fn func(batch []T) error, opts ...Option) error {
	collector := New[T](opts...)
	return trace.Wrap(collector.Run(ctx, events, fn))
}

// RunWithState is like Run but also provides the current State to the processing function.
// The state is managed by the provided StateMonitor and updated based on the batch size.
func RunWithState[T any](
	ctx context.Context,
	events <-chan T,
	fn func(batch []T, state State) error,
	stateManager *StateMonitor,
	opts ...Option,
) error {
	return trace.Wrap(Run(ctx, events, func(batch []T) error {
		currentState := stateManager.UpdateState(len(batch))
		return trace.Wrap(fn(batch, currentState))
	}, opts...))
}

// Iter allows to iterate over batches of events.
func Iter[T any](ctx context.Context, events <-chan T, opts ...Option) iter.Seq2[[]T, error] {
	collector := New[T](opts...)
	return func(yield func([]T, error) bool) {
		for {
			batch, err := collector.CollectBatch(ctx, events)
			if !yield(batch, err) {
				return
			}
		}
	}
}

// Option configures a Collector using the functional options pattern.
type Option func(*config)

// WithWindow sets the collection window duration.
func WithWindow(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.window = d
		}
	}
}

// WithThreshold sets the maximum number of events per batch.
func WithThreshold(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.threshold = n
		}
	}
}

// WithClock sets a custom clock for testing.
// Default uses the system clock.
func WithClock(clock clockwork.Clock) Option {
	return func(c *config) {
		if clock != nil {
			c.clock = clock
		}
	}
}

type config struct {
	window    time.Duration
	threshold int
	clock     clockwork.Clock
}

// New creates a new Collector with the given options.
func New[T any](opts ...Option) *Collector[T] {
	cfg := config{
		window:    10 * time.Second,
		threshold: 50,
		clock:     clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Collector[T]{config: cfg}
}

// Collector batches events into time windows.
type Collector[T any] struct {
	config
}

// Run starts collecting events and calls fn for each batch.
// It blocks until ctx is canceled or an error occurs.
func (c *Collector[T]) Run(ctx context.Context, events <-chan T, fn func(batch []T) error) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		batch, err := c.CollectBatch(ctx, events)
		if err != nil {
			return err
		}

		if len(batch) > 0 {
			if err := fn(batch); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// CollectBatch collects events into a batch until the time window elapses
// or the threshold is reached, whichever comes first.
// It blocks until at least one event is collected or ctx is canceled.
//
// NOTE: CollectBatch returns already collected events if ctx is canceled.
// If you want to still process then you need to check the error returned
// and handle ctx.Err() case separately.
func (c *Collector[T]) CollectBatch(ctx context.Context, events <-chan T) ([]T, error) {
	var timer clockwork.Timer
	var timeout <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	batch := make([]T, 0, c.threshold)

	for {
		select {
		case <-ctx.Done():
			return batch, ctx.Err()

		case <-timeout:
			return batch, nil

		case event, ok := <-events:
			if !ok {
				// Channel closed, return what we have
				return batch, io.EOF
			}
			batch = append(batch, event)
			// The batch is full, return it
			// to not allow to grow the memory indefinitely.
			if len(batch) >= c.threshold {
				return batch, nil
			}
			if len(batch) == 1 {
				// Start the timer on the first event instead of during the call
				// to avoid returning empty batches and block if there are no events.
				if timer == nil {
					timer = c.clock.NewTimer(c.window)
					timeout = timer.Chan()
				}
			}

		}
	}
}
