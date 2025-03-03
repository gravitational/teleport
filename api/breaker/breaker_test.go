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
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCircuitBreaker_generation(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	cb, err := New(Config{
		Clock:    clock,
		Interval: time.Second,
		Trip:     StaticTripper(false),
		Recover:  StaticTripper(false),
	})
	require.NoError(t, err)

	generation, state := cb.currentState(clock.Now())
	require.Equal(t, uint64(1), generation)
	require.Equal(t, StateStandby, state)
	require.Equal(t, clock.Now().Add(time.Second), cb.expiry)

	clock.Advance(500 * time.Millisecond)
	generation, state = cb.currentState(clock.Now())
	require.Equal(t, uint64(1), generation)
	require.Equal(t, StateStandby, state)
	clock.Advance(501 * time.Millisecond)
	generation, state = cb.currentState(clock.Now())
	require.Equal(t, uint64(2), generation)
	require.Equal(t, StateStandby, state)
	require.Equal(t, clock.Now().Add(time.Second), cb.expiry)

	for i := 0; i < 1000; i++ {
		prevGeneration, prevState := cb.currentState(clock.Now())
		cb.nextGeneration(clock.Now())
		generation, state := cb.currentState(clock.Now())
		require.NotEqual(t, prevGeneration, generation)
		require.Equal(t, prevState, state)
	}

	generation, state = cb.currentState(clock.Now())
	require.Equal(t, uint64(1002), generation)
	require.Equal(t, StateStandby, state)
}

func TestCircuitBreaker_beforeRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc       string
		generation uint64
		executions uint32
		advance    time.Duration
		state      State
		errorCheck require.ErrorAssertionFunc
	}{
		{
			desc:       "standby allows execution",
			generation: 1,
			executions: 1,
			state:      StateStandby,
			errorCheck: require.NoError,
		},
		{
			desc:       "tripped prevents executions",
			generation: 1,
			executions: 0,
			state:      StateTripped,
			errorCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, ErrStateTripped, err)
			},
		},
		{
			desc:       "recovering after allows executions",
			generation: 1,
			executions: 1,
			state:      StateRecovering,
			advance:    3 * time.Second,
			errorCheck: require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			clock := clockwork.NewFakeClock()

			cb, err := New(Config{
				Clock:         clock,
				Interval:      time.Second,
				Trip:          StaticTripper(false),
				Recover:       StaticTripper(false),
				RecoveryLimit: 1,
			})
			require.NoError(t, err)
			cb.state = tt.state

			clock.Advance(tt.advance)

			generation, err := cb.beforeExecution()
			tt.errorCheck(t, err)
			require.Equal(t, tt.generation, generation)
			require.Equal(t, tt.executions, cb.metrics.Executions)

		})
	}
}

func TestCircuitBreaker_afterExecution(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc            string
		err             error
		priorGeneration uint64
		checkMetrics    require.ValueAssertionFunc
		trip            TripFn
		recover         TripFn
		expectedState   State
	}{
		{
			desc:            "successful execution",
			priorGeneration: 1,
			checkMetrics: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				m, ok := i.(Metrics)
				require.True(t, ok)
				require.Equal(t, uint32(1), m.Successes)
				require.Equal(t, uint32(0), m.Failures)
			},
			trip:          StaticTripper(false),
			recover:       StaticTripper(false),
			expectedState: StateStandby,
		},
		{
			desc:            "generation change",
			priorGeneration: 0,
			trip:            StaticTripper(false),
			recover:         StaticTripper(false),
			checkMetrics: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				m, ok := i.(Metrics)
				require.True(t, ok)
				require.Equal(t, uint32(0), m.Successes)
				require.Equal(t, uint32(0), m.Failures)
			},
			expectedState: StateStandby,
		},
		{
			desc:            "failed execution with out tripping",
			priorGeneration: 1,
			err:             errors.New("failure"),
			trip:            StaticTripper(false),
			recover:         StaticTripper(false),
			checkMetrics: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				m, ok := i.(Metrics)
				require.True(t, ok)
				require.Equal(t, uint32(0), m.Successes)
				require.Equal(t, uint32(1), m.Failures)
			},
			expectedState: StateStandby,
		},
		{
			desc:            "failed execution causing a trip",
			priorGeneration: 1,
			err:             errors.New("failure"),
			trip:            StaticTripper(true),
			recover:         StaticTripper(false),
			checkMetrics: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				m, ok := i.(Metrics)
				require.True(t, ok)
				require.Equal(t, uint32(0), m.Successes)
				require.Equal(t, uint32(0), m.Failures)
			},
			expectedState: StateTripped,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			cb, err := New(Config{
				Clock:    clock,
				Interval: time.Second,
				Trip:     tt.trip,
				Recover:  tt.recover,
			})
			require.NoError(t, err)

			cb.afterExecution(tt.priorGeneration, nil, tt.err)
			tt.checkMetrics(t, cb.metrics)
			require.Equal(t, tt.expectedState, cb.state)
		})
	}
}

