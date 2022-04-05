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
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"

	"go.uber.org/atomic"
)

const defaultQueueSize = 64

type fanoutEntry struct {
	kind    types.WatchKind
	watcher *fanoutWatcher
}

type bufferConfig struct {
	gracePeriod time.Duration
	capacity    int
	clock       clockwork.Clock
}

type BufferOption func(*bufferConfig)

// BufferCapacity sets the event capacity of the circular buffer.
func BufferCapacity(c int) BufferOption {
	return func(cfg *bufferConfig) {
		if c > 0 {
			cfg.capacity = c
		}
	}
}

// BacklogGracePeriod sets the amount of time a watcher with a backlog will be tolerated.
func BacklogGracePeriod(d time.Duration) BufferOption {
	return func(cfg *bufferConfig) {
		if d > 0 {
			cfg.gracePeriod = d
		}
	}
}

// BufferClock sets a custom clock for the buffer (used in tests).
func BufferClock(c clockwork.Clock) BufferOption {
	return func(cfg *bufferConfig) {
		if c != nil {
			cfg.clock = c
		}
	}
}

// Fanout is a helper which allows a stream of events to be fanned-out to many
// watchers.  Used by the cache layer to forward events.
type Fanout struct {
	mu           sync.Mutex
	init, closed bool
	watchers     map[string][]fanoutEntry
	// eventsCh is used in tests
	eventsCh chan FanoutEvent
	cfg      bufferConfig
}

