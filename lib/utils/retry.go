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
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

// Jitter is a function which applies random jitter to a
// duration.  Used to randomize backoff values.  Must be
// safe for concurrent usage.
type Jitter func(time.Duration) time.Duration

// NewJitter returns the default jitter (currently jitters on
// the range [n/2,n), but this is subject to change).
func NewJitter() Jitter {
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
	Jitter Jitter
	// AutoReset, if greater than zero, causes the linear retry to automatically
	// reset after Max * AutoReset has elapsed since the last call to Incr.
	AutoReset int64
}

// CheckAndSetDefaults checks and sets defaults
func (c *LinearConfig) CheckAndSetDefaults() error {
	if c.Step == 0 {
		return trace.BadParameter("missing parameter Step")
	}
	if c.Max == 0 {
		return trace.BadParameter("missing parameter Max")
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

// Reset resetes retry period to initial state
func (r *Linear) Reset() {
	r.attempt = 0
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
	r.lastIncr = time.Now()
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
	return time.After(d)
}

// String returns user-friendly representation of the LinearPeriod
func (r *Linear) String() string {
	return fmt.Sprintf("Linear(attempt=%v, duration=%v)", r.attempt, r.Duration())
}
