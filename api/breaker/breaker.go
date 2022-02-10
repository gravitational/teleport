// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package breaker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc/grpclog"
)

// Metrics tallies success and failure counts
// for all executions performed by a CircuitBreaker
type Metrics struct {
	// Executions the total number of times the breaker has executed within the interval
	Executions uint32
	// Successes the number of successful executions
	Successes uint32
	// Failures the total number of failed executions
	Failures uint32
	// ConsecutiveSuccesses the number of consecutive successful executions
	ConsecutiveSuccesses uint32
	// ConsecutiveFailures the number of consecutive failed executions
	ConsecutiveFailures uint32
}

func (m *Metrics) String() string {
	return fmt.Sprintf("Metrics(executions=%d, successes=%d, failures=%d, consecutiveSuccesses=%d, CconsecutiveFailures=%d)", m.Executions, m.Successes, m.Failures, m.ConsecutiveSuccesses, m.ConsecutiveFailures)
}

// reset restores all counts to zero
func (m *Metrics) reset() {
	m.Executions = 0
	m.Successes = 0
	m.Failures = 0
	m.ConsecutiveSuccesses = 0
	m.ConsecutiveFailures = 0
}

// success increments the counters tracking successful executions
// and resets the ConsecutiveFailures count
func (m *Metrics) success() {
	m.Successes++
	m.ConsecutiveSuccesses++
	m.ConsecutiveFailures = 0
}

// failure increments the counters tracking failed executions
// and resets the ConsecutiveSuccesses count
func (m *Metrics) failure() {
	m.Failures++
	m.ConsecutiveFailures++
	m.ConsecutiveSuccesses = 0
}

// execute increments Executions
func (m *Metrics) execute() {
	m.Executions++
}

// State represents an operating state that a CircuitBreaker may be in.
type State int

const (
	// StateStandby indicates the breaker is passing all requests and watching stats
	StateStandby State = iota
	// StateTripped indicates too many errors have occurred and requests are actively being rejected
	StateTripped
	// StateRecovering indicates the breaker is allowing some requests to go through and rejecting others
	StateRecovering
)

// String returns the string representation of a State
func (s State) String() string {
	switch s {
	case StateStandby:
		return "standby"
	case StateTripped:
		return "tripped"
	case StateRecovering:
		return "recovering"
	default:
		return "undefined"
	}
}

var (
	// ErrStateTripped will be returned from executions performed while the CircuitBreaker
	// is in StateTripped
	ErrStateTripped = errors.New("breaker is tripped")
	// ErrLimitExceeded will be returned from executions performed while the CircuitBreaker
	// is in StateRecovering and the Config.RecoveryLimit is exceeded
	ErrLimitExceeded = errors.New("too many requests while breaker is recovering")
)

// Config contains configuration of the CircuitBreaker
type Config struct {
	// Clock is used to control time - mainly used for testing
	Clock clockwork.Clock
	// Interval is the
	Interval time.Duration
	// TrippedPeriod is the amount of time to remain in StateTripped before transitioning
	// into StateRecovering
	TrippedPeriod time.Duration
	// RecoveryRampPeriod is the ramp up time used in StateRecovering to slowly allow letting
	// new requests be processed
	RecoveryRampPeriod time.Duration
	// RecoveryLimit is the number on consecutive successful executions required to transition from
	// StateRecovering to StateStandby
	RecoveryLimit uint32
	// Trip specifies the TripFn that will be used to determine if the CircuitBreaker should transition from
	// StateStandby to StateTripped. This is required to be supplied, failure to do so will result in an error
	// creating the CircuitBreaker.
	Trip TripFn
	// OnTripped will be called when the CircuitBreaker enters the StateTripped state
	OnTripped func()
	// OnStandby will be called when the CircuitBreaker returns to the StateStandby state
	OnStandBy func()
	// IsSuccessful is used by the CircuitBreaker to determine if the executed function was successful or not
	IsSuccessful func(v interface{}, err error) bool
}

// TripFn determines if the CircuitBreaker should be tripped based
// on the state of the provided Metrics. A return value of true will
// cause the CircuitBreaker to transition into the StateTripped state
type TripFn = func(m Metrics) bool

// StaticTripper is a TripFn that always returns the provided value
// regardless of the Metrics. Useful for testing.
func StaticTripper(b bool) TripFn {
	return func(m Metrics) bool {
		return b
	}
}

// RatioTripper is a TripFn that returns true it the error ratio
// is greater than the provided ratio and there have been at least
// minExecutions performed.
func RatioTripper(ratio float32, minExecutions uint32) TripFn {
	return func(m Metrics) bool {
		if m.Executions < minExecutions {
			return false
		}

		r := float32(m.Failures) / float32(m.Executions)
		return r >= ratio
	}
}

// ConsecutiveFailureTripper is a TripFn that will return true if
// Metrics.ConsecutiveFailures is greater than the provided value.
func ConsecutiveFailureTripper(max uint32) TripFn {
	return func(m Metrics) bool {
		return m.ConsecutiveFailures > max
	}
}

