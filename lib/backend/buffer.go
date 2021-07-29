/*
Copyright 2018-2019 Gravitational, Inc.

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

package backend

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"

	radix "github.com/armon/go-radix"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// CircularBuffer implements in-memory circular buffer
// of predefined size, that is capable of fan-out of the backend events.
type CircularBuffer struct {
	sync.Mutex
	*log.Entry
	events       []Event
	start        int
	end          int
	size         int
	init, closed bool
	watchers     *watcherTree
}

// NewCircularBuffer returns a new uninitialized instance of circular buffer.
func NewCircularBuffer(size int) (*CircularBuffer, error) {
	if size <= 0 {
		return nil, trace.BadParameter("circular buffer size should be > 0")
	}
	buf := &CircularBuffer{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentBuffer,
		}),
		events:   make([]Event, size),
		start:    -1,
		end:      -1,
		size:     0,
		watchers: newWatcherTree(),
	}
	return buf, nil
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
	c.start = -1
	c.end = -1
	c.size = 0
	for i := 0; i < len(c.events); i++ {
		c.events[i] = Event{}
	}
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
		c.Warningf("Closing %v, failed to send init event.", watcher)
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

// Size returns circular buffer size
func (c *CircularBuffer) Size() int {
	return c.size
}

// Events returns a copy of records as arranged from start to end
func (c *CircularBuffer) Events() []Event {
	c.Lock()
	defer c.Unlock()
	return c.eventsCopy()
}

// eventsCopy returns a copy of events as arranged from start to end
func (c *CircularBuffer) eventsCopy() []Event {
	if c.size == 0 {
		return nil
	}
	var out []Event
	for i := 0; i < c.size; i++ {
		index := (c.start + i) % len(c.events)
		if out == nil {
			out = make([]Event, 0, c.size)
		}
		out = append(out, c.events[index])
	}
	return out
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
	if c.size == 0 {
		c.start = 0
		c.end = 0
		c.size = 1
	} else if c.size < len(c.events) {
		c.end = (c.end + 1) % len(c.events)
		c.events[c.end] = r
		c.size++
	} else {
		c.end = c.start
		c.start = (c.start + 1) % len(c.events)
	}
	c.events[c.end] = r
	c.fanOutEvent(r)
}

func (c *CircularBuffer) fanOutEvent(r Event) {
	var watchersToDelete []*BufferWatcher
	c.watchers.walkPath(string(r.Item.Key), func(watcher *BufferWatcher) {
		if watcher.MetricComponent != "" {
			watcherQueues.WithLabelValues(watcher.MetricComponent).Set(float64(len(watcher.eventsC)))
		}
		select {
		case watcher.eventsC <- r:
		default:
			watchersToDelete = append(watchersToDelete, watcher)
		}
	})

	for _, watcher := range watchersToDelete {
		c.Warningf("Closing %v, buffer overflow at %v elements.", watcher, len(watcher.eventsC))
		watcher.closeWatcher()
		c.watchers.rm(watcher)
	}
}

func removeRedundantPrefixes(prefixes [][]byte) [][]byte {
	if len(prefixes) == 0 {
		return prefixes
	}
	// group adjacent prefixes together
	sort.Slice(prefixes, func(i, j int) bool {
		return bytes.Compare(prefixes[i], prefixes[j]) == -1
	})
	// j increments only for values with non-redundant prefixes
	j := 0
	for i := 1; i < len(prefixes); i++ {
		// skip keys that have first key as a prefix
		if bytes.HasPrefix(prefixes[i], prefixes[j]) {
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
		watch.QueueSize = len(c.events)
	}

	if len(watch.Prefixes) == 0 {
		// if watcher has no prefixes, assume it will match anything
		// starting from the separator (what includes all keys in backend invariant, see Keys function)
		watch.Prefixes = append(watch.Prefixes, []byte{Separator})
	} else {
		// if watcher's prefixes are redundant, keep only shorter prefixes
		// to avoid double fan out
		watch.Prefixes = removeRedundantPrefixes(watch.Prefixes)
	}

	closeCtx, cancel := context.WithCancel(ctx)
	w := &BufferWatcher{
		buffer:   c,
		Watch:    watch,
		eventsC:  make(chan Event, watch.QueueSize),
		ctx:      closeCtx,
		cancel:   cancel,
		capacity: watch.QueueSize,
	}
	c.Debugf("Add %v.", w)
	if c.init {
		if ok := w.init(); !ok {
			c.Warningf("Closing %v, failed to send init event.", w)
			return nil, trace.BadParameter("failed to send init event")
		}
	}
	c.watchers.add(w)
	return w, nil
}

func (c *CircularBuffer) removeWatcherWithLock(watcher *BufferWatcher) {
	c.Lock()
	defer c.Unlock()
	if watcher == nil {
		c.Warningf("Internal logic error: %v.", trace.DebugReport(trace.BadParameter("empty watcher")))
		return
	}
	c.Debugf("Removed watcher %p via external close.", watcher)
	found := c.watchers.rm(watcher)
	if !found {
		c.Debugf("Could not find watcher %v.", watcher)
	}
}

// BufferWatcher is a watcher connected to the
// buffer and receiving fan-out events from the watcher
type BufferWatcher struct {
	buffer *CircularBuffer
	Watch
	eventsC  chan Event
	ctx      context.Context
	cancel   context.CancelFunc
	initOnce sync.Once
	initOk   bool
	capacity int
}

// String returns user-friendly representation
// of the buffer watcher
func (w *BufferWatcher) String() string {
	return fmt.Sprintf("Watcher(name=%v, prefixes=%v, capacity=%v, size=%v)", w.Name, string(bytes.Join(w.Prefixes, []byte(", "))), w.capacity, len(w.eventsC))
}

// Events returns events channel
func (w *BufferWatcher) Events() <-chan Event {
	return w.eventsC
}

// Done channel is closed when watcher is closed
func (w *BufferWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
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
		prefix := string(p)
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
		prefix := string(p)
		val, ok := t.Tree.Get(prefix)
		if !ok {
			continue
		}
		buffers := val.([]*BufferWatcher)
		prevLen := len(buffers)
		for i := range buffers {
			if buffers[i] == w {
				buffers = append(buffers[:i], buffers[i+1:]...)
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
	t.Tree.WalkPath(key, func(prefix string, val interface{}) bool {
		watchers := val.([]*BufferWatcher)
		for _, w := range watchers {
			fn(w)
		}
		return false
	})
}

// walk calls fn for every matching leaf of the tree
func (t *watcherTree) walk(fn walkFn) {
	t.Tree.Walk(func(prefix string, val interface{}) bool {
		watchers := val.([]*BufferWatcher)
		for _, w := range watchers {
			fn(w)
		}
		return false
	})
}