// NewFanout creates a new Fanout instance in an uninitialized
// state.  Until initialized, watchers will be queued but no
// events will be sent.
func NewFanout(eventsCh ...chan FanoutEvent) *Fanout {
	cfg := bufferConfig{
		gracePeriod: backend.DefaultBacklogGracePeriod,
		capacity:    backend.DefaultBufferCapacity,
		clock:       clockwork.NewRealClock(),
	}

	f := &Fanout{
		watchers: make(map[string][]fanoutEntry),
		cfg:      cfg,
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
		// fanout is already initialized; emit init event immediately.
		if !w.init() {
			w.cancel()
			return nil, trace.BadParameter("failed to send init event")
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
			if !entry.watcher.init() {
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
		// If the event has no associated resource, emit it to all watchers.
		if event.Resource == nil {
			for _, entries := range f.watchers {
				for _, entry := range entries {
					if err := entry.watcher.emit(event); err != nil {
						entry.watcher.setError(err)
						remove = append(remove, entry.watcher)
					}
				}
			}
		} else {
			for _, entry := range f.watchers[event.Resource.GetKind()] {
				match, err := entry.kind.Matches(event)
				if err != nil {
					entry.watcher.setError(err)
					remove = append(remove, entry.watcher)
					continue
				}
				if !match {
					continue
				}
				emitEvent := event
				// if this entry loads secrets, emit the
				// full unfiltered event.
				if entry.kind.LoadSecrets {
					emitEvent = fullEvent
				}
				if err := entry.watcher.emit(emitEvent); err != nil {
					entry.watcher.setError(err)
					remove = append(remove, entry.watcher)
				}
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
	f.closeWatchersAsync()
	f.init = false
}

// Close permanently closes the fanout.  Existing watchers will be
// closed and no new watchers will be added.
func (f *Fanout) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeWatchersAsync()
	f.closed = true
}

// closeWatchersAsync moves ownership of the watcher mapping to a background goroutine
// for asynchronous cancellation and sets up a new empty mapping.
func (f *Fanout) closeWatchersAsync() {
	watchersToClose := f.watchers
	f.watchers = make(map[string][]fanoutEntry)
	// goroutines run with a "happens after" releationship to the
	// expressions that create them.  since we move ownership of the
	// old watcher mapping prior to spawning this goroutine, we are
	// "safe" to modify it without worrying about locking.  because
	// we don't continue to hold the lock in the foreground goroutine,
	// this fanout instance may permit new events/registrations/inits/resets
	// while the old watchers are still being closed.  this is fine, since
	// the aformentioned move guarantees that these old watchers aren't
	// going to observe any of the new state transitions.
	go func() {
		for _, entries := range watchersToClose {
			for _, entry := range entries {
				entry.watcher.cancel()
			}
		}
	}()
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
	emux     sync.Mutex
	fanout   *Fanout
	err      error
	watch    types.Watch
	eventC   chan types.Event
	cancel   context.CancelFunc
	ctx      context.Context
	initOnce sync.Once
	initOk   bool

	bmu          sync.Mutex
	backlog      []types.Event
	backlogSince time.Time
}

// init transmits the OpInit event.  safe to double-call.
func (w *fanoutWatcher) init() (ok bool) {
	w.initOnce.Do(func() {
		select {
		case w.eventC <- types.Event{Type: types.OpInit}:
			w.initOk = true
		default:
			w.initOk = false
		}
	})
	return w.initOk
}

func (w *fanoutWatcher) emit(event types.Event) error {
	w.bmu.Lock()
	defer w.bmu.Unlock()

	if !w.flushBacklog() {
		if w.fanout.cfg.clock.Now().After(w.backlogSince.Add(w.fanout.cfg.gracePeriod)) {
			// backlog has existed for longer than grace period,
			// this watcher needs to be removed.
			return trace.BadParameter("buffer overflow")
		}
		// backlog exists, but we are still within grace period.
		w.backlog = append(w.backlog, event)
		return nil
	}

	select {
	case <-w.ctx.Done():
		return trace.Wrap(w.ctx.Err(), "watcher closed")
	case w.eventC <- event:
		return nil
	default:
		// primary event buffer is full; start backlog.
		w.backlog = append(w.backlog, event)
		w.backlogSince = w.fanout.cfg.clock.Now()
		return nil
	}
}

func (w *fanoutWatcher) backlogLen() int {
	w.bmu.Lock()
	defer w.bmu.Unlock()
	return len(w.backlog)
}

func (w *fanoutWatcher) Events() <-chan types.Event {
	w.bmu.Lock()
	defer w.bmu.Unlock()
	// it is possible that the channel has been drained, but events exist in
	// the backlog, so make sure to flush the backlog.  we can ignore the result
	// of the flush here since we don't actually care if the backlog was fully
	// flushed, only that the event channel is non-empty if a backlog does exist.
	w.flushBacklog()

	return w.eventC
}

// flushBacklog attempts to push any backlogged events into the
// event channel.  returns true if backlog is empty.
func (w *fanoutWatcher) flushBacklog() (ok bool) {
	for i, e := range w.backlog {
		select {
		case w.eventC <- e:
		default:
			w.backlog = w.backlog[i:]
			return false
		}
	}
	w.backlogSince = time.Time{}
	w.backlog = nil
	return true
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

// fanoutSetSize is the number of members in a fanout set.  selected based on some experimentation with
// the FanoutSetRegistration benchmark.  This value keeps 100K concurrent registrations well under 1s.
const fanoutSetSize = 128

// FanoutSet is a collection of separate Fanout instances. It exposes an identical API, and "load balances"
// watcher registration across the enclosed instances. In very large clusters it is possible for tens of
// thousands of nodes to simultaneously request watchers. This can cause serious contention issues. FanoutSet is
// a simple but effective solution to that problem.
type FanoutSet struct {
	// rw mutex is used to ensure that Close and Reset operations are exclusive,
	// since these operations close watchers. Enforcing this property isn't strictly
	// necessary, but it prevents a scenario where watchers might observe a reset/close,
	// attempt re-registration, and observe the *same* reset/close again. This isn't
	// necessarily a problem, but it might confuse attempts to debug other event-system
	// issues, so we choose to avoid it.
	rw      sync.RWMutex
	counter *atomic.Uint64
	members []*Fanout
}

// NewFanoutSet creates a new FanoutSet instance in an uninitialized
// state.  Until initialized, watchers will be queued but no
// events will be sent.
func NewFanoutSet() *FanoutSet {
	members := make([]*Fanout, 0, fanoutSetSize)
	for i := 0; i < fanoutSetSize; i++ {
		members = append(members, NewFanout())
	}
	return &FanoutSet{
		counter: atomic.NewUint64(0),
		members: members,
	}
}

// NewWatcher attaches a new watcher to a fanout instance.
func (s *FanoutSet) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	s.rw.RLock() // see field-level docks for locking model
	defer s.rw.RUnlock()
	fi := int(s.counter.Inc() % uint64(len(s.members)))
	return s.members[fi].NewWatcher(ctx, watch)
}

// SetInit sets the Fanout instances into an initialized state, sending OpInit
// events to any watchers which were added prior to initialization.
func (s *FanoutSet) SetInit() {
	s.rw.RLock() // see field-level docks for locking model
	defer s.rw.RUnlock()
	for _, f := range s.members {
		f.SetInit()
	}
}

// Emit broadcasts events to all matching watchers that have been attached
// to this fanout set.
func (s *FanoutSet) Emit(events ...types.Event) {
	s.rw.RLock() // see field-level docks for locking model
	defer s.rw.RUnlock()
	for _, f := range s.members {
		f.Emit(events...)
	}
}

// Reset closes all attached watchers and places the fanout instances
// into an uninitialized state.  Reset may be called on an uninitialized
// fanout set to remove "queued" watchers.
func (s *FanoutSet) Reset() {
	s.rw.Lock() // see field-level docks for locking model
	defer s.rw.Unlock()
	for _, f := range s.members {
		f.Reset()
	}
}

// Close permanently closes the fanout.  Existing watchers will be
// closed and no new watchers will be added.
func (s *FanoutSet) Close() {
	s.rw.Lock() // see field-level docks for locking model
	defer s.rw.Unlock()
	for _, f := range s.members {
		f.Close()
	}
}
