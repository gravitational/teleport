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
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// errorMessage is the error message to return when invalid input is provided by the caller.
const errorMessage = "special characters are not allowed in resource names, please use name composed only from characters, hyphens, dots, and plus signs: %q"

// allowPattern is the pattern of allowed characters for each key within
// the path.
var allowPattern = regexp.MustCompile(`^[0-9A-Za-z@_:.\-+]*$`)

// IsKeySafe checks if the passed in key conforms to whitelist
func IsKeySafe(key Key) bool {
	components := key.Components()
	for i, k := range components {
		switch k {
		case string(noEnd):
			continue
		case ".", "..":
			return false
		case "":
			return key.exactKey && i == len(components)-1
		}

		if strings.Contains(k, string(Separator)) {
			return false
		}

		if !allowPattern.MatchString(k) {
			return false
		}
	}
	return true
}

var _ Backend = (*Sanitizer)(nil)

// Sanitizer wraps a [Backend] implementation to make sure all
// [Key]s written to the backend are allowed. Retrieval and deletion
// of items do not perform any [Key] sanitization in order to allow
// interacting with any items that might already exist in the
// [Backend] prior to validation being performed on each
// subcomponent of a [Key] instead of on the entire [Key].
type Sanitizer struct {
	backend Backend
}

// NewSanitizer returns a new Sanitizer.
func NewSanitizer(backend Backend) *Sanitizer {
	return &Sanitizer{
		backend: backend,
	}
}

// GetRange returns query range
func (s *Sanitizer) GetRange(ctx context.Context, startKey, endKey Key, limit int) (*GetResult, error) {
	return s.backend.GetRange(ctx, startKey, endKey, limit)
}

// Create creates item if it does not exist
func (s *Sanitizer) Create(ctx context.Context, i Item) (*Lease, error) {
	if !IsKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage, i.Key)
	}
	return s.backend.Create(ctx, i)
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (s *Sanitizer) Put(ctx context.Context, i Item) (*Lease, error) {
	if !IsKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage, i.Key)
	}

	return s.backend.Put(ctx, i)
}

// Update updates value in the backend
func (s *Sanitizer) Update(ctx context.Context, i Item) (*Lease, error) {
	if !IsKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage, i.Key)
	}

	return s.backend.Update(ctx, i)
}

// ConditionalUpdate updates the value in the backend if the revision of the [Item] matches
// the stored revision.
func (s *Sanitizer) ConditionalUpdate(ctx context.Context, i Item) (*Lease, error) {
	if !IsKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage, i.Key)
	}

	return s.backend.ConditionalUpdate(ctx, i)
}

// Get returns a single item or not found error
func (s *Sanitizer) Get(ctx context.Context, key Key) (*Item, error) {
	return s.backend.Get(ctx, key)
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Sanitizer) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	if !IsKeySafe(expected.Key) {
		return nil, trace.BadParameter(errorMessage, expected.Key)
	}

	return s.backend.CompareAndSwap(ctx, expected, replaceWith)
}

// Delete deletes item by key
func (s *Sanitizer) Delete(ctx context.Context, key Key) error {
	return s.backend.Delete(ctx, key)
}

// ConditionalDelete deletes the item by key if the revision matches the stored revision.
func (s *Sanitizer) ConditionalDelete(ctx context.Context, key Key, revision string) error {
	return s.backend.ConditionalDelete(ctx, key, revision)
}

// DeleteRange deletes range of items
func (s *Sanitizer) DeleteRange(ctx context.Context, startKey, endKey Key) error {
	return s.backend.DeleteRange(ctx, startKey, endKey)
}

func (s *Sanitizer) AtomicWrite(ctx context.Context, condacts []ConditionalAction) (revision string, err error) {
	for _, ca := range condacts {
		if !IsKeySafe(ca.Key) {
			return "", trace.BadParameter(errorMessage, ca.Key)
		}
	}

	return s.backend.AtomicWrite(ctx, condacts)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (s *Sanitizer) KeepAlive(ctx context.Context, lease Lease, expires time.Time) error {
	if !IsKeySafe(lease.Key) {
		return trace.BadParameter(errorMessage, lease.Key)
	}
	return s.backend.KeepAlive(ctx, lease, expires)
}

// NewWatcher returns a new event watcher
func (s *Sanitizer) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	for _, prefix := range watch.Prefixes {
		if !IsKeySafe(prefix) {
			return nil, trace.BadParameter(errorMessage, prefix)
		}
	}
	return s.backend.NewWatcher(ctx, watch)
}

// Close releases the resources taken up by this backend
func (s *Sanitizer) Close() error {
	return s.backend.Close()
}

// Clock returns clock used by this backend
func (s *Sanitizer) Clock() clockwork.Clock {
	return s.backend.Clock()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (s *Sanitizer) CloseWatchers() {
	s.backend.CloseWatchers()
}

func (s *Sanitizer) GetName() string {
	return s.backend.GetName()
}
