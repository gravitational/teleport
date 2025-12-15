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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
)

func TestFileLogPagination(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	log, err := NewFileLog(FileLogConfig{
		Dir:            t.TempDir(),
		RotationPeriod: time.Hour * 24,
		Clock:          clock,
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(ctx, &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "a",
			Type: SessionJoinEvent,
			Time: clock.Now().UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "bob",
		},
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(ctx, &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "b",
			Type: SessionJoinEvent,
			Time: clock.Now().Add(time.Minute).UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "alice",
		},
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(ctx, &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "c",
			Type: SessionJoinEvent,
			Time: clock.Now().Add(time.Minute * 2).UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "dave",
		},
	})
	require.NoError(t, err)

	from := clock.Now().Add(-time.Hour).UTC()
	to := clock.Now().Add(time.Hour).UTC()
	eventArr, checkpoint, err := log.SearchEvents(ctx, SearchEventsRequest{
		From:  from,
		To:    to,
		Limit: 2,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, eventArr, 2)
	require.NotEmpty(t, checkpoint)

	eventArr, checkpoint, err = log.SearchEvents(ctx, SearchEventsRequest{
		From:     from,
		To:       to,
		Limit:    2,
		Order:    types.EventOrderAscending,
		StartKey: checkpoint,
	})
	require.NoError(t, err)
	require.Len(t, eventArr, 1)
	require.Empty(t, checkpoint)
}

func TestFileLogCheckpoint(t *testing.T) {
	clock := clockwork.NewFakeClock()
	testRotationPeriod := time.Hour * 24
	// Test setup: Create a test file log
	log, err := NewFileLog(FileLogConfig{
		Dir:            t.TempDir(),
		RotationPeriod: testRotationPeriod,
		Clock:          clock,
	})
	require.NoError(t, err)

	// Test setup: Crafting fixtures.
	// Truncating by an hour to avoid an awkward low probability case when the
	// time is a minute away from EOD and adding a few seconds to events causes
	// a day change and a flaky test.
	startupTime := clock.Now().UTC().Truncate(time.Hour)

	// The timeframe is 24 hours to match FileLog rotation and the event exporter's default
	// Adding a 2 minutes offset to be sure we are always strictly between the
	// time frame boundaries.
	beforeTimeFrame := startupTime.Add(testRotationPeriod).Add(time.Minute * 2)
	inTimeFrame := beforeTimeFrame.Add(testRotationPeriod)
	afterTimeFrame := inTimeFrame.Add(testRotationPeriod)

	from := inTimeFrame.Truncate(testRotationPeriod)
	to := from.Add(testRotationPeriod)

	eventsBeforeTimeFrame := []events.AuditEvent{
		&events.Exec{Metadata: events.Metadata{Time: beforeTimeFrame, ID: uuid.NewString()}},
	}

	eventsInTimeFrame := []events.AuditEvent{
		&events.Exec{Metadata: events.Metadata{Time: inTimeFrame.Add(time.Second), ID: uuid.NewString()}},
		&events.Exec{Metadata: events.Metadata{Time: inTimeFrame.Add(2 * time.Second), ID: uuid.NewString()}},
		&events.Exec{Metadata: events.Metadata{Time: inTimeFrame.Add(3 * time.Second), ID: uuid.NewString()}},
	}

	eventsAfterTimeFrame := []events.AuditEvent{
		&events.Exec{Metadata: events.Metadata{Time: afterTimeFrame, ID: uuid.NewString()}},
	}

	// Test setup: Loading fixtures.

	// set the clock to beforeTimeFrame
	clock.Advance(testRotationPeriod)
	for _, event := range eventsBeforeTimeFrame {
		require.NoError(t, log.EmitAuditEvent(t.Context(), event))
	}

	// set the clock to inTimeFrame
	clock.Advance(testRotationPeriod)
	for _, event := range eventsInTimeFrame {
		require.NoError(t, log.EmitAuditEvent(t.Context(), event))
	}

	// set the clock to afterTimeFrame
	clock.Advance(testRotationPeriod)
	for _, event := range eventsAfterTimeFrame {
		require.NoError(t, log.EmitAuditEvent(t.Context(), event))
	}

	tests := []struct {
		name           string
		checkpoint     string
		expectedEvents []events.AuditEvent
		eventOrder     types.EventOrder
	}{
		{
			name:           "no checkpoint (asc)",
			checkpoint:     "",
			eventOrder:     types.EventOrderAscending,
			expectedEvents: eventsInTimeFrame,
		},
		{
			name:           "no checkpoint (desc)",
			checkpoint:     "",
			eventOrder:     types.EventOrderDescending,
			expectedEvents: eventsInTimeFrame,
		},
		{
			name:       "checkpoint in range (asc)",
			checkpoint: fmt.Sprintf("%s/%d", eventsInTimeFrame[0].GetID(), inTimeFrame.UnixNano()),
			eventOrder: types.EventOrderAscending,
			// first event should be skipped because of the checkpoint
			expectedEvents: eventsInTimeFrame[1:],
		},
		{
			name:       "checkpoint in range (desc)",
			checkpoint: fmt.Sprintf("%s/%d", eventsInTimeFrame[2].GetID(), inTimeFrame.UnixNano()),
			eventOrder: types.EventOrderDescending,
			// last event should be skipped because of the checkpoint
			expectedEvents: eventsInTimeFrame[:2],
		},
		{
			name:       "checkpoint legacy format (asc)",
			checkpoint: eventsInTimeFrame[0].GetID(),
			eventOrder: types.EventOrderAscending,
			// first event should be skipped because of the checkpoint
			expectedEvents: eventsInTimeFrame[1:],
		},
		{
			name:       "checkpoint legacy format (desc)",
			checkpoint: eventsInTimeFrame[2].GetID(),
			eventOrder: types.EventOrderDescending,
			// last event should be skipped because of the checkpoint
			expectedEvents: eventsInTimeFrame[:2],
		},
		{
			name:           "checkpoint before range (asc)",
			eventOrder:     types.EventOrderAscending,
			checkpoint:     fmt.Sprintf("%s/%d", eventsBeforeTimeFrame[0].GetID(), beforeTimeFrame.UnixNano()),
			expectedEvents: eventsInTimeFrame,
		},
		{
			name:           "checkpoint after range (asc)",
			eventOrder:     types.EventOrderAscending,
			checkpoint:     fmt.Sprintf("%s/%d", eventsAfterTimeFrame[0].GetID(), afterTimeFrame.UnixNano()),
			expectedEvents: nil,
		},
		{
			name:           "checkpoint before range (desc)",
			eventOrder:     types.EventOrderDescending,
			checkpoint:     fmt.Sprintf("%s/%d", eventsBeforeTimeFrame[0].GetID(), beforeTimeFrame.UnixNano()),
			expectedEvents: nil,
		},
		{
			name:           "checkpoint after range (desc)",
			eventOrder:     types.EventOrderDescending,
			checkpoint:     fmt.Sprintf("%s/%d", eventsAfterTimeFrame[0].GetID(), afterTimeFrame.UnixNano()),
			expectedEvents: eventsInTimeFrame,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test execution: querying all events until we get an empty response
			remainingEvents := true
			checkpoint := tt.checkpoint
			var evts []events.AuditEvent
			for remainingEvents {
				e, c, err := log.SearchEvents(t.Context(), SearchEventsRequest{
					From:     from,
					To:       to,
					Limit:    1,
					Order:    tt.eventOrder,
					StartKey: checkpoint,
				})
				require.NoError(t, err)
				remainingEvents = c != ""
				evts = append(evts, e...)
				checkpoint = c
			}

			// Test validation: check if the checkpoint and event match the expected result
			expectedEventIDs := make(map[string]struct{}, len(tt.expectedEvents))
			for _, e := range tt.expectedEvents {
				expectedEventIDs[e.GetID()] = struct{}{}
			}

			gotEventIDs := make(map[string]struct{}, len(evts))
			for _, e := range evts {
				gotEventIDs[e.GetID()] = struct{}{}
			}
			require.Equal(t, expectedEventIDs, gotEventIDs)
		})
	}
}

