package utils

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// MinTTL finds min non 0 TTL duration,
// if both durations are 0, fails
func MinTTL(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// ToTTL converts expiration time to TTL duration
// relative to current time as provided by clock
func ToTTL(c clockwork.Clock, tm time.Time) time.Duration {
	now := c.Now().UTC()
	if tm.IsZero() || tm.Before(now) {
		return 0
	}
	return tm.Sub(now)
}

// TimeUnderLock is a time.Time protected by a sync.RWMutex.
type TimeUnderLock struct {
	sync.RWMutex
	t time.Time
}

// Get returns the stored time.Time value.
func (s *TimeUnderLock) Get() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.t
}

// SetIfZero updates the stored time.Time value if it is zero.
func (s *TimeUnderLock) SetIfZero(t time.Time) {
	s.Lock()
	defer s.Unlock()
	if s.t.IsZero() {
		s.t = t
	}
}

// Clear sets the stored time.Time value to zero.
func (s *TimeUnderLock) Clear() {
	s.Lock()
	defer s.Unlock()
	s.t = time.Time{}
}