func TestCircuitBreaker_success(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc          string
		initialState  State
		successState  State
		expectedState State
		recoveryLimit uint32
	}{
		{
			desc:          "success in standby",
			initialState:  StateStandby,
			successState:  StateStandby,
			expectedState: StateStandby,
		},
		{
			desc:          "success in recovery below limit",
			initialState:  StateRecovering,
			successState:  StateRecovering,
			expectedState: StateRecovering,
			recoveryLimit: 2,
		},
		{
			desc:          "success in recovery above limit",
			initialState:  StateRecovering,
			successState:  StateRecovering,
			expectedState: StateStandby,
			recoveryLimit: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			cb, err := New(Config{
				Clock:         clock,
				Interval:      time.Second,
				RecoveryLimit: tt.recoveryLimit,
				Trip:          StaticTripper(false),
				Recover:       StaticTripper(false),
			})
			require.NoError(t, err)
			cb.state = tt.initialState

			generation, state := cb.currentState(clock.Now())
			cb.successLocked(tt.successState, clock.Now())
			require.Equal(t, tt.expectedState, cb.state)
			if tt.expectedState != state {
				require.NotEqual(t, generation, cb.generation)
			}
		})
	}
}

func TestCircuitBreaker_failure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc           string
		initialState   State
		failureState   State
		expectedState  State
		tripFn         TripFn
		recover        TripFn
		onTrip         func(ch chan bool) func()
		tripped        bool
		requireTripped require.BoolAssertionFunc
	}{
		{
			desc:           "failure in recovering transitions to tripped",
			initialState:   StateRecovering,
			failureState:   StateRecovering,
			expectedState:  StateTripped,
			tripFn:         StaticTripper(false),
			recover:        StaticTripper(true),
			requireTripped: require.False,
		},
		{
			desc:           "failure in standby without tripping",
			initialState:   StateStandby,
			failureState:   StateStandby,
			expectedState:  StateStandby,
			tripFn:         StaticTripper(false),
			recover:        StaticTripper(false),
			requireTripped: require.False,
		},
		{
			desc:           "failure in standby causes tripping",
			initialState:   StateStandby,
			failureState:   StateStandby,
			expectedState:  StateTripped,
			tripFn:         StaticTripper(true),
			recover:        StaticTripper(false),
			requireTripped: require.True,
			onTrip: func(ch chan bool) func() {
				return func() {
					ch <- true
				}
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			clock := clockwork.NewFakeClock()

			if tt.onTrip == nil {
				tt.onTrip = func(ch chan bool) func() {
					ch <- false
					return func() {}
				}
			}

			trippedCh := make(chan bool, 1)

			cb, err := New(Config{
				Clock:     clock,
				Interval:  time.Second,
				Trip:      tt.tripFn,
				OnTripped: tt.onTrip(trippedCh),
				Recover:   tt.recover,
			})
			require.NoError(t, err)
			cb.state = tt.initialState

			generation, state := cb.currentState(clock.Now())
			cb.failureLocked(tt.failureState, clock.Now())
			require.Equal(t, tt.expectedState, cb.state)
			if tt.expectedState != state {
				require.NotEqual(t, generation, cb.generation)
			}

			tripped := <-trippedCh

			tt.requireTripped(t, tripped)
		})
	}
}

