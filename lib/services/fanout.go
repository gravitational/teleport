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

	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
)

const defaultQueueSize = 64

type fanoutEntry struct {
	kind    WatchKind
	watcher *fanoutWatcher
}

// Fanout is a helper which allows a stream of events to be fanned-out to many
// watchers.  Used by the cache layer to forward events.
type Fanout struct {
	mu       sync.Mutex
	watchers map[string][]fanoutEntry
}

// NewFanout creates a new Fanout instance.
func NewFanout() *Fanout {
	return &Fanout{
		watchers: make(map[string][]fanoutEntry),
	}
}

// NewWatcher attaches a new watcher to this fanout instance.
func (f *Fanout) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, err := newFanoutWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := w.emit(Event{Type: backend.OpInit}); err != nil {
		w.cancel()
		return nil, trace.Wrap(err)
	}
	f.addWatcher(w)
	return w, nil
}

func filterEventSecrets(event Event) Event {
	r, ok := event.Resource.(ResourceWithSecrets)
	if !ok {
		return event
	}
	event.Resource = r.WithoutSecrets()
	return event
}

// Emit broadcasts events to all matching watchers that have been attached
// to this fanout instance.
func (f *Fanout) Emit(events ...Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// CloseWatchers closes all attached watchers, effectively
// resetting the Fanout instance.
func (f *Fanout) CloseWatchers() {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (f *Fanout) removeWatcher(w *fanoutWatcher) {
	for _, kind := range w.watch.Kinds {
		entries := f.watchers[kind.Kind]
	Inner:
		for i, entry := range entries {
			if entry.watcher == w {
				entries = append(entries[:i], entries[i+1:]...)
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

func newFanoutWatcher(ctx context.Context, watch Watch) (*fanoutWatcher, error) {
	if len(watch.Kinds) < 1 {
		return nil, trace.BadParameter("must specify at least one resource kind to watch")
	}
	ctx, cancel := context.WithCancel(ctx)
	if watch.QueueSize < 1 {
		watch.QueueSize = defaultQueueSize
	}
	return &fanoutWatcher{
		watch:  watch,
		eventC: make(chan Event, watch.QueueSize),
		cancel: cancel,
		ctx:    ctx,
	}, nil
}

type fanoutWatcher struct {
	emux   sync.Mutex
	err    error
	watch  Watch
	eventC chan Event
	cancel context.CancelFunc
	ctx    context.Context
}

func (w *fanoutWatcher) emit(event Event) error {
	select {
	case <-w.ctx.Done():
		return trace.Wrap(w.ctx.Err(), "watcher closed")
	case w.eventC <- event:
		return nil
	default:
		return trace.BadParameter("buffer overflow")
	}
}

func (w *fanoutWatcher) Events() <-chan Event {
	return w.eventC
}

func (w *fanoutWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

func (w *fanoutWatcher) Close() error {
	w.cancel()
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
	return w.err
}
