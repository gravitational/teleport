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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
)

// TestDynamicTypeUnknown checks that we correctly translate unknown events strings into the correct proto type.
func TestDynamicUnknownType(t *testing.T) {
	fields := EventFields{
		EventType: "suspicious-cert-event",
		EventCode: "foobar",
	}

	event, err := FromEventFields(fields)
	require.NoError(t, err)

	require.Equal(t, UnknownEvent, event.GetType())
	require.Equal(t, UnknownCode, event.GetCode())
	unknownEvent := event.(*events.Unknown)
	require.Equal(t, "suspicious-cert-event", unknownEvent.UnknownType)
	require.Equal(t, "foobar", unknownEvent.UnknownCode)
}

// TestDynamicNotSet checks that we properly handle cases where the event type is not set.
func TestDynamicTypeNotSet(t *testing.T) {
	fields := EventFields{
		"foo": "bar",
	}

	event, err := FromEventFields(fields)
	require.NoError(t, err)

	require.Equal(t, UnknownEvent, event.GetType())
	require.Equal(t, UnknownCode, event.GetCode())
	unknownEvent := event.(*events.Unknown)
	require.Empty(t, unknownEvent.UnknownType)
	require.Empty(t, unknownEvent.UnknownCode)
}

// TestDynamicTypeUnknown checks that we correctly translate known events into the correct proto type.
func TestDynamicKnownType(t *testing.T) {
	fields := EventFields{
		EventType: "print",
	}

	event, err := FromEventFields(fields)
	require.NoError(t, err)
	printEvent := event.(*events.SessionPrint)
	require.Equal(t, SessionPrintEvent, printEvent.GetType())
}

func TestGetTeleportUser(t *testing.T) {
	tests := []struct {
		name  string
		event events.AuditEvent
		want  string
	}{
		{
			name:  "event without user metadata",
			event: &events.InstanceJoin{},
			want:  "",
		},
		{
			name: "event with user metadata",
			event: &events.SessionStart{
				UserMetadata: events.UserMetadata{
					User: "user-1",
				},
			},
			want: "user-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GetTeleportUser(tt.event))
		})
	}
}