func TestCircuitBreaker_Execute(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	trippedCh := make(chan struct{})
	onTripped := func(ch chan struct{}) func() {
		return func() {
			ch <- struct{}{}
		}
	}

	cb, err := New(Config{
		Clock:         clock,
		Interval:      time.Second,
		Trip:          ConsecutiveFailureTripper(3),
		Recover:       ConsecutiveFailureTripper(1),
		OnTripped:     onTripped(trippedCh),
		TrippedPeriod: 2 * time.Second,
		RecoveryLimit: 2,
	})
	require.NoError(t, err)

	testErr := errors.New("failure")
	errorFn := func() (interface{}, error) { return nil, testErr }
	noErrorFn := func() (interface{}, error) { return nil, nil }
	cases := []struct {
		desc               string
		exec               func() (interface{}, error)
		advance            time.Duration
		errorAssertion     require.ErrorAssertionFunc
		expectedState      State
		expectedGeneration uint64
	}{
		{
			desc:               "no errors remain in standby",
			exec:               noErrorFn,
			errorAssertion:     require.NoError,
			expectedState:      StateStandby,
			expectedGeneration: 1,
		},
		{
			desc:               "error below limit remain in standby",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateStandby,
			expectedGeneration: 1,
		},
		{
			desc:               "another error below limit remain in standby",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateStandby,
			expectedGeneration: 1,
		},
		{
			desc:               "last error below limit remain in standby",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateStandby,
			expectedGeneration: 1,
		},
		{
			desc:               "transition from standby to tripped",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateTripped,
			expectedGeneration: 2,
		},
		{
			desc:               "error remain tripped",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateTripped,
			expectedGeneration: 2,
		},
		{
			desc:               "no error remain tripped",
			exec:               noErrorFn,
			errorAssertion:     require.Error,
			expectedState:      StateTripped,
			expectedGeneration: 2,
		},
		{
			desc:               "transition from tripped to recovering",
			exec:               noErrorFn,
			errorAssertion:     require.NoError,
			expectedState:      StateRecovering,
			expectedGeneration: 3,
			advance:            3 * time.Second,
		},
		{
			desc:               "first failed execution recovering remains in recovering",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateRecovering,
			expectedGeneration: 3,
			advance:            250 * time.Millisecond,
		},
		{
			desc:               "second failed execution recovering transitions to tripped",
			exec:               errorFn,
			errorAssertion:     require.Error,
			expectedState:      StateTripped,
			expectedGeneration: 4,
			advance:            450 * time.Millisecond,
		},
		{
			desc:               "transition from tripped to recovering",
			exec:               noErrorFn,
			errorAssertion:     require.NoError,
			expectedState:      StateRecovering,
			expectedGeneration: 5,
			advance:            3 * time.Second,
		},
		{
			desc:               "transition from recovering to standby",
			exec:               noErrorFn,
			errorAssertion:     require.NoError,
			expectedState:      StateStandby,
			expectedGeneration: 6,
			advance:            450 * time.Millisecond,
		},
		{
			desc:               "remain in standby while in new generation",
			exec:               noErrorFn,
			errorAssertion:     require.NoError,
			expectedState:      StateStandby,
			expectedGeneration: 7,
			advance:            time.Minute,
		},
	}

	for i, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			clock.Advance(tt.advance)
			_, err := cb.Execute(tt.exec)
			tt.errorAssertion(t, err)
			generation, state := cb.currentState(clock.Now())
			require.Equal(t, tt.expectedGeneration, generation, "incorrect generation")
			require.Equal(t, tt.expectedState, state, "incorrect state")

			if state != StateTripped && tt.expectedState == StateTripped {
				select {
				case <-trippedCh:
				default:
					t.Fatalf("step %d expected to get tripped, but wasn't", i)
				}
			}
		})
	}

}

func TestMetrics(t *testing.T) {
	m := Metrics{}

	zero := uint32(0)
	one := uint32(1)
	require.Equal(t, zero, m.Executions)
	require.Equal(t, zero, m.Successes)
	require.Equal(t, zero, m.Failures)
	require.Equal(t, zero, m.ConsecutiveSuccesses)
	require.Equal(t, zero, m.ConsecutiveFailures)

	m.success()

	require.Equal(t, zero, m.Executions)
	require.Equal(t, one, m.Successes)
	require.Equal(t, zero, m.Failures)
	require.Equal(t, one, m.ConsecutiveSuccesses)
	require.Equal(t, zero, m.ConsecutiveFailures)

	m.execute()

	require.Equal(t, one, m.Executions)
	require.Equal(t, one, m.Successes)
	require.Equal(t, zero, m.Failures)
	require.Equal(t, one, m.ConsecutiveSuccesses)
	require.Equal(t, zero, m.ConsecutiveFailures)

	m.failure()

	require.Equal(t, one, m.Executions)
	require.Equal(t, one, m.Successes)
	require.Equal(t, one, m.Failures)
	require.Equal(t, zero, m.ConsecutiveSuccesses)
	require.Equal(t, one, m.ConsecutiveFailures)

	m.reset()

	require.Equal(t, zero, m.Executions)
	require.Equal(t, zero, m.Successes)
	require.Equal(t, zero, m.Failures)
	require.Equal(t, zero, m.ConsecutiveSuccesses)
	require.Equal(t, zero, m.ConsecutiveFailures)
}

func TestIsResponseSuccessful(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		response  *http.Response
		assertion require.BoolAssertionFunc
	}{
		{
			name:      "nil error",
			assertion: require.True,
		},
		{
			name:      "codes.Canceled error",
			err:       status.Error(codes.Canceled, ""),
			assertion: require.False,
		},
		{
			name:      "codes.Unknown error",
			err:       status.Error(codes.Unknown, ""),
			assertion: require.False,
		},
		{
			name:      "codes.Unavailable error",
			err:       status.Error(codes.Unavailable, ""),
			assertion: require.False,
		},
		{
			name:      "codes.Unavailable error",
			err:       status.Error(codes.DeadlineExceeded, ""),
			assertion: require.False,
		},
		{
			name:      "other error",
			err:       trace.NotFound("not found"),
			assertion: require.False,
		},
		{
			name:      "error",
			err:       trace.NotFound(""),
			assertion: require.False,
		},
		{
			name:      "200",
			response:  &http.Response{StatusCode: http.StatusOK},
			assertion: require.True,
		},
		{
			name:      "500",
			response:  &http.Response{StatusCode: http.StatusBadGateway},
			assertion: require.False,
		},
		{
			name:      "404",
			response:  &http.Response{StatusCode: http.StatusNotFound},
			assertion: require.True,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertion(t, IsResponseSuccessful(tt.response, tt.err))
		})
	}
}
