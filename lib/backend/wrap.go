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

package backend

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Wrapper wraps a Backend implementation that can fail
// on demand.
type Wrapper struct {
	sync.RWMutex
	backend Backend
	// readErr if set, will result in an error returned
	// on every read operation
	readErr error
}

// NewWrapper returns a new Wrapper.
func NewWrapper(backend Backend) *Wrapper {
	return &Wrapper{
		backend: backend,
	}
}

func (s *Wrapper) GetName() string {
	return s.backend.GetName()
}

// GetReadError returns error to be returned by
// read backend operations
func (s *Wrapper) GetReadError() error {
	s.RLock()
	defer s.RUnlock()
	return s.readErr
}

// SetReadError sets error to be returned by read backend operations
func (s *Wrapper) SetReadError(err error) {
	s.Lock()
	defer s.Unlock()
	s.readErr = err
}

// GetRange returns query range
func (s *Wrapper) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	if err := s.GetReadError(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.GetRange(ctx, startKey, endKey, limit)
}

// Create creates item if it does not exist
func (s *Wrapper) Create(ctx context.Context, i Item) (*Lease, error) {
	return s.backend.Create(ctx, i)
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (s *Wrapper) Put(ctx context.Context, i Item) (*Lease, error) {
	return s.backend.Put(ctx, i)
}

// Update updates value in the backend
func (s *Wrapper) Update(ctx context.Context, i Item) (*Lease, error) {
	return s.backend.Update(ctx, i)
}

// Get returns a single item or not found error
func (s *Wrapper) Get(ctx context.Context, key []byte) (*Item, error) {
	if err := s.GetReadError(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.Get(ctx, key)
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Wrapper) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	return s.backend.CompareAndSwap(ctx, expected, replaceWith)
}

// Delete deletes item by key
func (s *Wrapper) Delete(ctx context.Context, key []byte) error {
	return s.backend.Delete(ctx, key)
}

// DeleteRange deletes range of items
func (s *Wrapper) DeleteRange(ctx context.Context, startKey []byte, endKey []byte) error {
	return s.backend.DeleteRange(ctx, startKey, endKey)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (s *Wrapper) KeepAlive(ctx context.Context, lease Lease, expires time.Time) error {
	return s.backend.KeepAlive(ctx, lease, expires)
}

// NewWatcher returns a new event watcher
func (s *Wrapper) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	if err := s.GetReadError(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.NewWatcher(ctx, watch)
}

// Close releases the resources taken up by this backend
func (s *Wrapper) Close() error {
	return s.backend.Close()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (s *Wrapper) CloseWatchers() {
	s.backend.CloseWatchers()
}

// Clock returns clock used by this backend
func (s *Wrapper) Clock() clockwork.Clock {
	return s.backend.Clock()
}
