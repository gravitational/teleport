package backend

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

// errorMessage is the error message to return when invalid input is provided by the caller.
const errorMessage = "special characters are not allowed in resource names, please use name composed only from characters, hyphens and dots"

// whitelistPattern is the pattern of allowed characters for each key within
// the path.
var whitelistPattern = regexp.MustCompile(`^[0-9A-Za-z@_:.-]*$`)

// isStringSafe checks if the passed in string conforms to the whitelist.
func isStringSafe(s string) bool {
	if strings.Contains(s, "..") {
		return false
	}
	if strings.Contains(s, string(filepath.Separator)) {
		return false
	}

	return whitelistPattern.MatchString(s)
}

// isSliceSafe checks if the passed in slice conforms to the whitelist.
func isSliceSafe(slice []string) bool {
	for _, s := range slice {
		if !isStringSafe(s) {
			return false
		}
	}

	return true
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

// GetKeys returns a list of keys for a given path.
func (s *Sanitizer) GetKeys(bucket []string) ([]string, error) {
	if !isSliceSafe(bucket) {
		return nil, trace.BadParameter(errorMessage)
	}

	return s.backend.GetKeys(bucket)
}

// CreateVal creates value with a given TTL and key in the bucket. If the
// value already exists, returns trace.AlreadyExistsError.
func (s *Sanitizer) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	if !isSliceSafe(bucket) {
		return trace.BadParameter(errorMessage)
	}
	if !isStringSafe(key) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.CreateVal(bucket, key, val, ttl)
}

// UpsertVal updates or inserts value with a given TTL into a bucket. Use
// backend.ForeverTTL for no TTL.
func (s *Sanitizer) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	if !isSliceSafe(bucket) {
		return trace.BadParameter(errorMessage)
	}
	if !isStringSafe(key) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.UpsertVal(bucket, key, val, ttl)
}

// GetVal returns a value for a given key in the bucket.
func (s *Sanitizer) GetVal(bucket []string, key string) ([]byte, error) {
	if !isSliceSafe(bucket) {
		return nil, trace.BadParameter(errorMessage)
	}
	if !isStringSafe(key) {
		return nil, trace.BadParameter(errorMessage)
	}

	return s.backend.GetVal(bucket, key)
}

// CompareAndSwapVal compares and swaps values in atomic operation, succeeds
// if prevVal matches the value stored in the database, requires prevVal as a
// non-empty value. Returns trace.CompareFailed in case if value did not match.
func (s *Sanitizer) CompareAndSwapVal(bucket []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	if !isSliceSafe(bucket) {
		return trace.BadParameter(errorMessage)
	}
	if !isStringSafe(key) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.CompareAndSwapVal(bucket, key, val, prevVal, ttl)
}

// DeleteKey deletes a key in a bucket.
func (s *Sanitizer) DeleteKey(bucket []string, key string) error {
	if !isSliceSafe(bucket) {
		return trace.BadParameter(errorMessage)
	}
	if !isStringSafe(key) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.DeleteKey(bucket, key)
}

// DeleteBucket deletes the bucket by a given path.
func (s *Sanitizer) DeleteBucket(path []string, bucket string) error {
	if !isSliceSafe(path) {
		return trace.BadParameter(errorMessage)
	}
	if !isStringSafe(bucket) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.DeleteBucket(path, bucket)
}

// AcquireLock grabs a lock that will be released automatically after a TTL.
func (s *Sanitizer) AcquireLock(token string, ttl time.Duration) error {
	if !isStringSafe(token) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.AcquireLock(token, ttl)
}

// ReleaseLock forces lock release before the TTL has expired.
func (s *Sanitizer) ReleaseLock(token string) error {
	if !isStringSafe(token) {
		return trace.BadParameter(errorMessage)
	}

	return s.backend.ReleaseLock(token)
}

// Close releases the resources taken up by this backend
func (s *Sanitizer) Close() error {
	return s.backend.Close()
}

// Clock returns clock used by this backend
func (s *Sanitizer) Clock() clockwork.Clock {
	return s.backend.Clock()
}
