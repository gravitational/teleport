/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package once

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyedValue verifies the basic expected function of KeyedValue.
func TestKeyedValue(t *testing.T) {

	var callsMu sync.Mutex
	calls := make(map[int]int)

	fn, cancel := KeyedValue(func(ctx context.Context, key int) (string, error) {
		callsMu.Lock()
		defer callsMu.Unlock()
		calls[key]++
		return fmt.Sprintf("value-%d", key), nil
	})
	defer cancel()

	results := make(chan struct {
		key int
		val string
		err error
	})
	for i := 0; i < 16; i++ {
		key := i % 2
		go func() {
			for j := 0; j < 16; j++ {
				val, err := fn(context.Background(), key)
				results <- struct {
					key int
					val string
					err error
				}{key, val, err}
			}
		}()
	}

	for i := 0; i < 256; i++ {
		res := <-results
		require.NoError(t, res.err)
		require.Equal(t, fmt.Sprintf("value-%d", res.key), res.val)
	}

	callsMu.Lock()
	require.Equal(t, map[int]int{0: 1, 1: 1}, calls)
	callsMu.Unlock()
}

// TestKeyedValueCancellation verifies that KeyedValue respects caller context cancellation
// and that the cancel function returned by KeyedValue causes cancellation of inflight
// calls to the wrapped function.
func TestKeyedValueCancellation(t *testing.T) {
	canceledInner := errors.New("inner-function-canceled")

	unblock := make(chan struct{}, 1)

	fn, cancelInner := KeyedValue(func(ctx context.Context, key int) (string, error) {
		select {
		case <-unblock:
			return fmt.Sprintf("value-%d", key), nil
		case <-ctx.Done():
			return "", canceledInner
		}
	})
	defer cancelInner()

	// verify normal operation
	unblock <- struct{}{}
	val, err := fn(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "value-1", val)

	// verify cancellation of caller context
	tctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	val, err = fn(tctx, 2)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, val)

	// unblock the inner function and verify it completes normally
	unblock <- struct{}{}
	val, err = fn(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, "value-2", val)

	// verify cancellation of the inner function via the cancel function
	cancelInner()
	val, err = fn(context.Background(), 3)
	require.ErrorIs(t, err, canceledInner)
	require.Empty(t, val)
}
