/*
Copyright 2019-2022 Gravitational, Inc.

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

// Package retryutils defines common retry and jitter logic.
package retryutils

import (
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// maxBackoff is an absolute maximum amount of backoff that our backoff helpers will
// apply. Used as a safety precaution to limit the impact of misconfigured backoffs.
const maxBackoff = 16 * time.Minute

// maxAttempts is the peak attempt number we will scale to (used to prevent overflows).
const maxAttempts = 16

// statically assert that we don't overflow.
const _ = maxBackoff << (maxAttempts - 1)

// statically assert that RetryV2 implements the Retry interface.
var _ Retry = (*RetryV2)(nil)

// driver is the underlying retry driver. determines the difference in behavior between
// linear/exponential retries.
//
// NOTE: drivers must be stateless. If a stateful driver needs to be implemented in the
// future, this interface will need to be extended to support safe use of Retry.Clone.
type Driver interface {
	// Duration calculates the step-specific delay for a given attempt. Excludes
	// base duration and jitter, which are applied by the outer retry instance.
	Duration(attempt int64) time.Duration

	// Check verifies the correctness of any driver-internal parameters.
	Check() error
}

// NewConstantDriver creates a constant retry driver with the supplied step value. Resulting
// retries have always the same value of the provided step.
func NewConstantDriver(step time.Duration) Driver {
	return constantDriver{step}
}

type constantDriver struct {
	step time.Duration
}

func (d constantDriver) Duration(_ int64) time.Duration {
	return d.step
}

func (d constantDriver) Check() error {
	if d.step <= 0 {
		return trace.BadParameter("constant driver requires positive step value")
	}

	if d.step > maxBackoff {
		return trace.BadParameter("constant backoff step value too large: %v (max=%v)", d.step, maxBackoff)
	}
	return nil
}

// NewLinearDriver creates a linear retry driver with the supplied step value. Resulting retries
// have increase their backoff by a fixed step amount on each increment, with the first retry
// having a base step amount of zero.
func NewLinearDriver(step time.Duration) Driver {
	return linearDriver{step}
}

type linearDriver struct {
	step time.Duration
}

func (d linearDriver) Duration(attempt int64) time.Duration {
	dur := d.step * time.Duration(attempt)
	if dur > maxBackoff {
		return maxBackoff
	}
	return dur
}

func (d linearDriver) Check() error {
	if d.step <= 0 {
		return trace.BadParameter("linear driver requires positive step value")
	}

	if d.step > maxBackoff {
		return trace.BadParameter("linear backoff step value too large: %v (max=%v)", d.step, maxBackoff)
	}
	return nil
}

// NewExponentialDriver creates a new exponential retry driver with the supplied base
// step value. Resulting retries double their base backoff on each increment.
func NewExponentialDriver(base time.Duration) Driver {
	return exponentialDriver{base}
}

type exponentialDriver struct {
	base time.Duration
}

func (d exponentialDriver) Duration(attempt int64) time.Duration {
	if attempt > maxAttempts {
		// 16 will exceed any reasonable Max value already, and we don't
		// want to accidentally wrap and end up w/ negative durations.
		attempt = 16
	}

	// in order to maintain consistency with existing linear behavior, the first attempt
	// results in a base duration of 0.
	if attempt <= 0 {
		return 0
	}

	// duration calculated as step * the square of the attempt number
	dur := d.base << (attempt - 1)

	if dur > maxBackoff {
		return maxBackoff
	}

	return dur
}

func (d exponentialDriver) Check() error {
	if d.base <= 0 {
		return trace.BadParameter("exponential driver requires positive base")
	}

	if d.base > maxBackoff {
		return trace.BadParameter("exponential backoff base too large: %v (max=%v)", d.base, maxBackoff)
	}
	return nil
}

// RetryV2Config sets up retry configuration
// using arithmetic progression
type RetryV2Config struct {
	// First is a first element of the progression,
	// could be 0
	First time.Duration
	// Driver generates the underlying progression of delays. Cannot be nil.
	Driver Driver
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
func (c *RetryV2Config) CheckAndSetDefaults() error {
	if c.Driver == nil {
		return trace.BadParameter("missing parameter Driver")
	}
	if err := c.Driver.Check(); err != nil {
		return trace.Wrap(err)
	}
	if c.Max == 0 {
		return trace.BadParameter("missing parameter Max")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewRetryV2 returns a new retry instance.
func NewRetryV2(cfg RetryV2Config) (*RetryV2, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return newRetryV2(cfg), nil
}

// newRetryV2 creates an instance of RetryV2 from a
// previously verified configuration.
func newRetryV2(cfg RetryV2Config) *RetryV2 {
	return &RetryV2{RetryV2Config: cfg}
}

// RetryV2 is used to moderate the rate of retries by applying successively increasing
// delays. The nature of the progression is determined by the 'Driver', which generates
// the portion of the delay corresponding to the attempt number (e.g. Exponential(1s) might
// generate the sequence 0s, 1s, 2s, 4s, 8s, etc). This progression is can be modified through
// the use of a custom base/start value, jitters, etc.
type RetryV2 struct {
	// RetryV2Config is a linear retry config
	RetryV2Config
	lastUse time.Time
	attempt int64
}

// Reset resets retry period to initial state
func (r *RetryV2) Reset() {
	r.attempt = 0
}

// Clone creates an identical copy of RetryV2 with fresh state.
func (r *RetryV2) Clone() Retry {
	return newRetryV2(r.RetryV2Config)
}

// Inc increments attempt counter
func (r *RetryV2) Inc() {
	r.attempt++
}

// Duration returns retry duration based on state
func (r *RetryV2) Duration() time.Duration {
	if r.AutoReset > 0 {
		now := r.Clock.Now()
		if now.After(r.lastUse.Add(r.Max * time.Duration(r.AutoReset))) {
			r.Reset()
		}
		r.lastUse = now
	}

	a := r.First + r.Driver.Duration(r.attempt)
	if a < 1 {
		return 0
	}

	a = min(a, r.Max)

	if r.Jitter != nil {
		a = r.Jitter(a)
	}

	return a
}

// After returns channel that fires with timeout
// defined in Duration method.
func (r *RetryV2) After() <-chan time.Time {
	return r.Clock.After(r.Duration())
}
