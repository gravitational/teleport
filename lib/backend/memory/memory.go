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

package memory

import (
	"bytes"
	"context"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// GetName is a part of backend API and it returns in-memory backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return "in-memory"
}

const (
	// defaultBTreeDegree is a default degree of a B-Tree
	defaultBTreeDegree = 8
)

// Config holds configuration for the backend
type Config struct {
	// Context is a context for opening the
	// database
	Context context.Context
	// BTreeDegree is a degree of B-Tree, 2 for example, will create a
	// 2-3-4 tree (each node contains 1-3 items and 2-4 children).
	BTreeDegree int
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// Component is a logging component
	Component string
	// EventsOff turns off events generation
	EventsOff bool
	// BufferSize sets up event buffer size
	BufferSize int
	// Mirror mode is used when the memory backend is used for caching. In mirror
	// mode, revisions for Put requests are re-used (instead of
	// generating fresh ones) and expiration is turned off.
	Mirror bool
}

// CheckAndSetDefaults checks and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	if cfg.BTreeDegree <= 0 {
		cfg.BTreeDegree = defaultBTreeDegree
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentMemory
	}
	return nil
}

// New creates a new memory backend
func New(cfg Config) (*Memory, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)
	buf.SetInit()
	m := &Memory{
		Mutex:  &sync.Mutex{},
		logger: slog.With(teleport.ComponentKey, teleport.ComponentMemory),
		Config: cfg,
		tree: btree.NewG(cfg.BTreeDegree, func(a, b *btreeItem) bool {
			return a.Less(b)
		}),
		heap:   newMinHeap(),
		cancel: cancel,
		ctx:    ctx,
		buf:    buf,
	}
	return m, nil
}

// Memory is a memory B-Tree based backend
type Memory struct {
	*sync.Mutex
	logger *slog.Logger
	Config
	// tree is a BTree with items
	tree *btree.BTreeG[*btreeItem]
	// heap is a min heap with expiry records
	heap *minHeap
	// cancel is a function that cancels
	// all operations
	cancel context.CancelFunc
	// ctx is a context signaling close
	ctx context.Context
	buf *backend.CircularBuffer
}

func (m *Memory) GetName() string {
	return GetName()
}

// Close closes memory backend
func (m *Memory) Close() error {
	m.cancel()
	m.Lock()
	defer m.Unlock()
	m.buf.Close()
	return nil
}

// CloseWatchers closes all the watchers
// without closing the backend
func (m *Memory) CloseWatchers() {
	m.buf.Clear()
}

// Clock returns clock used by this backend
func (m *Memory) Clock() clockwork.Clock {
	return m.Config.Clock
}

// Create creates item if it does not exist
func (m *Memory) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if m.tree.Has(&btreeItem{Item: i}) {
		return nil, trace.AlreadyExists("key %q already exists", i.Key.String())
	}
	i.Revision = backend.CreateRevision()
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return backend.NewLease(i), nil
}

// Get returns a single item or not found error
func (m *Memory) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	if key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i, found := m.tree.Get(&btreeItem{Item: backend.Item{Key: key}})
	if !found {
		return nil, trace.NotFound("key %q is not found", key.String())
	}
	return &i.Item, nil
}

// Update updates item if it exists, or returns NotFound error
func (m *Memory) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if !m.tree.Has(&btreeItem{Item: i}) {
		return nil, trace.NotFound("key %q is not found", i.Key.String())
	}
	if !m.Mirror {
		i.Revision = backend.CreateRevision()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return backend.NewLease(i), nil
}

