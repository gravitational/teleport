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

package resourceusage

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

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
	require.Len(t, mockEvents, result)
}
