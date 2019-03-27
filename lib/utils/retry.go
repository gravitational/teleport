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
	"time"

	"github.com/gravitational/trace"
)

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
	closedChan := make(chan time.Time)
	close(closedChan)
	return &Linear{LinearConfig: cfg, closedChan: closedChan}, nil
}

// Linear is used to calculate retry period
// that follows the following logic:
// On the first error there is no delay
// on the next error, delay is FastLinear
// on all other errors, delay is SlowLinear
type Linear struct {
	// LinearConfig is a linear retry config
	LinearConfig
	attempt    int64
	closedChan chan time.Time
}

// Reset resetes retry period to initial state
func (r *Linear) Reset() {
	r.attempt = 0
}

// Inc increments attempt counter
func (r *Linear) Inc() {
	r.attempt++
}

// Duration returns retry duration based on state
func (r *Linear) Duration() time.Duration {
	a := r.First + time.Duration(r.attempt)*r.Step
	if a < 0 {
		return 0
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
	if r.Duration() == 0 {
		return r.closedChan
	}
	return time.After(r.Duration())
}

// String returns user-friendly representation of the LinearPeriod
func (r *Linear) String() string {
	return fmt.Sprintf("Linear(attempt=%v, duration=%v)", r.attempt, r.Duration())
}
