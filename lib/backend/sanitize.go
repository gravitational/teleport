package backend

import (
	"context"
	"regexp"
	"time"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

// errorMessage is the error message to return when invalid input is provided by the caller.
const errorMessage = "special characters are not allowed in resource names, please use name composed only from characters, hyphens and dots"

// whitelistPattern is the pattern of allowed characters for each key within
// the path.
var whitelistPattern = regexp.MustCompile(`^[0-9A-Za-z@_:.\-/]*$`)

// isKeySafe checks if the passed in key conforms to whitelist
func isKeySafe(s []byte) bool {
	return whitelistPattern.Match(s)
}

// Sanitizer wraps a Backend implementation to make sure all values requested
// of the backend are whitelisted.
type Sanitizer struct {
	backend Backend
}

// NewSanitizer returns a new Sanitizer.
func NewSanitizer(backend Backend) *Sanitizer {
	return &Sanitizer{
		backend: backend,
	}
}

// Backend returns the underlying backend. Useful when knowing the type of
// backend is important (for example, can the backend support forking).
func (s *Sanitizer) Backend() Backend {
	return s.backend
}

// GetRange returns query range
func (s *Sanitizer) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	if !isKeySafe(startKey) {
		return nil, trace.BadParameter(errorMessage)
	}
	return s.backend.GetRange(ctx, startKey, endKey, limit)
}

// Create creates item if it does not exist
func (s *Sanitizer) Create(ctx context.Context, i Item) (*Lease, error) {
	if !isKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage)
	}
	return s.backend.Create(ctx, i)
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (s *Sanitizer) Put(ctx context.Context, i Item) (*Lease, error) {
	if !isKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage)
	}

	return s.backend.Put(ctx, i)
}

// Update updates value in the backend
func (s *Sanitizer) Update(ctx context.Context, i Item) (*Lease, error) {
	if !isKeySafe(i.Key) {
		return nil, trace.BadParameter(errorMessage)
	}

	return s.backend.Update(ctx, i)
}

// Get returns a single item or not found error
func (s *Sanitizer) Get(ctx context.Context, key []byte) (*Item, error) {
	if !isKeySafe(key) {
		return nil, trace.BadParameter(errorMessage)
	}
	return s.backend.Get(ctx, key)
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Sanitizer) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	if !isKeySafe(expected.Key) {
		return nil, trace.BadParameter(errorMessage)
	}

	return s.backend.CompareAndSwap(ctx, expected, replaceWith)
}

// Delete deletes item by key
func (s *Sanitizer) Delete(ctx context.Context, key []byte) error {
	if !isKeySafe(key) {
		return trace.BadParameter(errorMessage)
	}
	return s.backend.Delete(ctx, key)
}

// DeleteRange deletes range of items
func (s *Sanitizer) DeleteRange(ctx context.Context, startKey []byte, endKey []byte) error {
	if !isKeySafe(startKey) {
		return trace.BadParameter(errorMessage)
	}
	if !isKeySafe(endKey) {
		return trace.BadParameter(errorMessage)
	}
	return s.backend.Delete(ctx, endKey)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (s *Sanitizer) KeepAlive(ctx context.Context, lease Lease, expires time.Time) error {
	if !isKeySafe(lease.Key) {
		return trace.BadParameter(errorMessage)
	}
	return s.backend.KeepAlive(ctx, lease, expires)
}

// NewWatcher returns a new event watcher
func (s *Sanitizer) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	if !isKeySafe(watch.Prefix) {
		return nil, trace.BadParameter(errorMessage)
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
