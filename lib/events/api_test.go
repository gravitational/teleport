/*
Copyright 2018 Gravitational, Inc.

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
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
)

func TestUpdateEventFields(t *testing.T) {
	t.Parallel()

	event := Event{
		Name: "test.event",
		Code: "TEST0001I",
	}
	fields := EventFields{
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	}
	require.NoError(t, UpdateEventFields(event, fields, clockwork.NewFakeClock(), utils.NewFakeUID()))

	// Check the fields have been updated appropriately.
	require.Equal(t, EventFields{
		EventType:   event.Name,
		EventID:     fixtures.UUID,
		EventCode:   event.Code,
		EventTime:   time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC),
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	}, fields)
}
