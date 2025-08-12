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

package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/internal"
)

func TestChannelBroadcaster(t *testing.T) {
	cb := internal.NewChannelBroadcaster()
	sub1, unsubscribe1 := cb.Subscribe()
	t.Cleanup(unsubscribe1)
	sub2, unsubscribe2 := cb.Subscribe()
	t.Cleanup(unsubscribe2)

	cb.Broadcast()
	require.NotEmpty(t, sub1)
	require.NotEmpty(t, sub2)

	// remove value from sub1 to check that if sub2 is full broadcasting still
	// works
	<-sub1
	cb.Broadcast()
	require.NotEmpty(t, sub1)

	// empty out both channels and ensure unsubscribing means they no longer
	// receive values
	<-sub1
	<-sub2
	unsubscribe1()
	unsubscribe2()
	cb.Broadcast()
	require.Empty(t, sub1)
	require.Empty(t, sub2)

	// ensure unsubscribing twice doesn't cause panic
	unsubscribe1()
}
