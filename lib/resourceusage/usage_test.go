// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourceusage

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	eventstest "github.com/gravitational/teleport/lib/events/test"
)

func TestGetAccessRequestMonthlyUsage(t *testing.T) {
	makeEvent := func(eventType string, id string, timestamp time.Time) apievents.AuditEvent {
		return &apievents.AccessRequestCreate{
			Metadata: apievents.Metadata{
				Type: eventType,
				Time: timestamp,
			},
			RequestID: id,
		}
	}

	// Mock audit log
	clock := clockwork.NewFakeClockAt(time.Date(2023, 07, 15, 1, 2, 3, 0, time.UTC))
	now := clock.Now()
	mockEvents := []apievents.AuditEvent{
		makeEvent(events.AccessRequestCreateEvent, "aaa", now.AddDate(0, 0, -4)),
		makeEvent(events.AccessRequestCreateEvent, "bbb", now.AddDate(0, 0, -3)),
		makeEvent(events.AccessRequestCreateEvent, "ccc", now.AddDate(0, 0, -2)),
		makeEvent(events.AccessRequestCreateEvent, "ddd", now.AddDate(0, 0, -1)),
	}

	al := eventstest.NewMockAuditLogSessionStreamer(mockEvents, func(req events.SearchEventsRequest) error {
		if !slices.Equal([]string{events.AccessRequestCreateEvent}, req.EventTypes) {
			return trace.BadParameter("expected AccessRequestCreateEvent only, got %v", req.EventTypes)
		}
		return nil
	})

	result, err := GetAccessRequestMonthlyUsage(context.Background(), al, now)
	require.NoError(t, err)
	require.Equal(t, len(mockEvents), result)
}
