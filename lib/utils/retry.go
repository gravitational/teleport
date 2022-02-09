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

package utils

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// HalfJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls.
var HalfJitter = NewHalfJitter()

// Jitter is a function which applies random jitter to a
// duration.  Used to randomize backoff values.  Must be
// safe for concurrent usage.
type Jitter func(time.Duration) time.Duration

// NewJitter builds a new default jitter (currently jitters on
// the range [n/2,n), but this is subject to change).
func NewJitter() Jitter {
	return NewHalfJitter()
}

// NewHalfJitter returns a new jitter on the range [n/2,n).  This is
// a large range and most suitable for jittering things like backoff
// operations where breaking cycles quickly is a priority.
func NewHalfJitter() Jitter {
	var mu sync.Mutex
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(d time.Duration) time.Duration {
		// values less than 1 cause rng to panic, and some logic
		// relies on treating zero duration as non-blocking case.
		if d < 1 {
			return 0
		}
		mu.Lock()
		defer mu.Unlock()
		return (d / 2) + time.Duration(rng.Int63n(int64(d))/2)
	}
}

// NewSeventhJitter builds a new jitter on the range [6n/7,n). Prefer smaller
// jitters such as this when jittering periodic operations (e.g. cert rotation
// checks) since large jitters result in significantly increased load.
func NewSeventhJitter() Jitter {
	var mu sync.Mutex
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(d time.Duration) time.Duration {
		// values less than 1 cause rng to panic, and some logic
		// relies on treating zero duration as non-blocking case.
		if d < 1 {
			return 0
		}
		mu.Lock()
		defer mu.Unlock()
		return (6 * d / 7) + time.Duration(rng.Int63n(int64(d))/7)
	}
}

// Retry is an interface that provides retry logic
type Retry interface {
	// Reset resets retry state
	Reset()
	// Inc increments retry attempt
	Inc()
	// Duration returns retry duration,
	// could be 0
	Duration() time.Duration
	// After returns time.Time channel
	// that fires after Duration delay,
	// could fire right away if Duration is 0
	After() <-chan time.Time
	// Clone creates a copy of this retry in a
	// reset state.
	Clone() Retry
}

// LinearConfig sets up retry configuration
// using arithmetic progression
type LinearConfig struct {
	// First is a first element of the progression,
	// could be 0
	First time.Duration
	// Step is a step of the progression, can't be 0
	Step time.Duration
	// Max is a maximum value of the progression,
	// can't be 0
	Max time.Duration
	// Jitter is an optional jitter function to be applied
	// to the delay.  Note that supplying a jitter means that
	// successive calls to Duration may return different results.
	Jitter Jitter `json:"-"`
	// AutoReset, if greater than zero, causes the linear retry to automatically
	// reset after Max * AutoReset has elapsed since the last call to Incr.
	AutoReset int64
	// Clock to override clock in tests
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets defaults
func (c *LinearConfig) CheckAndSetDefaults() error {
	if c.Step == 0 {
		return trace.BadParameter("missing parameter Step")
	}
	if c.Max == 0 {
		return trace.BadParameter("missing parameter Max")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewLinear returns a new instance of linear retry
func NewLinear(cfg LinearConfig) (*Linear, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return newLinear(cfg), nil
}

// newLinear creates an instance of Linear from a
// previously verified configuration.
func newLinear(cfg LinearConfig) *Linear {
	closedChan := make(chan time.Time)
	close(closedChan)
	return &Linear{LinearConfig: cfg, closedChan: closedChan}
}

// NewConstant returns a new linear retry with constant interval.
func NewConstant(interval time.Duration) (*Linear, error) {
	return NewLinear(LinearConfig{Step: interval, Max: interval})
}

// Linear is used to calculate retry period
// that follows the following logic:
// On the first error there is no delay
// on the next error, delay is FastLinear
// on all other errors, delay is SlowLinear
type Linear struct {
	// LinearConfig is a linear retry config
	LinearConfig
	lastIncr   time.Time
	attempt    int64
	closedChan chan time.Time
}

// Reset resets retry period to initial state
func (r *Linear) Reset() {
	r.attempt = 0
}

// ResetToDelay resets retry period and increments the number of attempts.
func (r *Linear) ResetToDelay() {
	r.Reset()
	r.Inc()
}

// Clone creates an identical copy of Linear with fresh state.
func (r *Linear) Clone() Retry {
	return newLinear(r.LinearConfig)
}

// Inc increments attempt counter
func (r *Linear) Inc() {
	r.attempt++
	if r.AutoReset < 1 {
		// No AutoRest configured; we can skip
		// everything else.
		return
	}
	// when AutoReset is active, we track the time of the
	// last call to Incr.  If more than Max * AutoReset has
	// elapsed, we reset state internally.  This allows
	// Linear to function like as a long-lived rate-limiting
	// device.
	prev := r.lastIncr
	r.lastIncr = r.Clock.Now()
	if prev.IsZero() {
		return
	}
	if r.Max*time.Duration(r.AutoReset) < r.lastIncr.Sub(prev) {
		r.Reset()
	}
}

// Duration returns retry duration based on state
func (r *Linear) Duration() time.Duration {
	a := r.First + time.Duration(r.attempt)*r.Step
	if a < 1 {
		return 0
	}
	if r.Jitter != nil {
		a = r.Jitter(a)
	}
	if a <= r.Max {
		return a
	}
	if r.Jitter != nil {
		return r.Jitter(r.Max)
	}
	return r.Max
}

// After returns channel that fires with timeout
// defined in Duration method, as a special case
// if Duration is 0 returns a closed channel
func (r *Linear) After() <-chan time.Time {
	d := r.Duration()
	if d < 1 {
		return r.closedChan
	}
	return r.Clock.After(d)
}

// String returns user-friendly representation of the LinearPeriod
func (r *Linear) String() string {
	return fmt.Sprintf("Linear(attempt=%v, duration=%v)", r.attempt, r.Duration())
}

// For retries the provided function until it succeeds or the context expires.
func (r *Linear) For(ctx context.Context, retryFn func() error) error {
	for {
		err := retryFn()
		if err == nil {
			return nil
		}
		if _, ok := trace.Unwrap(err).(*permanentRetryError); ok {
			return trace.Wrap(err)
		}
		log.Debugf("Will retry in %v: %v.", r.Duration(), err)
		select {
		case <-r.After():
			r.Inc()
		case <-ctx.Done():
			return trace.LimitExceeded(ctx.Err().Error())
		}
	}
}

// PermanentRetryError returns a new instance of a permanent retry error.
func PermanentRetryError(err error) error {
	return &permanentRetryError{err: err}
}

// permanentRetryError indicates that retry loop should stop.
type permanentRetryError struct {
	err error
}

// Error returns the original error message.
func (e *permanentRetryError) Error() string {
	return e.err.Error()
}

// RetryFastFor retries a function repeatedly for a set amount of
// time before returning an error.
//
// Intended mostly for tests.
func RetryStaticFor(d time.Duration, w time.Duration, f func() error) error {
	start := time.Now()
	var err error

	for time.Since(start) < d {
		if err = f(); err == nil {
			break
		}

		time.Sleep(w)
	}

	return err
}