func TestSearchSessionEvents(t *testing.T) {
	clock := clockwork.NewFakeClock()
	start := clock.Now()
	ctx := context.Background()

	log, err := NewFileLog(FileLogConfig{
		Dir:            t.TempDir(),
		RotationPeriod: time.Hour * 24,
		Clock:          clock,
	})
	require.NoError(t, err)
	clock.Advance(1 * time.Minute)

	require.NoError(t, log.EmitAuditEvent(ctx, &events.SessionEnd{
		Metadata: events.Metadata{
			ID:   "a",
			Type: SessionEndEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err := log.SearchSessionEvents(ctx, SearchSessionEventsRequest{
		From:  start,
		To:    clock.Now(),
		Limit: 10,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, SessionEndEvent, result[0].GetType())
	require.Equal(t, "a", result[0].GetID())

	// emit a non-session event, it should not show up in the next query
	require.NoError(t, log.EmitAuditEvent(ctx, &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "b",
			Type: SessionJoinEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err = log.SearchSessionEvents(ctx, SearchSessionEventsRequest{
		From:  start,
		To:    clock.Now(),
		Limit: 10,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, SessionEndEvent, result[0].GetType())
	require.Equal(t, "a", result[0].GetID())

	// emit a desktop session event, it should show up in the next query
	require.NoError(t, log.EmitAuditEvent(ctx, &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			ID:   "c",
			Type: WindowsDesktopSessionEndEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err = log.SearchSessionEvents(ctx, SearchSessionEventsRequest{
		From:  start,
		To:    clock.Now(),
		Limit: 10,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, SessionEndEvent, result[0].GetType())
	require.Equal(t, "a", result[0].GetID())
	require.Equal(t, WindowsDesktopSessionEndEvent, result[1].GetType())
	require.Equal(t, "c", result[1].GetID())
}

// TestLargeEvent test fileLog behavior in case of large events.
// If an event is serializable the FileLog handler should try to trim the event size.
func TestLargeEvent(t *testing.T) {
	type check func(t *testing.T, event []events.AuditEvent)

	hasEventsLength := func(n int) check {
		return func(t *testing.T, ee []events.AuditEvent) {
			require.Len(t, ee, n, "events length mismatch")
		}
	}
	hasEventsIDs := func(ids ...string) check {
		return func(t *testing.T, ee []events.AuditEvent) {
			want := make([]string, 0, len(ee))
			for _, v := range ee {
				want = append(want, v.GetID())
			}
			require.Equal(t, want, ids)
		}
	}

	largeMongoQuery, err := makeLargeMongoQuery()
	require.NoError(t, err)

	tests := []struct {
		name   string
		in     []events.AuditEvent
		checks []check
	}{
		{
			name: "event should be trimmed",
			in: []events.AuditEvent{
				makeQueryEvent("1", "select 1"),
				makeQueryEvent("2", strings.Repeat("A", bufio.MaxScanTokenSize)),
				makeQueryEvent("3", "select 3"),
				makeQueryEvent("4", largeMongoQuery),
			},
			checks: []check{
				hasEventsLength(4),
				hasEventsIDs("1", "2", "3", "4"),
			},
		},
		{
			name: "large event should not be emitted",
			in: []events.AuditEvent{
				makeQueryEvent("1", "select 1"),
				makeAccessRequestEvent("2", strings.Repeat("A", bufio.MaxScanTokenSize)),
				makeQueryEvent("3", "select 3"),
				makeQueryEvent("4", "select 4"),
			},
			checks: []check{
				hasEventsLength(3),
				hasEventsIDs("1", "3", "4"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clock := clockwork.NewFakeClockAt(time.Now())

			log, err := NewFileLog(FileLogConfig{
				Dir:              t.TempDir(),
				RotationPeriod:   time.Hour * 24,
				Clock:            clock,
				MaxScanTokenSize: bufio.MaxScanTokenSize,
			})
			require.NoError(t, err)
			start := time.Now()
			clock.Advance(1 * time.Minute)

			for _, v := range tc.in {
				v.SetTime(clock.Now().UTC())
				log.EmitAuditEvent(ctx, v)
			}
			events := mustSearchEvent(t, log, start)
			for _, ch := range tc.checks {
				ch(t, events)
			}
		})
	}
}

// makeLargeMongoQuery returns an example MongoDB query to test TrimToMaxSize when a
// query contains a lot of characters that need to be escaped. The additional
// escaping might push the message size over the limit even after being trimmed.
// The goal of to make this about as pathological a query as is possible so there
// are many very small string fields that will require quoting.
func makeLargeMongoQuery() (string, error) {
	record := map[string]string{"_id": `{"$oid":"63a0dd6da68baaeb828581fe"}`}
	for i := range 100 {
		t := fmt.Sprintf("%v", i)
		record[t] = t
	}

	out, err := json.Marshal(record)
	if err != nil {
		return "", err
	}

	return `OpMsg(Body={"insert": "books","ordered": true,"lsid": {"id": {"$binary":{"base64":"NX7MXcLdRi6pIT86e52k5A==","subType":"04"}}},"$db": "teleport"}, Documents=[` +
		strings.Repeat(string(out), 500) +
		`], Flags=)`, nil
}

func makeQueryEvent(id string, query string) *events.DatabaseSessionQuery {
	return &events.DatabaseSessionQuery{
		Metadata: events.Metadata{
			ID:   id,
			Type: DatabaseSessionQueryEvent,
		},
		DatabaseQuery: query,
	}
}

func makeAccessRequestEvent(id string, in string) *events.AccessRequestDelete {
	return &events.AccessRequestDelete{
		Metadata: events.Metadata{
			ID:   id,
			Type: DatabaseSessionQueryEvent,
		},
		RequestID: in,
	}
}

func mustSearchEvent(t *testing.T, log *FileLog, start time.Time) []events.AuditEvent {
	ctx := context.TODO()
	result, _, err := log.SearchEvents(ctx, SearchEventsRequest{
		From:  start,
		To:    start.Add(time.Hour),
		Limit: 100,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	return result
}
