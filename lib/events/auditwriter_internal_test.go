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

package events

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestBytesToSessionPrintEvents(t *testing.T) {
	b := make([]byte, MaxProtoMessageSizeBytes+1)
	_, err := rand.Read(b)
	require.NoError(t, err)

	events := bytesToSessionPrintEvents(b)
	require.Len(t, events, 2)

	event0, ok := events[0].(*apievents.SessionPrint)
	require.True(t, ok)

	event1, ok := events[1].(*apievents.SessionPrint)
	require.True(t, ok)

	allBytes := append(event0.Data, event1.Data...)
	require.Equal(t, b, allBytes)
}
