/*
Copyright 2015-2019 Gravitational, Inc.

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

// Package backend provides storage backend abstraction layer
package backend

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/jonboulle/clockwork"
)

// Forever means that object TTL will not expire unless deleted
const (
	Forever time.Duration = 0
)

// Backend implements abstraction over local or remote storage backend.
// Item keys are assumed to be valid UTF8, which may be enforced by the
// various Backend implementations.
type Backend interface {
	// Create creates item if it does not exist
	Create(ctx context.Context, i Item) (*Lease, error)

	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i Item) (*Lease, error)

	// CompareAndSwap compares item with existing item
	// and replaces is with replaceWith item
	CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error)

	// Update updates value in the backend
	Update(ctx context.Context, i Item) (*Lease, error)

	// Get returns a single item or not found error
	Get(ctx context.Context, key []byte) (*Item, error)

	// GetRange returns query range
	GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error)

	// Delete deletes item by key, returns NotFound error
	// if item does not exist
	Delete(ctx context.Context, key []byte) error

	// DeleteRange deletes range of items with keys between startKey and endKey
	DeleteRange(ctx context.Context, startKey, endKey []byte) error

	// KeepAlive keeps object from expiring, updates lease on the existing object,
	// expires contains the new expiry to set on the lease,
	// some backends may ignore expires based on the implementation
	// in case if the lease managed server side
	KeepAlive(ctx context.Context, lease Lease, expires time.Time) error

	// NewWatcher returns a new event watcher
	NewWatcher(ctx context.Context, watch Watch) (Watcher, error)

	// Close closes backend and all associated resources
	Close() error

	// Clock returns clock used by this backend
	Clock() clockwork.Clock

	// CloseWatchers closes all the watchers
	// without closing the backend
	CloseWatchers()

	// Migrate performs any data migration necessary between Teleport versions.
	// Migrate must be called BEFORE using any other methods of the Backend.
	Migrate(context.Context) error
}

// Batch implements some batch methods
// that are not mandatory for all interfaces,
// only the ones used in bulk operations.
type Batch interface {
	// PutRange puts range of items in one transaction
	PutRange(ctx context.Context, items []Item) error
}

// Lease represents a lease on the item that can be used
// to extend item's TTL without updating its contents.
//
// Here is an example of renewing object TTL:
//
// lease, err := backend.Create()
// lease.Expires = time.Now().Add(time.Second)
// // Item TTL is extended
// err = backend.KeepAlive(lease)
//
type Lease struct {
	// Key is an object representing lease
	Key []byte
	// ID is a lease ID, could be empty
	ID int64
}

// IsEmpty returns true if the lease is empty value
func (l *Lease) IsEmpty() bool {
	return l.ID == 0 && len(l.Key) == 0
}

// Watch specifies watcher parameters
type Watch struct {
	// Name is a watch name set for debugging
	// purposes
	Name string
	// Prefixes specifies prefixes to watch,
	// passed to the backend implementation
	Prefixes [][]byte
	// QueueSize is an optional queue size
	QueueSize int
	// MetricComponent if set will start reporting
	// with a given component metric
	MetricComponent string
}

// String returns a user-friendly description
// of the watcher
func (w *Watch) String() string {
	return fmt.Sprintf("Watcher(name=%v, prefixes=%v)", w.Name, string(bytes.Join(w.Prefixes, []byte(", "))))
}

// Watcher returns watcher
type Watcher interface {
	// Events returns channel with events
	Events() <-chan Event

	// Done returns the channel signalling the closure
	Done() <-chan struct{}

	// Close closes the watcher and releases
	// all associated resources
	Close() error
}

// GetResult provides the result of GetRange request
type GetResult struct {
	// Items returns a list of items
	Items []Item
}

// Event is a event containing operation with item
type Event struct {
	// Type is operation type
	Type types.OpType
	// Item is event Item
	Item Item
}

// Item is a key value item
type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value is a value of the key value item
	Value []byte
	// Expires is an optional record expiry time
	Expires time.Time
	// ID is a record ID, newer records have newer ids
	ID int64
	// LeaseID is a lease ID, could be set on objects
	// with TTL
	LeaseID int64
}

// Config is used for 'storage' config section. It's a combination of
// values for various backends: 'boltdb', 'etcd', 'filesystem' and 'dynamodb'
type Config struct {
	// Type can be "bolt" or "etcd" or "dynamodb"
	Type string `yaml:"type,omitempty"`

	// Params is a generic key/value property bag which allows arbitrary
	// falues to be passed to backend
	Params Params `yaml:",inline"`
}

// Params type defines a flexible unified back-end configuration API.
// It is just a map of key/value pairs which gets populated by `storage` section
// in Teleport YAML config.
type Params map[string]interface{}

// GetString returns a string value stored in Params map, or an empty string
// if nothing is found
func (p Params) GetString(key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// NoLimit specifies no limits
const NoLimit = 0

// RangeEnd returns end of the range for given key
func RangeEnd(key []byte) []byte {
	end := make([]byte, len(key))
	copy(end, key)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i] = end[i] + 1
			end = end[:i+1]
			return end
		}
	}
	// next key does not exist (e.g., 0xffff);
	return noEnd
}

var (
	noEnd = []byte{0}
)

// Items is a sortable list of backend items
type Items []Item

// Len is part of sort.Interface.
func (it Items) Len() int {
	return len(it)
}

// Swap is part of sort.Interface.
func (it Items) Swap(i, j int) {
	it[i], it[j] = it[j], it[i]
}

// Less is part of sort.Interface.
func (it Items) Less(i, j int) bool {
	return bytes.Compare(it[i].Key, it[j].Key) < 0
}

// TTL returns TTL in duration units, rounds up to one second
func TTL(clock clockwork.Clock, expires time.Time) time.Duration {
	ttl := expires.Sub(clock.Now())
	if ttl < time.Second {
		return time.Second
	}
	return ttl
}

// EarliestExpiry returns first of the
// otherwise returns empty
func EarliestExpiry(times ...time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	sort.Sort(earliest(times))
	return times[0]
}

// Expiry converts ttl to expiry time, if ttl is 0
// returns empty time
func Expiry(clock clockwork.Clock, ttl time.Duration) time.Time {
	if ttl == 0 {
		return time.Time{}
	}
	return clock.Now().UTC().Add(ttl)
}

type earliest []time.Time

func (p earliest) Len() int {
	return len(p)
}

func (p earliest) Less(i, j int) bool {
	if p[i].IsZero() {
		return false
	}
	if p[j].IsZero() {
		return true
	}
	return p[i].Before(p[j])
}

func (p earliest) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Separator is used as a separator between key parts
const Separator = '/'

// Key joins parts into path separated by Separator,
// makes sure path always starts with Separator ("/")
func Key(parts ...string) []byte {
	return []byte(strings.Join(append([]string{""}, parts...), string(Separator)))
}

// NoMigrations implements a nop Migrate method of Backend.
// Backend implementations should embed this when no migrations are necessary.
type NoMigrations struct{}

func (NoMigrations) Migrate(context.Context) error { return nil }
