// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package utils_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestKeyLock(t *testing.T) {
	t.Parallel()

	t.Run("lock and unlock", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]

		locker.Lock("test-key")
		locker.Unlock("test-key")

		// Re-locking after unlock should not block.
		locker.Lock("test-key")
		locker.Unlock("test-key")
	})

	t.Run("acquire", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]

		unlock := locker.Acquire("test-key")
		unlock()
	})

	t.Run("re-acquiring after unlock succeeds", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]

		unlock1 := locker.Acquire("test-key")
		unlock1()

		unlock2 := locker.Acquire("test-key")
		unlock2()
	})

	t.Run("concurrent goroutines on same key", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]
		const goroutineCount = 200

		var concurrentCount int
		var maxConcurrent int
		var wg sync.WaitGroup

		for i := 0; i < goroutineCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				unlock := locker.Acquire("key")

				// It is safe to increment concurrentCount without
				// atomic operations since the lock ensures only one
				// goroutine is in this critical section at a time.
				concurrentCount++
				if concurrentCount > maxConcurrent {
					maxConcurrent = concurrentCount
				}
				concurrentCount--

				unlock()
			}()
		}
		wg.Wait()
		require.Equal(t, 1, maxConcurrent)
	})

	t.Run("different keys are independent", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]

		unlockA := locker.Acquire("key-a")

		// Locking a different key should not block.
		unlockB := locker.Acquire("key-b")

		unlockA()
		unlockB()
	})

	t.Run("double unlock is safe", func(t *testing.T) {
		t.Parallel()
		var locker utils.KeyLock[string]

		unlock := locker.Acquire("test-key")

		unlock()
		require.NotPanics(t, func() {
			unlock()
		})
	})
}
