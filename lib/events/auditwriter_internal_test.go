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