// Put puts value into backend (creates if it does not
// exist, updates it otherwise)
func (m *Memory) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if !m.Mirror {
		i.Revision = backend.CreateRevision()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return backend.NewLease(i), nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (m *Memory) Delete(ctx context.Context, key backend.Key) error {
	if key.IsZero() {
		return trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if !m.tree.Has(&btreeItem{Item: backend.Item{Key: key}}) {
		return trace.NotFound("key %q is not found", key.String())
	}
	event := backend.Event{
		Type: types.OpDelete,
		Item: backend.Item{
			Key: key,
		},
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return nil
}

// DeleteRange deletes range of items with keys between startKey and endKey
// Note that elements deleted by range do not produce any events
func (m *Memory) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	if startKey.IsZero() {
		return trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return trace.BadParameter("missing parameter endKey")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()

	var items []*btreeItem
	startItem := &btreeItem{Item: backend.Item{Key: startKey}}
	endItem := &btreeItem{Item: backend.Item{Key: endKey}}
	m.tree.AscendGreaterOrEqual(startItem, func(item *btreeItem) bool {
		if endItem.Less(item) {
			return false
		}
		items = append(items, item)
		return true
	})

	for _, item := range items {
		event := backend.Event{
			Type: types.OpDelete,
			Item: item.Item,
		}
		m.processEvent(event)
		if !m.EventsOff {
			m.buf.Emit(event)
		}
	}

	return nil
}

func (m *Memory) Iterate(ctx context.Context, startKey, endKey backend.Key, limit int, order backend.IterationOrder) (iter.Seq2[backend.Item, error], error) {
	if startKey.IsZero() {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return nil, trace.BadParameter("missing parameter endKey")
	}

	return func(yield func(backend.Item, error) bool) {
		iteratorFn := m.tree.AscendGreaterOrEqual
		if order == backend.IterateDescend {
			iteratorFn = m.tree.DescendGreaterThan
		}

		count := 0
		startItem := &btreeItem{Item: backend.Item{Key: startKey}}
		endItem := &btreeItem{Item: backend.Item{Key: endKey}}

		m.Lock()
		m.removeExpired()
		defer m.Unlock()
		iteratorFn(startItem, func(item *btreeItem) bool {
			if endItem.Less(item) {
				return false
			}

			if !yield(item.Item, nil) {
				return false
			}

			count++
			if limit != backend.NoLimit && count >= limit {
				return false
			}

			return true
		})
	}, nil
}

// GetRange returns query range
func (m *Memory) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	if startKey.IsZero() {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return nil, trace.BadParameter("missing parameter endKey")
	}

	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}

	iter, err := m.Iterate(ctx, startKey, endKey, limit, backend.IterateAscend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := backend.GetResult{Items: make([]backend.Item, 0, 1000)}
	for item, err := range iter {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.Items = append(result.Items, item)
	}

	if len(result.Items) == backend.DefaultRangeLimit {
		m.logger.WarnContext(ctx, "Range query hit backend limit. (this is a bug!)", "start_key", startKey, "limit", backend.DefaultRangeLimit)
	}
	return &result, nil
}

// KeepAlive updates TTL on the lease
func (m *Memory) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if lease.Key.IsZero() {
		return trace.BadParameter("missing parameter key")
	}

	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i, found := m.tree.Get(&btreeItem{Item: backend.Item{Key: lease.Key}})
	if !found {
		return trace.NotFound("key %q is not found", lease.Key.String())
	}
	item := i.Item
	item.Expires = expires
	if !m.Mirror {
		item.Revision = backend.CreateRevision()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: item,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return nil
}

// CompareAndSwap compares item with existing item and replaces it with replaceWith item
func (m *Memory) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if expected.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if replaceWith.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if expected.Key.Compare(replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i, found := m.tree.Get(&btreeItem{Item: expected})
	if !found {
		return nil, trace.CompareFailed("key %q is not found", expected.Key.String())
	}
	existingItem := i.Item
	if !bytes.Equal(existingItem.Value, expected.Value) {
		return nil, trace.CompareFailed("current value does not match expected for %v", expected.Key)
	}
	if !m.Mirror {
		replaceWith.Revision = backend.CreateRevision()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: replaceWith,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return backend.NewLease(replaceWith), nil
}

func (m *Memory) ConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	if key.IsZero() || (rev == "" && !m.Mirror) {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	m.Lock()
	defer m.Unlock()
	m.removeExpired()

	item, found := m.tree.Get(&btreeItem{Item: backend.Item{Key: key}})
	if !found || item.Item.Revision != rev {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	event := backend.Event{
		Type: types.OpDelete,
		Item: backend.Item{
			Key: key,
		},
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return nil
}

func (m *Memory) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() || (i.Revision == "" && !m.Mirror) {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	m.Lock()
	defer m.Unlock()
	m.removeExpired()

	item, found := m.tree.Get(&btreeItem{Item: i})
	if !found || item.Item.Revision != i.Revision {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	if !m.Mirror {
		i.Revision = backend.CreateRevision()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Emit(event)
	}
	return backend.NewLease(i), nil
}

// NewWatcher returns a new event watcher
func (m *Memory) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	if m.EventsOff {
		return nil, trace.BadParameter("events are turned off for this backend")
	}
	return m.buf.NewWatcher(ctx, watch)
}

// removeExpired makes a pass through map and removes expired elements
// returns the number of expired elements removed
func (m *Memory) removeExpired() int {
	// In mirror mode, don't expire any elements. This allows the cache to setup
	// a watch and expire elements as the events roll in.
	if m.Mirror {
		return 0
	}

	removed := 0
	now := m.Clock().Now().UTC()
	for {
		if len(*m.heap) == 0 {
			break
		}
		item := m.heap.PeekEl()
		if now.Before(item.Expires) {
			break
		}
		m.heap.PopEl()
		m.tree.Delete(item)
		m.logger.DebugContext(m.ctx, "Removed expired item.", "key", item.Key.String(), "expiry", item.Expires)
		removed++

		event := backend.Event{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: item.Key,
			},
		}
		if !m.EventsOff {
			m.buf.Emit(event)
		}
	}
	if removed > 0 {
		m.logger.DebugContext(m.ctx, "Removed expired items.", "num_expired", removed)
	}
	return removed
}

func (m *Memory) processEvent(event backend.Event) {
	switch event.Type {
	case types.OpPut:
		item := &btreeItem{Item: event.Item, index: -1}
		treeItem, found := m.tree.Get(item)
		var existingItem *btreeItem
		if found {
			existingItem = treeItem
		}

		// new item is added, but it has not expired yet
		if existingItem != nil && existingItem.index >= 0 {
			m.heap.RemoveEl(existingItem)
		}
		if !item.Expires.IsZero() {
			m.heap.PushEl(item)
		}
		m.tree.ReplaceOrInsert(item)
	case types.OpDelete:
		item, found := m.tree.Get(&btreeItem{Item: event.Item})
		if !found {
			return
		}

		m.tree.Delete(item)
		if item.index >= 0 {
			m.heap.RemoveEl(item)
		}
	default:
		// skip unsupported record
	}
}
