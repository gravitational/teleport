// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

const (
	maxInstallBackoff = 3 * time.Hour
	minInstallBackoff = time.Minute
)

// installerBackoffEntry stores the completed installation state for a target.
type installerBackoffEntry[T any] struct {
	target T
	// issueType is the latest installation issue for this entry.
	issueType string
	// attempts is the count of installation attempts for this entry.
	attempts int32
	// lastAttemptAt is the time of the last attempt.
	lastAttemptAt time.Time
	// retryAfter is the time after which the installation can be retried.
	retryAfter time.Time
	// seenInLastScan indicates that the target was seen in the last discovery
	// scan. These are targets which were discovered and not already enrolled.
	seenInLastScan bool
	// retry tracks the attempts and calculates the retry backoff duration.
	retry retryutils.Retry
}

// retryable returns true if t is strictly after the entry's retry deadline.
// A target is still backed off when t equals retryAfter.
func (e *installerBackoffEntry[T]) retryable(t time.Time) bool {
	return t.After(e.retryAfter)
}

// isFailedAttempt reports whether the latest completed attempt failed.
func (e *installerBackoffEntry[T]) isFailedAttempt() bool {
	return e.issueType != ""
}

// installerBackoff tracks completed target installation attempts and backs off
// subsequent attempts for equivalent keys. The key function defines target
// identity and must return the same key for equivalent targets.
type installerBackoff[K comparable, T any] struct {
	retry *retryutils.RetryV2
	key   func(T) K

	mu      sync.Mutex
	entries map[K]*installerBackoffEntry[T]
}

// newInstallerBackoff creates an installer backoff. baseDelay is bounded to
// the supported installer backoff range. key must be non-nil and defines the
// identity used for all cache operations.
func newInstallerBackoff[K comparable, T any](baseDelay time.Duration, jitter retryutils.Jitter, key func(T) K) (*installerBackoff[K, T], error) {
	if key == nil {
		return nil, errors.New("key function is required")
	}
	// bound the base delay to [minInstallBackoff, maxInstallBackoff/4]
	baseDelay = min(
		max(baseDelay, minInstallBackoff),
		maxInstallBackoff/4,
	)
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(baseDelay),
		Max:    maxInstallBackoff,
		Jitter: jitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &installerBackoff[K, T]{
		retry:   retry,
		key:     key,
		entries: make(map[K]*installerBackoffEntry[T]),
	}, nil
}

// filter partitions targets into targets which may be attempted and completed
// entries which are still backed off. Callers must use the returned target
// slice in place of the input slice; filtering may modify the input's backing
// array and the returned slice may have a shorter length.
func (b *installerBackoff[K, T]) filter(targets []T, t time.Time) ([]T, []installerBackoffEntry[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var removed []installerBackoffEntry[T]
	targets = slices.DeleteFunc(targets, func(target T) bool {
		entry := b.addLocked(target)
		entry.seenInLastScan = true
		if entry.retryable(t) {
			return false
		}
		removed = append(removed, *entry)
		return true
	})
	return targets, removed
}

// addLocked returns the entry for target, creating one if necessary. The
// caller must hold b.mu.
func (b *installerBackoff[K, T]) addLocked(target T) *installerBackoffEntry[T] {
	key := b.key(target)
	entry := b.entries[key]
	if entry == nil {
		entry = &installerBackoffEntry[T]{
			retry: b.retry.Clone(),
		}
		b.entries[key] = entry
	}
	entry.target = target
	return entry
}

// recordAttemptLocked advances the completed-attempt state for target. The
// caller must hold b.mu.
func (b *installerBackoff[K, T]) recordAttemptLocked(target T, t time.Time) *installerBackoffEntry[T] {
	entry := b.addLocked(target)
	entry.retry.Inc()
	entry.attempts++
	entry.lastAttemptAt = t
	entry.retryAfter = t.Add(entry.retry.Duration())
	entry.seenInLastScan = true
	return entry
}

// recordSuccessfulAttempt records a completed successful installation and
// returns its updated backoff entry.
func (b *installerBackoff[K, T]) recordSuccessfulAttempt(target T, t time.Time) installerBackoffEntry[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.recordAttemptLocked(target, t)
	entry.issueType = ""
	return *entry
}

// recordFailedAttempt records a completed failed installation and returns its
// updated backoff entry.
func (b *installerBackoff[K, T]) recordFailedAttempt(target T, issueType string, t time.Time) installerBackoffEntry[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.recordAttemptLocked(target, t)
	entry.issueType = issueType
	return *entry
}

// expireEntries removes completed entries which were not seen in the last
// discovery scan once their backoff period has elapsed. Retained entries are
// marked unseen for the next scan.
func (b *installerBackoff[K, T]) expireEntries(t time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for key, entry := range b.entries {
		if !entry.seenInLastScan && entry.retryable(t) {
			delete(b.entries, key)
		} else {
			entry.seenInLastScan = false
		}
	}
}

// reset clears all completed backoff entries.
func (b *installerBackoff[K, T]) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make(map[K]*installerBackoffEntry[T])
}
