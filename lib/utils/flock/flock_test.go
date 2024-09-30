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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLock verifies that second lock call is blocked until first is released.
func TestLock(t *testing.T) {
	var locked atomic.Bool

	ctx := context.Background()
	lockFile := filepath.Join(os.TempDir(), ".lock")
	t.Cleanup(func() {
		require.NoError(t, os.Remove(lockFile))
	})

	// Acquire first lock should not return any error.
	unlock, err := Lock(ctx, lockFile)
	require.NoError(t, err)
	locked.Store(true)

	signal := make(chan struct{})
	errChan := make(chan error)
	go func() {
		signal <- struct{}{}
		unlock, err := Lock(ctx, lockFile)
		if err != nil {
			errChan <- err
		}
		if locked.Load() {
			errChan <- fmt.Errorf("first lock is still acquired, second lock must be blocking")
		}
		unlock()
		signal <- struct{}{}
	}()

	<-signal
	// We have to wait till next lock is reached to ensure we block execution of goroutine.
	// Since this is system call we can't track if the function reach blocking state already.
	time.Sleep(100 * time.Millisecond)
	locked.Store(false)
	unlock()

	select {
	case <-signal:
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "second lock is not released")
	}
}
