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

package signal

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetSignalHandler verifies the cancellation stack order.
func TestGetSignalHandler(t *testing.T) {
	testHandler := GetSignalHandler()
	parent := context.Background()

	ctx1, cancel1 := testHandler.NotifyContext(parent)
	ctx2, cancel2 := testHandler.NotifyContext(parent)
	ctx3, cancel3 := testHandler.NotifyContext(parent)
	ctx4, cancel4 := testHandler.NotifyContext(parent)
	t.Cleanup(func() {
		cancel4()
		cancel2()
		cancel1()
	})

	// Verify that all context not canceled.
	require.NoError(t, ctx4.Err())
	require.NoError(t, ctx3.Err())
	require.NoError(t, ctx2.Err())
	require.NoError(t, ctx1.Err())

	// Cancel context manually to ensure it was removed from stack in right order.
	cancel3()

	// Check that last added context is canceled.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case <-ctx4.Done():
		assert.ErrorIs(t, ctx3.Err(), context.Canceled)
		assert.NoError(t, ctx2.Err())
		assert.NoError(t, ctx1.Err())
	case <-time.After(time.Second):
		assert.Fail(t, "context 3 must be canceled")
	}

	// Send interrupt signal to cancel next context in the stack.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case <-ctx2.Done():
		assert.ErrorIs(t, ctx4.Err(), context.Canceled)
		assert.ErrorIs(t, ctx3.Err(), context.Canceled)
		assert.NoError(t, ctx1.Err())
	case <-time.After(time.Second):
		assert.Fail(t, "context 2 must be canceled")
	}

	// All context must be canceled.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case <-ctx1.Done():
		assert.ErrorIs(t, ctx4.Err(), context.Canceled)
		assert.ErrorIs(t, ctx3.Err(), context.Canceled)
		assert.ErrorIs(t, ctx2.Err(), context.Canceled)
	case <-time.After(time.Second):
		assert.Fail(t, "context 1 must be canceled")
	}
}
