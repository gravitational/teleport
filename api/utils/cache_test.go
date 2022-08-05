/*
Copyright 2022 Gravitational, Inc.

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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	clock := clockwork.NewFakeClock()
	cache := NewCache[string, int](CacheConfig{
		Clock:      clock,
		DefaultTTL: time.Minute,
	})

	t.Run("auto cleanup", func(t *testing.T) {
		cache.Set("item1", 1)                   // Expires in one minute.
		cache.SetWithTTL("item2", 2, time.Hour) // Expires in one hour.
		require.Equal(t, 2, cache.Len())

		// Auto cleanup is not triggered yet.
		clock.Advance(time.Minute * 2)
		require.Equal(t, 2, cache.Len())

		// Auto cleanup is triggered, and item1 should be removed.
		clock.Advance(time.Minute * 30)
		require.Equal(t, 1, cache.Len())

		// Auto cleanup is triggered, and item2 should be removed.
		clock.Advance(time.Minute * 60)
		require.Equal(t, 0, cache.Len())
	})

	t.Run("Delete", func(t *testing.T) {
		// Never expires.
		cache.SetWithTTL("item-never-expire", 333, 0)
		clock.Advance(time.Hour * 1000)

		value, found := cache.Get("item-never-expire")
		require.True(t, found)
		require.Equal(t, 333, value)

		// Now deletes it "manually".
		cache.Delete("item-never-expire")

		_, found = cache.Get("item-never-expire")
		require.False(t, found)
	})

	t.Run("Get expired", func(t *testing.T) {
		cache.Set("expire-in-one-minute", 100)
		value, found := cache.Get("expire-in-one-minute")
		require.True(t, found)
		require.Equal(t, 100, value)

		// Auto cleanup is not triggered yet.
		clock.Advance(time.Minute * 2)

		// Get should not return expired item.
		_, found = cache.Get("expire-in-one-minute")
		require.False(t, found)
	})
}
