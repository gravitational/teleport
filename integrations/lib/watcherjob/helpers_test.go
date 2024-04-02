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

package watcherjob

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
)

// MockWatcher is a mock events watcher.
type MockWatcher struct {
	events <-chan types.Event
	ctx    context.Context
	cancel context.CancelFunc
}

// MockEvents is mock watcher builder.
type MockEvents struct {
	sync.Mutex
	channels []chan<- types.Event
}

// NewWatcher creates a new watcher.
func (e *MockEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	events := make(chan types.Event, 1000)
	e.Lock()
	e.channels = append(e.channels, events)
	e.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	return MockWatcher{events: events, ctx: ctx, cancel: cancel}, ctx.Err()
}

// Fire emits a watcher events for all the subscribers to consume.
func (e *MockEvents) Fire(event types.Event) {
	e.Lock()
	channels := e.channels
	e.Unlock()
	for _, events := range channels {
		events <- event
	}
}

// WaitSomeWatchers blocks until either some watcher is subscribed or context is done.
func (e *MockEvents) WaitSomeWatchers(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.Lock()
			n := len(e.channels)
			e.Unlock()
			if n > 0 {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// Events returns a stream of events.
func (w MockWatcher) Events() <-chan types.Event {
	return w.events
}

// Done returns a completion channel.
func (w MockWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// Close sends a termination signal to watcher.
func (w MockWatcher) Close() error {
	w.cancel()
	return nil
}

// Error returns a watcher error.
func (w MockWatcher) Error() error {
	return trace.Wrap(w.ctx.Err())
}

// MockEventsProcess is a new process with a mock events streamer.
type MockEventsProcess struct {
	*lib.Process
	eventsJob lib.ServiceJob
	Events    MockEvents
}

// NewMockEventsProcess creates a new process.
func NewMockEventsProcess(ctx context.Context, t *testing.T, config Config, fn EventFunc) *MockEventsProcess {
	t.Helper()
	process := MockEventsProcess{
		Process: lib.NewProcess(ctx),
	}
	t.Cleanup(func() {
		process.Terminate()
		err := process.Shutdown(ctx)
		assert.ErrorContains(t, err, context.Canceled.Error(), "if a non-nil error is returned, it should be canceled context")
		process.Close()
	})
	var err error
	process.eventsJob, err = NewJobWithEvents(&process.Events, config, fn)
	require.NoError(t, err)
	process.SpawnCriticalJob(process.eventsJob)
	require.NoError(t, process.Events.WaitSomeWatchers(ctx))
	process.Events.Fire(types.Event{Type: types.OpInit})

	return &process
}

// Shutdown sends a termination signal and waits for process completion.
func (process *MockEventsProcess) Shutdown(ctx context.Context) error {
	process.Terminate()
	job := process.eventsJob
	select {
	case <-job.Done():
		select {
		case <-process.Done():
			return trace.Wrap(job.Err())
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// Countdown is a convenient WaitGroup wrapper which you can wait with deadline.
type Countdown struct {
	wg   sync.WaitGroup
	done chan struct{}
}

// NewCountdown creates a countdown with a given number of count.
func NewCountdown(n int) *Countdown {
	countdown := Countdown{done: make(chan struct{})}
	countdown.wg.Add(n)
	go func() {
		countdown.wg.Wait()
		close(countdown.done)
	}()
	return &countdown
}

// Decrement atomically subtracts one from the counter.
func (countdown *Countdown) Decrement() {
	countdown.wg.Done()
}

// Wait blocks until either countdown or context is done.
func (countdown *Countdown) Wait(ctx context.Context) error {
	select {
	case <-countdown.done:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
