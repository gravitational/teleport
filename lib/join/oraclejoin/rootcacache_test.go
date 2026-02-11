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

package oraclejoin

import (
	"crypto/x509"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCACache(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testRootCACache)
}

func testRootCACache(t *testing.T) {
	cache := NewRootCACache()

	caPool := x509.NewCertPool()
	const caTTL = 5 * time.Minute

	var loadCount atomic.Int32
	loadFn := func() (*x509.CertPool, time.Time, error) {
		loadCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		expires := time.Now().Add(caTTL)
		return caPool, expires, nil
	}

	// Getting the same key many times only results in 1 load.
	for range 10 {
		pool, err := cache.get(t.Context(), "test", loadFn)
		require.Equal(t, caPool, pool)
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), loadCount.Load())

	// Waiting until the entry expires then getting the same key again a number
	// of times results in 1 additional load.
	time.Sleep(caTTL + time.Minute)
	for range 10 {
		_, err := cache.get(t.Context(), "test", loadFn)
		require.NoError(t, err)
	}
	require.Equal(t, int32(2), loadCount.Load())

	// Getting 10 different keys multiple times results in 10 total loads.
	loadCount.Store(0)
	for range 10 {
		for i := range 10 {
			_, err := cache.get(t.Context(), strconv.Itoa(i), loadFn)
			require.NoError(t, err)
		}
	}
	require.Equal(t, int32(10), loadCount.Load())

	// Wait until all current entries expire, then do 1 more load, expired
	// entries should be cleaned up.
	time.Sleep(caTTL + time.Minute)
	_, err := cache.get(t.Context(), "test", loadFn)
	require.NoError(t, err)
	require.Len(t, cache.entries, 1)

	// Error values should not be cached.
	_, err = cache.get(t.Context(), "testerror", func() (*x509.CertPool, time.Time, error) {
		return nil, time.Time{}, errors.New("test error")
	})
	require.Error(t, err)
	// An immediate re-fetch does not get the error.
	_, err = cache.get(t.Context(), "testerror", loadFn)
	require.NoError(t, err)

	// Even with many concurrent calls the value will only be loaded once.
	loadCount.Store(0)
	var wg sync.WaitGroup
	wg.Add(100)
	for range 100 {
		go func() {
			_, err := cache.get(t.Context(), "concurrent", loadFn)
			assert.NoError(t, err)
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), loadCount.Load())
}
