// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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

	ctx := t.Context()
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

	ctx := t.Context()
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

	ctx := t.Context()
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

	clock.BlockUntilContext(ctx, 1)

	totalAttempts := updateWithRetryMaxRetries + 1

	for expectedCallCnt := 2; expectedCallCnt < totalAttempts; expectedCallCnt++ {
		clock.Advance(updateWithRetryHalfJitterBetweenAttempts)
		clock.BlockUntilContext(ctx, 1)

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

	ctx := t.Context()
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

	ctx := t.Context()
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

	clock.BlockUntilContext(ctx, 1)

	totalAttempts := updateWithRetryMaxRetries + 1
	somePoint := totalAttempts - 1

	for expectedCallCnt := 2; expectedCallCnt < totalAttempts; expectedCallCnt++ {
		clock.Advance(updateWithRetryHalfJitterBetweenAttempts)
		clock.BlockUntilContext(ctx, 1)

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
