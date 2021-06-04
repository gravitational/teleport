/*
Copyright 2019 Gravitational, Inc.

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

package memory

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// GetName is a part of backend API and it returns in-memory backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return "in-memory"
}

const (
	// defaultBTreeDegreee is a default degree of a B-Tree
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
	// mode, record IDs for Put and PutRange requests are re-used (instead of
	// generating fresh ones) and expiration is turned off.
	Mirror bool
}

// CheckAndSetDefaults checks and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferSize
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
	buf, err := backend.NewCircularBuffer(ctx, cfg.BufferSize)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	m := &Memory{
		Mutex: &sync.Mutex{},
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentMemory,
		}),
		Config: cfg,
		tree:   btree.New(cfg.BTreeDegree),
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
	*log.Entry
	Config
	backend.NoMigrations
	// tree is a BTree with items
	tree *btree.BTree
	// heap is a min heap with expiry records
	heap *minHeap
	// cancel is a function that cancels
	// all operations
	cancel context.CancelFunc
	// ctx is a context signalling close
	ctx context.Context
	buf *backend.CircularBuffer
	//  nextID is a next record ID
	nextID int64
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
	m.buf.Reset()
}

// Clock returns clock used by this backend
func (m *Memory) Clock() clockwork.Clock {
	return m.Config.Clock
}

// Create creates item if it does not exist
func (m *Memory) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if m.tree.Get(&btreeItem{Item: i}) != nil {
		return nil, trace.AlreadyExists("key %q already exists", string(i.Key))
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return m.newLease(i), nil
}

// Get returns a single item or not found error
func (m *Memory) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	if len(key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i := m.tree.Get(&btreeItem{Item: backend.Item{Key: key}})
	if i == nil {
		return nil, trace.NotFound("key %q is not found", string(key))
	}
	item := i.(*btreeItem).Item
	return &item, nil
}

// Update updates item if it exists, or returns NotFound error
func (m *Memory) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if m.tree.Get(&btreeItem{Item: i}) == nil {
		return nil, trace.NotFound("key %q is not found", string(i.Key))
	}
	if !m.Mirror {
		i.ID = m.generateID()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return m.newLease(i), nil
}

// Put puts value into backend (creates if it does not
// exist, updates it otherwise)
func (m *Memory) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if !m.Mirror {
		i.ID = m.generateID()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: i,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return m.newLease(i), nil
}

// PutRange puts range of items into backend (creates if items do not
// exist, updates it otherwise)
func (m *Memory) PutRange(ctx context.Context, items []backend.Item) error {
	for i := range items {
		if items[i].Key == nil {
			return trace.BadParameter("missing parameter key in item %v", i)
		}
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	for _, item := range items {
		event := backend.Event{
			Type: types.OpPut,
			Item: item,
		}
		if !m.Mirror {
			event.Item.ID = m.generateID()
		}
		m.processEvent(event)
		if !m.EventsOff {
			m.buf.Push(event)
		}
	}
	return nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (m *Memory) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	if m.tree.Get(&btreeItem{Item: backend.Item{Key: key}}) == nil {
		return trace.NotFound("key %q is not found", string(key))
	}
	event := backend.Event{
		Type: types.OpDelete,
		Item: backend.Item{
			Key: key,
		},
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return nil
}

// DeleteRange deletes range of items with keys between startKey and endKey
// Note that elements deleted by range do not produce any events
func (m *Memory) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	re := m.getRange(ctx, startKey, endKey, backend.NoLimit)
	for _, item := range re.Items {
		event := backend.Event{
			Type: types.OpDelete,
			Item: item,
		}
		m.processEvent(event)
		if !m.EventsOff {
			m.buf.Push(event)
		}
	}
	return nil
}

// GetRange returns query range
func (m *Memory) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultLargeLimit
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	re := m.getRange(ctx, startKey, endKey, limit)
	return &re, nil
}

// KeepAlive updates TTL on the lease
func (m *Memory) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if lease.IsEmpty() {
		return trace.BadParameter("lease is empty")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i := m.tree.Get(&btreeItem{Item: backend.Item{Key: lease.Key}})
	if i == nil {
		return trace.NotFound("key %q is not found", string(lease.Key))
	}
	item := i.(*btreeItem).Item
	item.Expires = expires
	if !m.Mirror {
		// ID is updated on keep alive for consistency with other backends
		item.ID = m.generateID()
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: item,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return nil
}

// CompareAndSwap compares item with existing item and replaces it with replaceWith item
func (m *Memory) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}
	m.Lock()
	defer m.Unlock()
	m.removeExpired()
	i := m.tree.Get(&btreeItem{Item: expected})
	if i == nil {
		return nil, trace.CompareFailed("key %q is not found", string(expected.Key))
	}
	existingItem := i.(*btreeItem).Item
	if !bytes.Equal(existingItem.Value, expected.Value) {
		return nil, trace.CompareFailed("current value does not match expected for %v", string(expected.Key))
	}
	event := backend.Event{
		Type: types.OpPut,
		Item: replaceWith,
	}
	m.processEvent(event)
	if !m.EventsOff {
		m.buf.Push(event)
	}
	return m.newLease(replaceWith), nil
}

// NewWatcher returns a new event watcher
func (m *Memory) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	if m.EventsOff {
		return nil, trace.BadParameter("events are turned off for this backend")
	}
	return m.buf.NewWatcher(ctx, watch)
}

func (m *Memory) generateID() int64 {
	return atomic.AddInt64(&m.nextID, 1)
}

func (m *Memory) getRange(ctx context.Context, startKey, endKey []byte, limit int) backend.GetResult {
	var res backend.GetResult
	m.tree.AscendRange(&btreeItem{Item: backend.Item{Key: startKey}}, &btreeItem{Item: backend.Item{Key: endKey}}, func(i btree.Item) bool {
		item := i.(*btreeItem)
		res.Items = append(res.Items, item.Item)
		if limit > 0 && len(res.Items) >= limit {
			return false
		}
		return true
	})
	return res
}

func (m *Memory) newLease(item backend.Item) *backend.Lease {
	var lease backend.Lease
	if item.Expires.IsZero() {
		return &lease
	}
	lease.Key = item.Key
	return &lease
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
		m.Debugf("Removed expired %v %v item.", string(item.Key), item.Expires)
		removed++

		event := backend.Event{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: item.Key,
			},
		}
		if !m.EventsOff {
			m.buf.Push(event)
		}
	}
	if removed > 0 {
		m.Debugf("Removed %v expired items.", removed)
	}
	return removed
}

func (m *Memory) processEvent(event backend.Event) {
	switch event.Type {
	case types.OpPut:
		item := &btreeItem{Item: event.Item, index: -1}
		treeItem := m.tree.Get(item)
		var existingItem *btreeItem
		if treeItem != nil {
			existingItem = treeItem.(*btreeItem)
		}
		switch {
		case item.Expires.IsZero():
			// new item is added, it does not expire,
			if existingItem != nil && existingItem.index >= 0 {
				// new item replaces the existing item that should be removed
				// from the heap
				m.heap.RemoveEl(existingItem)
			}
			m.tree.ReplaceOrInsert(item)
		case !item.Expires.IsZero() && m.Clock().Now().Before(item.Expires):
			// new item is added, but it has not expired yet
			if existingItem != nil && existingItem.index >= 0 {
				m.heap.RemoveEl(existingItem)
			}
			m.heap.PushEl(item)
			m.tree.ReplaceOrInsert(item)
		case !item.Expires.IsZero() && (m.Clock().Now().After(item.Expires) || m.Clock().Now() == item.Expires):
			// new expired item has added, remove the existing
			// item if present
			if existingItem != nil {
				// existing item should be removed from the heap
				if existingItem.index >= 0 {
					m.heap.RemoveEl(existingItem)
				}
				m.tree.Delete(existingItem)
			}
		default:
			// skip adding or updating the item that has expired
		}
	case types.OpDelete:
		treeItem := m.tree.Get(&btreeItem{Item: event.Item})
		if treeItem != nil {
			item := treeItem.(*btreeItem)
			m.tree.Delete(item)
			if item.index >= 0 {
				m.heap.RemoveEl(item)
			}
		}
	default:
		// skip unsupported record
	}
}