// NonNilErrorIsSuccess returns true if the provided error is non nil. This
// is the default value for Config.IsSuccessful if not provided.
func NonNilErrorIsSuccess(_ interface{}, err error) bool {
	return err == nil
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Interval <= 0 {
		return trace.BadParameter("CircuitBreaker Interval must be set")
	}

	if c.Trip == nil {
		return trace.BadParameter("CircuitBreaker Trip must be set")
	}

	if c.TrippedPeriod <= 0 {
		c.TrippedPeriod = defaults.TrippedPeriod
	}

	if c.RecoveryRampPeriod <= 0 {
		c.RecoveryRampPeriod = defaults.RecoveryRampPeriod
	}

	if c.RecoveryLimit <= 0 {
		c.RecoveryLimit = defaults.RecoveryLimit
	}

	if c.OnTripped == nil {
		c.OnTripped = func() {}
	}

	if c.OnStandBy == nil {
		c.OnStandBy = func() {}
	}

	if c.IsSuccessful == nil {
		c.IsSuccessful = NonNilErrorIsSuccess
	}

	c.TrippedPeriod = utils.NewSeventhJitter()(c.TrippedPeriod)

	return nil
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	cfg Config

	rc *ratioController

	mu         sync.Mutex
	state      State
	generation uint64
	metrics    Metrics
	expiry     time.Time
}

// New returns a CircuitBreaker configured with the provided Config
func New(cfg Config) (*CircuitBreaker, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	cb := CircuitBreaker{cfg: cfg}
	cb.nextGeneration(cfg.Clock.Now())

	return &cb, nil
}

// Execute calls the provided function depending on the CircuitBreaker state.
// - StateStandby: all functions are executed.
// - StateTripped: no functions are executed and ErrStateTripped is returned.
// - StateRecovering: some functions are executed, some functions are not,
//                    when not executed ErrLimitExceeded is returned.
//
// The CircuitBreaker state is updated according to the outcome of executing the
// provided function and the current state. See package docs for a more detailed
// explanation of state transitions.
func (c *CircuitBreaker) Execute(f func() (interface{}, error)) (interface{}, error) {
	generation, err := c.beforeExecution()
	if err != nil {
		return nil, err
	}

	v, err := f()

	c.afterExecution(generation, v, err)

	return v, err
}

// beforeExecution checks the current state to determine if a new generation
// should be created and whether Execute is allowed to proceed.
func (c *CircuitBreaker) beforeExecution() (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.cfg.Clock.Now()

	generation, state := c.currentState(now)

	switch {
	case state == StateTripped:
		return generation, ErrStateTripped
	case state == StateRecovering && !c.rc.allowRequest():
		return generation, ErrLimitExceeded
	}

	c.metrics.execute()
	return generation, nil
}

// afterExecution updates the CircuitBreaker state based on the outcome of
// processing the fn in Execute.
func (c *CircuitBreaker) afterExecution(prior uint64, v interface{}, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.cfg.Clock.Now()

	generation, state := c.currentState(now)
	if generation != prior {
		return
	}

	if c.cfg.IsSuccessful(v, err) {
		grpclog.Infof("[breaker] successful execution, %s", c.metrics.String())
		c.success(state, now)
	} else {
		grpclog.Infof("[breaker] failed execution, %s", c.metrics.String())
		c.failure(state, now)
	}
}

// success tallies a successful execution and migrates to StateStandby
// if in another state and criteria has been met to transition
func (c *CircuitBreaker) success(state State, t time.Time) {
	switch state {
	case StateStandby:
		c.metrics.success()
	case StateRecovering:
		c.metrics.success()
		if c.metrics.ConsecutiveSuccesses >= c.cfg.RecoveryLimit {
			c.setState(StateStandby, t)
			go c.cfg.OnStandBy()
		}
	}
}

// failure tallies a failed execution and migrate to StateTripped
// if in another state and criteria has been met to transition
func (c *CircuitBreaker) failure(state State, t time.Time) {
	switch state {
	case StateRecovering:
		c.setState(StateTripped, t)
	case StateStandby:
		c.metrics.failure()

		if c.cfg.Trip(c.metrics) {
			c.setState(StateTripped, t)
			go c.cfg.OnTripped()
		}
	}
}

// setState updates the state and creates a new generation if the
// provided state is different from the CircuitBreakers current state
func (c *CircuitBreaker) setState(s State, t time.Time) {
	if c.state == s {
		return
	}

	grpclog.Infof("[breaker] state is now %s", s)

	if s == StateRecovering {
		c.rc = newRatioController(c.cfg.Clock, c.cfg.RecoveryRampPeriod)
	}

	c.state = s
	c.nextGeneration(t)
}

// currentState returns the state of the CircuitBreaker
func (c *CircuitBreaker) currentState(t time.Time) (uint64, State) {
	switch {
	case c.state == StateTripped && c.expiry.Before(t):
		c.setState(StateRecovering, t)
	case c.state == StateStandby && !c.expiry.IsZero() && c.expiry.Before(t):
		c.nextGeneration(t)
	}

	return c.generation, c.state
}

// nextGeneration creates a new generation and adjusts its expiration
// based on the current state
func (c *CircuitBreaker) nextGeneration(t time.Time) {
	c.metrics.reset()
	c.generation++

	switch c.state {
	case StateRecovering:
		c.expiry = time.Time{}
	case StateTripped:
		c.expiry = t.Add(c.cfg.TrippedPeriod)
	case StateStandby:
		c.expiry = t.Add(c.cfg.Interval)
	}
}
