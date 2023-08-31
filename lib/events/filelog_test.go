/*
Copyright 2021 Gravitational, Inc.

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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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
	require.Nil(t, err)
	require.Len(t, eventArr, 1)
	require.Empty(t, checkpoint)
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
	require.Nil(t, err)
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
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")

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
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")

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
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")
	require.Equal(t, result[1].GetType(), WindowsDesktopSessionEndEvent)
	require.Equal(t, result[1].GetID(), "c")
}

// TestLargeEvent test fileLog behavior in case of large events.
// If an event is serializable the FileLog handler should try to trim the event size.
func TestLargeEvent(t *testing.T) {
	type check func(t *testing.T, event []events.AuditEvent)

	hasEventsLength := func(n int) check {
		return func(t *testing.T, ee []events.AuditEvent) {
			require.Equal(t, n, len(ee), "events length mismatch")
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
	for i := 0; i < 100; i++ {
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
