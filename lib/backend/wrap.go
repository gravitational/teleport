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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

var _ Backend = (*Wrapper)(nil)

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
func (s *Wrapper) GetRange(ctx context.Context, startKey, endKey Key, limit int) (*GetResult, error) {
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

// ConditionalUpdate updates value in the backend if revisions match.
func (s *Wrapper) ConditionalUpdate(ctx context.Context, i Item) (*Lease, error) {
	return s.backend.ConditionalUpdate(ctx, i)
}

// Get returns a single item or not found error
func (s *Wrapper) Get(ctx context.Context, key Key) (*Item, error) {
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
func (s *Wrapper) Delete(ctx context.Context, key Key) error {
	return s.backend.Delete(ctx, key)
}

// ConditionalDelete deletes item by key if revisions match.
func (s *Wrapper) ConditionalDelete(ctx context.Context, key Key, revision string) error {
	return s.backend.ConditionalDelete(ctx, key, revision)
}

// DeleteRange deletes range of items
func (s *Wrapper) DeleteRange(ctx context.Context, startKey, endKey Key) error {
	return s.backend.DeleteRange(ctx, startKey, endKey)
}

func (s *Wrapper) AtomicWrite(ctx context.Context, condacts []ConditionalAction) (revision string, err error) {
	return s.backend.AtomicWrite(ctx, condacts)
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
