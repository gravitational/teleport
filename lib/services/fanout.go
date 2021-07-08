/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
)

const defaultQueueSize = 64

type fanoutEntry struct {
	kind    types.WatchKind
	watcher *fanoutWatcher
}

// Fanout is a helper which allows a stream of events to be fanned-out to many
// watchers.  Used by the cache layer to forward events.
type Fanout struct {
	mu           sync.Mutex
	init, closed bool
	watchers     map[string][]fanoutEntry
	// eventsCh is used in tests
	eventsCh chan FanoutEvent
}

// NewFanout creates a new Fanout instance in an uninitialized
// state.  Until initialized, watchers will be queued but no
// events will be sent.
func NewFanout(eventsCh ...chan FanoutEvent) *Fanout {
	f := &Fanout{
		watchers: make(map[string][]fanoutEntry),
	}
	if len(eventsCh) != 0 {
		f.eventsCh = eventsCh[0]
	}
	return f
}

const (
	// EventWatcherRemoved is emitted when event watcher has been removed
	EventWatcherRemoved = iota
)

// FanoutEvent is used in tests
type FanoutEvent struct {
	// Kind is event kind
	Kind int
}

// NewWatcher attaches a new watcher to this fanout instance.
func (f *Fanout) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, trace.Errorf("cannot register watcher, fanout system closed")
	}

	w, err := newFanoutWatcher(ctx, f, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if f.init {
		// fanout is already initialized; emit OpInit immediately.
		if err := w.emit(types.Event{Type: types.OpInit}); err != nil {
			w.cancel()
			return nil, trace.Wrap(err)
		}
	}
	f.addWatcher(w)
	return w, nil
}

// SetInit sets Fanout into an initialized state, sending OpInit events
// to any watchers which were added prior to initialization.
func (f *Fanout) SetInit() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.init {
		return
	}
	for _, entries := range f.watchers {
		var remove []*fanoutWatcher
		for _, entry := range entries {
			if err := entry.watcher.emit(types.Event{Type: types.OpInit}); err != nil {
				entry.watcher.setError(err)
				remove = append(remove, entry.watcher)
			}
		}
		for _, w := range remove {
			f.removeWatcher(w)
			w.cancel()
		}
	}
	f.init = true
}

func filterEventSecrets(event types.Event) types.Event {
	r, ok := event.Resource.(types.ResourceWithSecrets)
	if !ok {
		return event
	}
	event.Resource = r.WithoutSecrets()
	return event
}

// Len returns a total count of watchers
func (f *Fanout) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int
	for key := range f.watchers {
		count += len(f.watchers[key])
	}
	return count
}

func (f *Fanout) trySendEvent(e FanoutEvent) {
	if f.eventsCh == nil {
		return
	}
	select {
	case f.eventsCh <- e:
	default:
	}
}

// Emit broadcasts events to all matching watchers that have been attached
// to this fanout instance.
func (f *Fanout) Emit(events ...types.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.init {
		panic("Emit called on uninitialized fanout instance")
	}
	for _, fullEvent := range events {
		// by default, we operate on a version of the event which
		// has had secrets filtered out.
		event := filterEventSecrets(fullEvent)
		var remove []*fanoutWatcher
	Inner:
		for _, entry := range f.watchers[event.Resource.GetKind()] {
			match, err := entry.kind.Matches(event)
			if err != nil {
				entry.watcher.setError(err)
				remove = append(remove, entry.watcher)
				continue Inner
			}
			if !match {
				continue Inner
			}
			emitEvent := event
			// if this entry loads secrets, emit the
			// full unfiltered event.
			if entry.kind.LoadSecrets {
				emitEvent = fullEvent
			}
			if err := entry.watcher.emit(emitEvent); err != nil {
				remove = append(remove, entry.watcher)
				continue Inner
			}
		}
		for _, w := range remove {
			f.removeWatcher(w)
			w.cancel()
		}
	}
}

// Reset closes all attached watchers and places the fanout instance
// into an uninitialized state.  Reset may be called on an uninitialized
// fanout instance to remove "queued" watchers.
func (f *Fanout) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeWatchers()
	f.init = false
}

// Close permanently closes the fanout.  Existing watchers will be
// closed and no new watchers will be added.
func (f *Fanout) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeWatchers()
	f.closed = true
}

func (f *Fanout) closeWatchers() {
	for _, entries := range f.watchers {
		for _, entry := range entries {
			entry.watcher.cancel()
		}
	}
	// watcher map was potentially quite large, so
	// relenguish that memory.
	f.watchers = make(map[string][]fanoutEntry)
}

func (f *Fanout) addWatcher(w *fanoutWatcher) {
	for _, kind := range w.watch.Kinds {
		entries := f.watchers[kind.Kind]
		entries = append(entries, fanoutEntry{
			kind:    kind,
			watcher: w,
		})
		f.watchers[kind.Kind] = entries
	}
}

func (f *Fanout) removeWatcherWithLock(w *fanoutWatcher) {
	if w == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removeWatcher(w)
}

func (f *Fanout) removeWatcher(w *fanoutWatcher) {
	for _, kind := range w.watch.Kinds {
		entries := f.watchers[kind.Kind]
	Inner:
		for i, entry := range entries {
			if entry.watcher == w {
				entries = append(entries[:i], entries[i+1:]...)
				f.trySendEvent(FanoutEvent{Kind: EventWatcherRemoved})
				break Inner
			}
		}
		switch len(entries) {
		case 0:
			delete(f.watchers, kind.Kind)
		default:
			f.watchers[kind.Kind] = entries
		}
	}
}

func newFanoutWatcher(ctx context.Context, f *Fanout, watch types.Watch) (*fanoutWatcher, error) {
	if len(watch.Kinds) < 1 {
		return nil, trace.BadParameter("must specify at least one resource kind to watch")
	}
	ctx, cancel := context.WithCancel(ctx)
	if watch.QueueSize < 1 {
		watch.QueueSize = defaultQueueSize
	}
	return &fanoutWatcher{
		fanout: f,
		watch:  watch,
		eventC: make(chan types.Event, watch.QueueSize),
		cancel: cancel,
		ctx:    ctx,
	}, nil
}

type fanoutWatcher struct {
	emux   sync.Mutex
	fanout *Fanout
	err    error
	watch  types.Watch
	eventC chan types.Event
	cancel context.CancelFunc
	ctx    context.Context
}

func (w *fanoutWatcher) emit(event types.Event) error {
	select {
	case <-w.ctx.Done():
		return trace.Wrap(w.ctx.Err(), "watcher closed")
	case w.eventC <- event:
		return nil
	default:
		return trace.BadParameter("buffer overflow")
	}
}

func (w *fanoutWatcher) Events() <-chan types.Event {
	return w.eventC
}

func (w *fanoutWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

func (w *fanoutWatcher) Close() error {
	w.cancel()
	// goroutine is to prevent accidental
	// deadlock, if watcher.Close is called
	// under Fanout mutex
	go w.fanout.removeWatcherWithLock(w)
	return nil
}

func (w *fanoutWatcher) setError(err error) {
	w.emux.Lock()
	defer w.emux.Unlock()
	w.err = err
}

func (w *fanoutWatcher) Error() error {
	w.emux.Lock()
	defer w.emux.Unlock()
	if w.err != nil {
		return w.err
	}
	select {
	case <-w.Done():
		return trace.Errorf("watcher closed")
	default:
		return nil
	}
}
