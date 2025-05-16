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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/tbot/service"
)

func TestWatchedValue(t *testing.T) {
	t.Parallel()

	value := service.NewWatchedValue(1)
	assert.Equal(t, 1, value.Get())

	notifCh, close := value.ChangeNotifications()
	t.Cleanup(close)

	changed := value.Set(2)
	assert.True(t, changed)

	select {
	case <-notifCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for notification")
	}
	assert.Equal(t, 2, value.Get())

	_ = value.Set(3)
	_ = value.Set(4)

	select {
	case <-notifCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for notification")
	}
	assert.Equal(t, 4, value.Get())

	close()

	_ = value.Set(5)

	// We shouldn't get any more notifications after the watcher is closed.
	select {
	case <-notifCh:
		t.Fatal("received unexpected notification")
	case <-time.After(500 * time.Millisecond):
	}
}
