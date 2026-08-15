// Copyright 2026 Gravitational, Inc.
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

package wait

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

func errIsRetryable(t require.TestingT, err error, args ...interface{}) {
	require.Error(t, err)
	require.False(t, retryutils.IsPermanent(err))
}

func errIsNotRetryable(t require.TestingT, err error, args ...interface{}) {
	require.Error(t, err)
	require.True(t, retryutils.IsPermanent(err))
}

func TestCheckFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		res       string
		err       error
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "found",
			res:       "found",
			assertErr: require.NoError,
		},
		{
			name:      "not found",
			err:       trace.NotFound("not found"),
			assertErr: errIsRetryable,
		},
		{
			name:      "other error",
			err:       trace.BadParameter("bad parameter"),
			assertErr: errIsNotRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, checkFound(tt.res, tt.err))
		})
	}
}

func TestCheckNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		res       string
		err       error
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "found",
			res:       "found",
			assertErr: errIsRetryable,
		},
		{
			name:      "not found",
			err:       trace.NotFound("not found"),
			assertErr: require.NoError,
		},
		{
			name:      "other error",
			err:       trace.BadParameter("bad parameter"),
			assertErr: errIsNotRetryable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, checkNotFound(tt.res, tt.err))
		})
	}
}

type fakeResource struct {
	name     string
	revision string
}

func (f fakeResource) GetMetadata() *headerv1.Metadata {
	return headerv1.Metadata_builder{
		Name:     f.name,
		Revision: f.revision,
	}.Build()
}

func TestCheckRevisionChanged(t *testing.T) {
	t.Parallel()
	const resourceName = "test"
	initialRevision := uuid.NewString()
	tests := []struct {
		name      string
		res       fakeResource
		err       error
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "same metadata",
			res:       fakeResource{name: resourceName, revision: initialRevision},
			assertErr: errIsRetryable,
		},
		{
			name:      "different metadata",
			res:       fakeResource{name: resourceName, revision: uuid.NewString()},
			assertErr: require.NoError,
		},
		{
			name:      "error",
			err:       trace.NotFound("not found"),
			assertErr: errIsNotRetryable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, checkRevisionChange[fakeResource](initialRevision)(tt.res, tt.err))
		})
	}

}

type mockCheck struct {
	t         *testing.T
	counter   int
	responses []error
}

func (m *mockCheck) check(string, error) error {
	m.counter += 1
	assert.LessOrEqual(m.t, m.counter, len(m.responses), "no response left")
	return m.responses[m.counter-1]
}

func (m *mockCheck) empty() bool {
	return m.counter == len(m.responses)
}

func TestWaitUntil(t *testing.T) {
	const response = "response"
	// creating the notFoundErr once to avoid capturing the call track dozens of times
	notFoundErr := trace.NotFound("not found")
	tests := []struct {
		name          string
		mock          *mockCheck
		prepareCtx    func(ctx context.Context) context.Context
		expectedTries int
		assertErr     assert.ErrorAssertionFunc
	}{
		{
			name:       "immediately return",
			prepareCtx: func(ctx context.Context) context.Context { return ctx },
			mock: &mockCheck{
				responses: []error{nil},
			},
			expectedTries: 1,
			assertErr:     assert.NoError,
		},
		{
			name:       "with retries",
			prepareCtx: func(ctx context.Context) context.Context { return ctx },
			mock: &mockCheck{
				responses: []error{notFoundErr, notFoundErr, nil},
			},
			expectedTries: 3,
			assertErr:     assert.NoError,
		},
		{
			name: "expired context",
			prepareCtx: func(ctx context.Context) context.Context {
				newCtx, cancel := context.WithCancel(ctx)
				cancel()
				return newCtx
			},
			mock: &mockCheck{
				responses: []error{notFoundErr},
			},
			// Linear retry `For` function runs at least once, even if context is canceled.
			expectedTries: 1,
			assertErr:     assert.Error,
		},
		{
			name:       "permanent error",
			prepareCtx: func(ctx context.Context) context.Context { return ctx },
			mock: &mockCheck{
				responses: []error{notFoundErr, retryutils.PermanentRetryError(notFoundErr)},
			},
			expectedTries: 2,
			assertErr:     assert.Error,
		},
		{
			name:       "maxTries exceeded",
			prepareCtx: func(ctx context.Context) context.Context { return ctx },
			mock: &mockCheck{
				responses: []error{
					// Max tries is 4
					notFoundErr,
					notFoundErr,
					notFoundErr,
					notFoundErr,
				},
			},
			expectedTries: 4,
			assertErr:     assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Test setup: construct fixtures and ways to track the test's progress.'
				var done atomic.Bool
				var getCalls atomic.Int64
				var tries int
				// inject the test-specific context into the mock (this saves us from making tt.mock a constructor)
				tt.mock.t = t

				get := func(_ context.Context) (string, error) {
					getCalls.Add(1)
					return response, nil
				}

				clock := clockwork.NewFakeClock()

				// Test setup: build a controlled retry config that is deterministic.
				// This allows us to inject a fake clock and know how to to advance it for every iteration.
				retryConfig := &retryutils.LinearConfig{
					// Prevent immediate start
					First:     1,
					Step:      100 * time.Millisecond,
					Max:       time.Second,
					AutoReset: 0,
					// No jitter so we can be deterministic
					Clock: clock,
				}

				backoff, err := retryutils.NewLinear(*retryConfig)
				require.NoError(t, err)

				go func() {
					// Test execution: run WaitUntil. This will routine will block waiting for the clock.
					// The main test routine is responsible for unblocking it.
					result, err := Until[string](tt.prepareCtx(t.Context()), get, tt.mock.check, WithMaxTries(4), WithRetryConfig(retryConfig))

					// Test validation: check the function return value and error.
					tt.assertErr(t, err)
					// Implementation details: we always return the last get value, and for the sake of simplicity, get always returns [response].
					assert.Equal(t, response, result)
					assert.Equal(t, int64(tt.expectedTries), getCalls.Load())
					done.Store(true)
				}()

				for {
					tries += 1
					synctest.Wait()
					if done.Load() {
						// Test validation, if the function returned, validate the numberof tries and calls that were performned.
						require.Equal(t, tt.expectedTries, tries, "WaitUntil returned too early, was expecting more tries")
						require.True(t, tt.mock.empty(), "mock is not empty")
						return
					}
					if tries > tt.expectedTries {
						require.FailNow(t, "WaitUntil did not return after expected number of tries")
					}
					// Test execution: advance clock to the next try.
					backoff.Inc()
					clock.Advance(backoff.Duration())
				}
			})
		})
	}

}
