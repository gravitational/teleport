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

package retryutils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUpdateWithRetryTest[R any](t *testing.T) (*clockwork.FakeClock, *refreshFnRecorder[R], *updateFnRecorder[R]) {
	t.Helper()
	return clockwork.NewFakeClock(), new(refreshFnRecorder[R]), new(updateFnRecorder[R])

}

func Test_UpdateWithRetry_returns_error_if_refreshFn_returns_error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock, refreshRecorder, updateRecorder := setupUpdateWithRetryTest[string](t)

	refreshRecorder.retErr = errors.New("error from retryFn")

	err := UpdateWithRetry(ctx, clock, refreshRecorder.Refresh, updateRecorder.Update)
	require.Error(t, err)
	require.Equal(t, refreshRecorder.retErr.Error(), err.Error())

	require.Equal(t, 1, refreshRecorder.callCnt)
	require.Equal(t, 0, updateRecorder.callCnt)

	require.False(t, refreshRecorder.lastArgIsRetry)
}

func Test_UpdateWithRetry_passes_value_from_refreshFn_to_updateFn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock, refreshRecorder, updateRecorder := setupUpdateWithRetryTest[string](t)

	resource := "test_resource_value_1"

	refreshRecorder.retResource = resource

	err := UpdateWithRetry(ctx, clock, refreshRecorder.Refresh, updateRecorder.Update)
	require.NoError(t, err)

	require.Equal(t, 1, refreshRecorder.callCnt)
	require.Equal(t, 1, updateRecorder.callCnt)
	require.Equal(t, resource, updateRecorder.lastArgResource)

	require.False(t, refreshRecorder.lastArgIsRetry)
}

func Test_UpdateWithRetry_retries_if_updateFn_returns_CompareFailedError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock, refreshRecorder, updateRecorder := setupUpdateWithRetryTest[string](t)

	updateRecorder.retErr = trace.CompareFailed("test_compare_failed_1")

	updateWithRetryRetCh := make(chan error)

	go func() {
		updateWithRetryRetCh <- UpdateWithRetry(ctx, clock, refreshRecorder.Refresh, updateRecorder.Update)
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Equal(c, 1, refreshRecorder.read().callCnt)
		require.Equal(c, 1, updateRecorder.read().callCnt)
		require.False(c, refreshRecorder.read().lastArgIsRetry)
	}, 4*time.Second, 10*time.Millisecond)

	clock.BlockUntil(1)

	totalAttempts := updateWithRetryMaxRetries + 1

	for expectedCallCnt := 2; expectedCallCnt < totalAttempts; expectedCallCnt++ {
		clock.Advance(updateWithRetryHalfJitterBetweenAttempts)
		clock.BlockUntil(1)

		require.Equal(t, expectedCallCnt, refreshRecorder.read().callCnt)
		require.Equal(t, expectedCallCnt, updateRecorder.read().callCnt)
		require.True(t, refreshRecorder.read().lastArgIsRetry)
	}

	clock.Advance(updateWithRetryHalfJitterBetweenAttempts)

	err := <-updateWithRetryRetCh
	require.Error(t, err)
	require.Equal(t, updateRecorder.read().retErr.Error(), err.Error())

	require.Equal(t, totalAttempts, refreshRecorder.read().callCnt)
	require.Equal(t, totalAttempts, updateRecorder.read().callCnt)
	require.True(t, refreshRecorder.read().lastArgIsRetry)
}

func Test_UpdateWithRetry_do_not_retry_if_updateFn_returns_different_error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock, refreshRecorder, updateRecorder := setupUpdateWithRetryTest[string](t)

	updateRecorder.retErr = errors.New("different_update_error_1")

	err := UpdateWithRetry(ctx, clock, refreshRecorder.Refresh, updateRecorder.Update)
	require.Error(t, err)
	require.Equal(t, updateRecorder.retErr.Error(), err.Error())

	require.Equal(t, 1, refreshRecorder.callCnt)
	require.Equal(t, 1, updateRecorder.callCnt)
}

func Test_UpdateWithRetry_stops_retrying_if_updateFn_returns_different_error_at_some_point(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock, refreshRecorder, updateRecorder := setupUpdateWithRetryTest[string](t)

	updateRecorder.retErr = trace.CompareFailed("test_compare_failed_1")

	updateWithRetryRetCh := make(chan error)

	go func() {
		updateWithRetryRetCh <- UpdateWithRetry(ctx, clock, refreshRecorder.Refresh, updateRecorder.Update)
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Equal(c, 1, refreshRecorder.read().callCnt)
		require.Equal(c, 1, updateRecorder.read().callCnt)
		require.False(c, refreshRecorder.read().lastArgIsRetry)
	}, 4*time.Second, 10*time.Millisecond)

	clock.BlockUntil(1)

	totalAttempts := updateWithRetryMaxRetries + 1
	somePoint := totalAttempts - 1

	for expectedCallCnt := 2; expectedCallCnt < totalAttempts; expectedCallCnt++ {
		clock.Advance(updateWithRetryHalfJitterBetweenAttempts)
		clock.BlockUntil(1)

		if expectedCallCnt == somePoint {
			updateRecorder.mu.Lock()
			updateRecorder.retErr = fmt.Errorf("achieved some point (%d)", somePoint)
			updateRecorder.mu.Unlock()
			break
		}

		require.Equal(t, expectedCallCnt, refreshRecorder.read().callCnt)
		require.Equal(t, expectedCallCnt, updateRecorder.read().callCnt)
		require.True(t, refreshRecorder.read().lastArgIsRetry)
	}

	clock.Advance(updateWithRetryHalfJitterBetweenAttempts)

	err := <-updateWithRetryRetCh
	require.Error(t, err)
	require.Equal(t, updateRecorder.read().retErr.Error(), err.Error())

	require.Equal(t, totalAttempts, refreshRecorder.read().callCnt)
	require.Equal(t, totalAttempts, updateRecorder.read().callCnt)
	require.True(t, refreshRecorder.read().lastArgIsRetry)
}

type refreshFnRecorder[R any] struct {
	mu sync.RWMutex
	refreshFnRecorderState[R]
}

type refreshFnRecorderState[R any] struct {
	callCnt int

	lastArgIsRetry bool

	retResource R
	retErr      error
}

func (r *refreshFnRecorder[R]) read() refreshFnRecorderState[R] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.refreshFnRecorderState
}

func (r *refreshFnRecorder[R]) Refresh(_ context.Context, isRetry bool) (R, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callCnt++
	r.lastArgIsRetry = isRetry
	return r.retResource, r.retErr
}

type updateFnRecorder[R any] struct {
	mu sync.RWMutex
	updateFnRecorderState[R]
}

type updateFnRecorderState[R any] struct {
	callCnt int

	lastArgResource R

	retErr error
}

func (u *updateFnRecorder[R]) read() updateFnRecorderState[R] {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.updateFnRecorderState
}

func (u *updateFnRecorder[R]) Update(_ context.Context, resource R) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.callCnt++
	u.lastArgResource = resource
	return u.retErr
}
