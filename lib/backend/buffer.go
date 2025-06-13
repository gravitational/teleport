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

package backend

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	radix "github.com/armon/go-radix"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type bufferConfig struct {
	gracePeriod         time.Duration
	creationGracePeriod time.Duration
	capacity            int
	clock               clockwork.Clock
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

// CreationGracePeriod sets the amount of time delay after watcher creation before
// it will be considered for removal due to backlog.
func CreationGracePeriod(d time.Duration) BufferOption {
	return func(cfg *bufferConfig) {
		if d > 0 {
			cfg.creationGracePeriod = d
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

// CircularBuffer implements in-memory circular buffer
// of predefined size, that is capable of fan-out of the backend events.
type CircularBuffer struct {
	sync.Mutex
	logger       *slog.Logger
	cfg          bufferConfig
	init, closed bool
	watchers     *watcherTree
}

// NewCircularBuffer returns a new uninitialized instance of circular buffer.
func NewCircularBuffer(opts ...BufferOption) *CircularBuffer {
	cfg := bufferConfig{
		gracePeriod:         DefaultBacklogGracePeriod,
		creationGracePeriod: DefaultCreationGracePeriod,
		capacity:            DefaultBufferCapacity,
		clock:               clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &CircularBuffer{
		logger:   slog.With(teleport.ComponentKey, teleport.ComponentBuffer),
		cfg:      cfg,
		watchers: newWatcherTree(),
	}
}

// Clear clears all events from the queue and closes all active watchers,
// but does not modify init state.
func (c *CircularBuffer) Clear() {
	c.Lock()
	defer c.Unlock()
	c.clear()
}

// Reset is equivalent to Clear except that is also sets the buffer into
// an uninitialized state.  This method should only be used when resetting
// after a broken event stream.  If only closure of watchers is desired,
// use Clear instead.
func (c *CircularBuffer) Reset() {
	c.Lock()
	defer c.Unlock()
	c.clear()
	c.init = false
}

func (c *CircularBuffer) clear() {
	// could close multiple times
	c.watchers.walk(func(w *BufferWatcher) {
		w.closeWatcher()
	})
	c.watchers = newWatcherTree()
}

// SetInit puts the buffer into an initialized state if it isn't already.  Any watchers already queued
// will be sent init events, and watchers added after this call will have their init events sent immediately.
// This function must be called *after* establishing a healthy parent event stream in order to preserve
// correct cache behavior.
func (c *CircularBuffer) SetInit() {
	c.Lock()
	defer c.Unlock()
	if c.init {
		return
	}

	var watchersToDelete []*BufferWatcher
	c.watchers.walk(func(watcher *BufferWatcher) {
		if ok := watcher.init(); !ok {
			watchersToDelete = append(watchersToDelete, watcher)
		}
	})

	for _, watcher := range watchersToDelete {
		c.logger.WarnContext(context.Background(), "Closing watcher, failed to send init event.", "watcher", logutils.StringerAttr(watcher))
		watcher.closeWatcher()
		c.watchers.rm(watcher)
	}

	c.init = true
}

// Close closes circular buffer and all watchers
func (c *CircularBuffer) Close() error {
	c.Lock()
	defer c.Unlock()
	c.clear()
	c.closed = true
	// note that we do not modify init state here.  this is because
	// calls to Close are allowed to happen concurrently with calls
	// to Emit().
	return nil
}

// Emit emits events to currently registered watchers and stores them to
// the buffer.  Panics if called before SetInit(), and returns false if called
// after Close().
func (c *CircularBuffer) Emit(events ...Event) (ok bool) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return false
	}

	for i := range events {
		c.emit(events[i])
	}
	return true
}

func (c *CircularBuffer) emit(r Event) {
	if !c.init {
		panic("push called on uninitialized buffer instance")
	}
	c.fanOutEvent(r)
}

func (c *CircularBuffer) fanOutEvent(r Event) {
	var watchersToDelete []*BufferWatcher
	c.watchers.walkPath(r.Item.Key.String(), func(watcher *BufferWatcher) {
		if watcher.MetricComponent != "" {
			watcherQueues.WithLabelValues(watcher.MetricComponent).Set(float64(len(watcher.eventsC)))
		}
		if !watcher.emit(r) {
			watchersToDelete = append(watchersToDelete, watcher)
		}
	})

	for _, watcher := range watchersToDelete {
		c.logger.WarnContext(context.Background(), "Closing watcher, buffer overflow", "watcher", logutils.StringerAttr(watcher), "events", len(watcher.eventsC), "backlog", watcher.backlogLen())
		watcher.closeWatcher()
		c.watchers.rm(watcher)
	}
}

// RemoveRedundantPrefixes will remove redundant prefixes from the given prefix list.
func RemoveRedundantPrefixes(prefixes []Key) []Key {
	if len(prefixes) == 0 {
		return prefixes
	}
	// group adjacent prefixes together
	sort.Slice(prefixes, func(i, j int) bool {
		return prefixes[i].Compare(prefixes[j]) == -1
	})
	// j increments only for values with non-redundant prefixes
	j := 0
	for i := 1; i < len(prefixes); i++ {
		// skip keys that have first key as a prefix
		if prefixes[i].HasPrefix(prefixes[j]) {
			continue
		}
		j++
		// assign the first non-matching key to the j
		prefixes[j] = prefixes[i]
	}
	return prefixes[:j+1]
}

// NewWatcher adds a new watcher to the events buffer
func (c *CircularBuffer) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, trace.Errorf("cannot register watcher, buffer is closed")
	}

	if watch.QueueSize == 0 {
		watch.QueueSize = c.cfg.capacity
	}

	if len(watch.Prefixes) == 0 {
		// if watcher has no prefixes, assume it will match anything
		// starting from the separator (what includes all keys in backend invariant, see Keys function)
		watch.Prefixes = append(watch.Prefixes, Key{})
	} else {
		// if watcher's prefixes are redundant, keep only shorter prefixes
		// to avoid double fan out
		watch.Prefixes = RemoveRedundantPrefixes(watch.Prefixes)
	}

	closeCtx, cancel := context.WithCancel(ctx)
	w := &BufferWatcher{
		buffer:   c,
		Watch:    watch,
		eventsC:  make(chan Event, watch.QueueSize),
		created:  c.cfg.clock.Now(),
		ctx:      closeCtx,
		cancel:   cancel,
		capacity: watch.QueueSize,
	}
	c.logger.DebugContext(ctx, "Adding watcher", "watcher", logutils.StringerAttr(w))
	if c.init {
		if ok := w.init(); !ok {
			c.logger.WarnContext(ctx, "Closing watcher, failed to send init event.", "watcher", logutils.StringerAttr(w))
			return nil, trace.BadParameter("failed to send init event")
		}
	}
	c.watchers.add(w)
	return w, nil
}

func (c *CircularBuffer) removeWatcherWithLock(watcher *BufferWatcher) {
	ctx := context.Background()
	c.Lock()
	defer c.Unlock()
	if watcher == nil {
		c.logger.WarnContext(ctx, "Internal logic error, empty watcher")
		return
	}
	c.logger.DebugContext(ctx, "Removing watcher via external close.", "watcher", logutils.StringerAttr(watcher))
	found := c.watchers.rm(watcher)
	if !found {
		c.logger.DebugContext(ctx, "Could not find watcher", "watcher", watcher.Name)
	}
}

// BufferWatcher is a watcher connected to the
// buffer and receiving fan-out events from the watcher
type BufferWatcher struct {
	buffer *CircularBuffer
	Watch
	eventsC chan Event

	bmu          sync.Mutex
	backlog      []Event
	backlogSince time.Time
	created      time.Time

	ctx      context.Context
	cancel   context.CancelFunc
	initOnce sync.Once
	initOk   bool
	capacity int
}

// String returns user-friendly representation
// of the buffer watcher
func (w *BufferWatcher) String() string {
	return fmt.Sprintf("Watcher(name=%v, prefixes=%v, capacity=%v, size=%v)", w.Name, w.Prefixes, w.capacity, len(w.eventsC))
}

// Events returns events channel.  This method performs internal work and should be re-called after each event
// is received, rather than having its output cached.
func (w *BufferWatcher) Events() <-chan Event {
	w.bmu.Lock()
	defer w.bmu.Unlock()
	// it is possible that the channel has been drained, but events exist in
	// the backlog, so make sure to flush the backlog.  we can ignore the result
	// of the flush here since we don't actually care if the backlog was fully
	// flushed, only that the event channel is non-empty if a backlog does exist.
	w.flushBacklog()
	return w.eventsC
}

// Done channel is closed when watcher is closed
func (w *BufferWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// flushBacklog attempts to push any backlogged events into the
// event channel.  returns true if backlog is empty.
func (w *BufferWatcher) flushBacklog() (ok bool) {
	for i, e := range w.backlog {
		select {
		case w.eventsC <- e:
		default:
			w.backlog = w.backlog[i:]
			return false
		}
	}
	w.backlogSince = time.Time{}
	w.backlog = nil
	return true
}

// emit attempts to emit an event. Returns false if the watcher has
// exceeded the backlog grace period.
func (w *BufferWatcher) emit(e Event) (ok bool) {
	w.bmu.Lock()
	defer w.bmu.Unlock()

	if !w.flushBacklog() {
		if now := w.buffer.cfg.clock.Now(); now.After(w.backlogSince.Add(w.buffer.cfg.gracePeriod)) && now.After(w.created.Add(w.buffer.cfg.creationGracePeriod)) {
			// backlog has existed for longer than grace period,
			// this watcher needs to be removed.
			return false
		}
		// backlog exists, but we are still within grace period.
		w.backlog = append(w.backlog, e)
		return true
	}

	select {
	case w.eventsC <- e:
	default:
		// primary event buffer is full; start backlog.
		w.backlog = append(w.backlog, e)
		w.backlogSince = w.buffer.cfg.clock.Now()
	}
	return true
}

func (w *BufferWatcher) backlogLen() int {
	w.bmu.Lock()
	defer w.bmu.Unlock()
	return len(w.backlog)
}

// init transmits the OpInit event.  safe to double-call.
func (w *BufferWatcher) init() (ok bool) {
	w.initOnce.Do(func() {
		select {
		case w.eventsC <- Event{Type: types.OpInit}:
			w.initOk = true
		default:
			w.initOk = false
		}
	})
	return w.initOk
}

// Close closes the watcher, could
// be called multiple times, removes the watcher
// from the buffer queue
func (w *BufferWatcher) Close() error {
	w.closeAndRemove(removeAsync)
	return nil
}

// closeWatcher closes watcher
func (w *BufferWatcher) closeWatcher() {
	w.cancel()
}

const (
	removeSync  = true
	removeAsync = false
)

// closeAndRemove closes the watcher, could
// be called multiple times, removes the watcher
// from the buffer queue synchronously (used in tests)
// or asyncronously, used in prod, to avoid potential deadlocks
func (w *BufferWatcher) closeAndRemove(sync bool) {
	w.closeWatcher()
	if sync {
		w.buffer.removeWatcherWithLock(w)
	} else {
		go w.buffer.removeWatcherWithLock(w)
	}
}

func newWatcherTree() *watcherTree {
	return &watcherTree{
		Tree: radix.New(),
	}
}

type watcherTree struct {
	*radix.Tree
}

// add adds buffer watcher to the tree
func (t *watcherTree) add(w *BufferWatcher) {
	for _, p := range w.Prefixes {
		prefix := p.String()
		val, ok := t.Tree.Get(prefix)
		var watchers []*BufferWatcher
		if ok {
			watchers = val.([]*BufferWatcher)
		}
		watchers = append(watchers, w)
		t.Tree.Insert(prefix, watchers)
	}
}

// rm removes the buffer watcher from the prefix tree
func (t *watcherTree) rm(w *BufferWatcher) bool {
	if w == nil {
		return false
	}
	var found bool
	for _, p := range w.Prefixes {
		prefix := p.String()
		val, ok := t.Tree.Get(prefix)
		if !ok {
			continue
		}
		buffers := val.([]*BufferWatcher)
		prevLen := len(buffers)
		for i := range buffers {
			if buffers[i] == w {
				buffers = slices.Delete(buffers, i, i+1)
				found = true
				break
			}
		}
		if len(buffers) == 0 {
			t.Tree.Delete(prefix)
		} else if len(buffers) != prevLen {
			t.Tree.Insert(prefix, buffers)
		}
	}
	return found
}

// walkFn is a callback executed for every matching watcher
type walkFn func(w *BufferWatcher)

// walkPath walks the tree above the longest matching prefix
// and calls fn callback for every buffer watcher
func (t *watcherTree) walkPath(key string, fn walkFn) {
	t.Tree.WalkPath(key, func(prefix string, val any) bool {
		watchers := val.([]*BufferWatcher)
		for _, w := range watchers {
			fn(w)
		}
		return false
	})
}

// walk calls fn for every matching leaf of the tree
func (t *watcherTree) walk(fn walkFn) {
	t.Tree.Walk(func(prefix string, val any) bool {
		watchers := val.([]*BufferWatcher)
		for _, w := range watchers {
			fn(w)
		}
		return false
	})
}
