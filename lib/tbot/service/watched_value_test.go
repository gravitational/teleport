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
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/service"
)

func TestWatchedValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	value := service.NewWatchedValue(1)
	assert.Equal(t, 1, value.Get())

	current, watcher := value.Watch()
	t.Cleanup(watcher.Close)
	assert.Equal(t, 1, current)

	nextVal := make(chan int)
	go func() {
		next, ok := watcher.Wait(ctx)
		assert.True(t, ok)
		nextVal <- next
	}()

	changed := value.Set(2)
	assert.True(t, changed)
	assert.Equal(t, 2, <-nextVal)

	_ = value.Set(3)
	_ = value.Set(4)

	next, _ := watcher.Wait(ctx)
	assert.Equal(t, 4, next)

	watcher.Close()

	_ = value.Set(5)

	// We shouldn't get any more values after the watcher is closed.
	ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)
	_, ok := watcher.Wait(ctx)
	require.False(t, ok)
}
