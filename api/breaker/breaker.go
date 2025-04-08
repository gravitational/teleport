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
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils/retryutils"
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
	return fmt.Sprintf("Metrics(executions=%d, successes=%d, failures=%d, consecutiveSuccesses=%d, consecutiveFailures=%d)", m.Executions, m.Successes, m.Failures, m.ConsecutiveSuccesses, m.ConsecutiveFailures)
}

// reset restores all counts to zero
func (m *Metrics) reset() {
	*m = Metrics{}
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
		return fmt.Sprintf("undefined(%v)", int(s))
	}
}

// ErrStateTripped will be returned from executions performed while the CircuitBreaker
// is in StateTripped
var ErrStateTripped = &trace.ConnectionProblemError{Message: "breaker is tripped"}

// Config contains configuration of the CircuitBreaker
type Config struct {
	// Clock is used to control time - mainly used for testing
	Clock clockwork.Clock
	// Interval is the period of time that execution metrics will be collected for within StateStandby before
	// transitioning to the next generation.
	Interval time.Duration
	// TrippedPeriod is the amount of time to remain in StateTripped before transitioning
	// into StateRecovering
	TrippedPeriod time.Duration
	// Recover specifies the TripFn that will be used to determine if the CircuitBreaker should transition from
	// StateRecovering to StateTripped. This is required to be supplied, failure to do so will result in an error
	// creating the CircuitBreaker.
	Recover TripFn
	// RecoveryLimit is the number on consecutive successful executions required to transition from
	// StateRecovering to StateStandby
	RecoveryLimit uint32
	// Trip specifies the TripFn that will be used to determine if the CircuitBreaker should transition from
	// StateStandby to StateTripped. This is required to be supplied, failure to do so will result in an error
	// creating the CircuitBreaker.
	Trip TripFn
	// OnTripped will be called when the CircuitBreaker enters the StateTripped
	// state; this callback is called while holding a lock, so it should return
	// quickly.
	OnTripped func()
	// OnStandby will be called when the CircuitBreaker returns to the
	// StateStandby state; this callback is called while holding a lock, so it
	// should return quickly.
	OnStandBy func()
	// OnExecute will be called once for each execution, and given the result
	// and the current state of the breaker state; this callback is called while
	// holding a lock, so it should return quickly.
	OnExecute func(success bool, state State)
	// IsSuccessful is used by the CircuitBreaker to determine if the executed function was successful or not
	IsSuccessful func(v interface{}, err error) bool
	// TrippedErrorMessage is an optional message to use as the error message when the CircuitBreaker
	// is tripped. Defaults to ErrStateTripped if not provided.
	TrippedErrorMessage string
}

// Clone returns a clone of the Config.
func (c *Config) Clone() Config {
	// the current Config can just be copied without issues
	return *c
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
func RatioTripper(ratio float64, minExecutions uint32) TripFn {
	return func(m Metrics) bool {
		if m.Executions < minExecutions {
			return false
		}

		r := float64(m.Failures) / float64(m.Executions)
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

// IsResponseSuccessful determines whether the error provided should be ignored by the circuit breaker. This checks
// for http status codes < 500 and a few unsuccessful gRPC status codes.
func IsResponseSuccessful(v interface{}, err error) bool {
	switch t := v.(type) {
	case nil:
		break
	case *http.Response:
		if t == nil {
			break
		}
		return t.StatusCode < http.StatusInternalServerError
	}

	code := status.Code(err)
	switch {
	case err == nil:
		return true
	case code == codes.Canceled || code == codes.Unknown || code == codes.Unavailable || code == codes.DeadlineExceeded:
		return false
	default:
		return true
	}
}

func DefaultBreakerConfig(clock clockwork.Clock) Config {
	return Config{
		Clock:        clock,
		Interval:     defaults.BreakerInterval,
		Trip:         RatioTripper(defaults.BreakerRatio, defaults.BreakerRatioMinExecutions),
		Recover:      RatioTripper(defaults.BreakerRatio/2, defaults.BreakerRatioMinExecutions/3),
		IsSuccessful: IsResponseSuccessful,
	}
}

func NoopBreakerConfig() Config {
	return Config{
		Interval:     defaults.BreakerInterval,
		Trip:         StaticTripper(false),
		Recover:      StaticTripper(false),
		IsSuccessful: func(v interface{}, err error) bool { return true },
	}
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
	if c.Recover == nil {
		return trace.BadParameter("CircuitBreaker Recover must be set")
	}

	if c.TrippedPeriod <= 0 {
		c.TrippedPeriod = defaults.TrippedPeriod
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

	if c.OnExecute == nil {
		c.OnExecute = func(bool, State) {}
	}

	if c.IsSuccessful == nil {
		c.IsSuccessful = NonNilErrorIsSuccess
	}

	c.TrippedPeriod = retryutils.NewSeventhJitter()(c.TrippedPeriod)

	return nil
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	cfg Config

	mu         sync.Mutex
	state      State
	generation uint64
	metrics    Metrics
	expiry     time.Time
}

func NewNoop() *CircuitBreaker {
	return &CircuitBreaker{
		cfg: NoopBreakerConfig(),
	}
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
//   - StateStandby: all functions are executed.
//   - StateTripped: no functions are executed and ErrStateTripped is returned.
//   - StateRecovering: some functions are executed, some functions are not,
//     when not executed ErrLimitExceeded is returned.
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

	if state == StateTripped {
		c.cfg.OnExecute(false, StateTripped)

		if c.cfg.TrippedErrorMessage != "" {
			return generation, trace.ConnectionProblem(nil, c.cfg.TrippedErrorMessage)
		}

		return generation, trace.Wrap(ErrStateTripped)
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
		c.successLocked(state, now)
	} else {
		c.failureLocked(state, now)
	}
}

// successLocked tallies a successful execution and migrates to StateStandby
// if in another state and criteria has been met to transition
func (c *CircuitBreaker) successLocked(state State, t time.Time) {
	switch state {
	case StateStandby:
		c.cfg.OnExecute(true, StateStandby)
		c.metrics.success()
	case StateRecovering:
		c.cfg.OnExecute(true, StateRecovering)
		c.metrics.success()
		if c.metrics.ConsecutiveSuccesses >= c.cfg.RecoveryLimit {
			c.setState(StateStandby, t)
			go c.cfg.OnStandBy()
		}
	}
}

// failureLocked tallies a failed execution and migrate to StateTripped
// if in another state and criteria has been met to transition
func (c *CircuitBreaker) failureLocked(state State, t time.Time) {
	c.metrics.failure()

	switch state {
	case StateRecovering:
		c.cfg.OnExecute(false, StateRecovering)
		if c.cfg.Recover(c.metrics) {
			c.setState(StateTripped, t)
		}
	case StateStandby:
		c.cfg.OnExecute(false, StateStandby)
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
