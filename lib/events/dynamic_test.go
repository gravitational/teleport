/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	require.Equal(t, "", unknownEvent.UnknownType)
	require.Equal(t, "", unknownEvent.UnknownCode)
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
