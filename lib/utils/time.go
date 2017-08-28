package utils

import (
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

// UTC converts time to UTC timezone
func UTC(t *time.Time) {
	if t == nil {
		return
	}

	if t.IsZero() {
		// to fix issue with timezones for tests
		*t = time.Time{}
		return
	}
	*t = t.UTC()
}
