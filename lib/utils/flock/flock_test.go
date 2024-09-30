/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package flock

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLock verifies that second lock call is blocked until first is released.
func TestLock(t *testing.T) {
	var locked atomic.Bool

	lockFile := filepath.Join(os.TempDir(), ".lock")
	t.Cleanup(func() {
		require.NoError(t, os.Remove(lockFile))
	})

	// Acquire first lock should not return any error.
	unlock, err := Lock(lockFile, false)
	require.NoError(t, err)
	locked.Store(true)

	signal := make(chan struct{})
	errChan := make(chan error)
	go func() {
		signal <- struct{}{}
		unlock, err := Lock(lockFile, false)
		if err != nil {
			errChan <- err
			return
		}
		if locked.Load() {
			errChan <- fmt.Errorf("first lock is still acquired, second lock must be blocking")
			return
		}
		if err := unlock(); err != nil {
			errChan <- err
			return
		}
		signal <- struct{}{}
	}()

	<-signal
	// We have to wait till next lock is reached to ensure we block execution of goroutine.
	// Since this is system call we can't track if the function reach blocking state already.
	time.Sleep(100 * time.Millisecond)
	locked.Store(false)
	require.NoError(t, unlock())

	select {
	case err := <-errChan:
		require.NoError(t, err)
	case <-signal:
	case <-time.After(5 * time.Second):
		require.Fail(t, "second lock is not released")
	}
}

// TestLockNonBlock verifies that second lock call returns error until first lock is released.
func TestLockNonBlock(t *testing.T) {
	lockfile := filepath.Join(t.TempDir(), ".lock")
	unlock, err := Lock(lockfile, true)
	require.NoError(t, err)

	_, err = Lock(lockfile, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLocked)

	err = unlock()
	require.NoError(t, err)

	unlock2, err := Lock(lockfile, true)
	require.NoError(t, err)
	err = unlock2()
	require.NoError(t, err)
}
