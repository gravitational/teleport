/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func failIfCalled(t *testing.T, value string, err error) func(context.Context) (string, error) {
	return func(_ context.Context) (string, error) {
		require.Fail(t, "should not be called")
		return value, err
	}
}

func failIfCalledTwice(t *testing.T, value string, err error) func(context.Context) (string, error) {
	var fuse bool
	return func(_ context.Context) (string, error) {
		if !fuse {
			fuse = true
			return value, err
		}
		require.Fail(t, "should not be called twice")
		return value, err
	}
}

func TestTimedMemoize_Get(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	longBefore := now.Add(-2 * time.Hour)
	longAfter := now.Add(2 * time.Hour)

	upstreamValue := "upstream"
	cachedValue := "cached"
	upstreamError := trace.LimitExceeded("rate-limited")
	oldUpstreamError := trace.CompareFailed("comparison failed")

	assertUncachedError := func(t2 require.TestingT, err error, _ ...interface{}) {
		require.Equal(t2, err, upstreamError)
	}
	assertCachedError := func(t2 require.TestingT, err error, _ ...interface{}) {
		_, ok := trace.Unwrap(err).(cachedError)
		require.True(t2, ok)
	}

	tests := []struct {
		name          string
		cachedValue   string
		cachedError   error
		validUntil    time.Time
		getter        func(*testing.T, string, error) func(context.Context) (string, error)
		expectedValue string
		assertErr     require.ErrorAssertionFunc
	}{
		{
			name:          "fresh cache",
			cachedValue:   "",
			cachedError:   nil,
			validUntil:    time.Time{},
			getter:        failIfCalledTwice,
			expectedValue: upstreamValue,
			assertErr:     assertUncachedError,
		},
		{
			name:          "valid cache",
			cachedValue:   cachedValue,
			cachedError:   newCachedError(oldUpstreamError, longAfter),
			validUntil:    longAfter,
			getter:        failIfCalled,
			expectedValue: cachedValue,
			assertErr:     assertCachedError,
		},
		{
			name:          "expired cache",
			cachedValue:   cachedValue,
			cachedError:   newCachedError(oldUpstreamError, longBefore),
			validUntil:    longBefore,
			getter:        failIfCalledTwice,
			expectedValue: upstreamValue,
			assertErr:     assertUncachedError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := TimedMemoize[string]{
				cachedValue:   tt.cachedValue,
				cachedError:   tt.cachedError,
				validUntil:    tt.validUntil,
				clock:         clockwork.NewFakeClockAt(now),
				cacheDuration: 30 * time.Minute,
				getter:        tt.getter(t, upstreamValue, upstreamError),
			}
			result, err := cache.Get(ctx)
			require.Equal(t, tt.expectedValue, result)
			// The first error might or might not be cached
			tt.assertErr(t, err)
			result, err = cache.Get(ctx)
			require.Equal(t, tt.expectedValue, result)
			// The second error must be cached
			assertCachedError(t, err)
		})
	}
}
