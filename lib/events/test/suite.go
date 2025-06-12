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

package test

import (
	"bytes"
	"context"
	"io"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/export"
	"github.com/gravitational/teleport/lib/session"
)

// UploadDownload tests uploads and downloads
func UploadDownload(t *testing.T, handler events.MultipartHandler) {
	val := "hello, how is it going? this is the uploaded file"
	id := session.NewID()
	_, err := handler.Upload(context.TODO(), id, bytes.NewBuffer([]byte(val)))
	require.NoError(t, err)

	f, err := os.CreateTemp("", string(id))
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(context.TODO(), id, f)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	data, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, string(data), val)
}

// DownloadNotFound tests handling of the scenario when download is not found
func DownloadNotFound(t *testing.T, handler events.MultipartHandler) {
	id := session.NewID()

	f, err := os.CreateTemp("", string(id))
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(context.TODO(), id, f)
	require.True(t, trace.IsNotFound(err))
}

// EventsSuite is a conformance test suite to verify external event backends
type EventsSuite struct {
	Log        events.AuditLogger
	Clock      clockwork.Clock
	QueryDelay time.Duration

	// SearchSessionEvensBySessionIDTimeout is used to specify timeout on query
	// in SearchSessionEvensBySessionID test case.
	SearchSessionEvensBySessionIDTimeout time.Duration
}

func (s *EventsSuite) EventExport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseTime := time.Now().UTC()

	// initial state should contain no chunks
	chunks := s.Log.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
		Date: timestamppb.New(baseTime),
	})

	require.False(t, chunks.Next())
	require.NoError(t, chunks.Done())

	names := []string{"bob", "jack", "daisy", "evan"}

	// create an initial set of events that should all end up in the same chunk
	for i, name := range names {
		err := s.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: name},
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.UserLoginEvent,
				Time: baseTime.Add(time.Duration(i)),
			},
		})
		require.NoError(t, err)
	}

	// wait for the events to be processed
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		chunks := s.Log.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
			Date: timestamppb.New(baseTime),
		})

		var chunkCount, eventCount int

		for chunks.Next() {
			chunkCount++

			events := s.Log.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
				Date:  timestamppb.New(baseTime),
				Chunk: chunks.Item().Chunk,
			})

			for events.Next() {
				eventCount++
			}
			assert.NoError(t, events.Done())
		}

		assert.NoError(t, chunks.Done())

		assert.Equal(t, 1, chunkCount)
		assert.Equal(t, 4, eventCount)
	}, 30*time.Second, 500*time.Millisecond)

	// add more events that should end up in a new chunk
	for i, name := range names {
		err := s.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: name},
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.UserLoginEvent,
				Time: baseTime.Add(time.Duration(i + 4)),
			},
		})
		require.NoError(t, err)
	}

	// wait for the events to be processed
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		chunks := s.Log.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
			Date: timestamppb.New(baseTime),
		})

		var chunkCount, eventCount int

		for chunks.Next() {
			chunkCount++

			events := s.Log.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
				Date:  timestamppb.New(baseTime),
				Chunk: chunks.Item().Chunk,
			})

			for events.Next() {
				eventCount++
			}
			assert.NoError(t, events.Done())
		}

		assert.NoError(t, chunks.Done())

		assert.Equal(t, 2, chunkCount)
		assert.Equal(t, 8, eventCount)
	}, 30*time.Second, 500*time.Millisecond)

	// generate a random chunk and verify that it is not found
	events := s.Log.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
		Date:  timestamppb.New(baseTime),
		Chunk: uuid.New().String(),
	})

	require.False(t, events.Next())
	require.True(t, trace.IsNotFound(events.Done()))

	// try a different day and verify that no chunks are found
	chunks = s.Log.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
		Date: timestamppb.New(baseTime.AddDate(0, 0, 1)),
	})

	require.False(t, chunks.Next())

	require.NoError(t, chunks.Done())

	// as a sanity check, try pulling events using the exporter helper (should be
	// equivalent to the above behavior)
	var exportedEvents atomic.Uint64
	var exporter *export.DateExporter
	var err error
	exporter, err = export.NewDateExporter(export.DateExporterConfig{
		Client: s.Log,
		Date:   baseTime,
		Export: func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
			exportedEvents.Add(1)
			return nil
		},
		OnIdle: func(ctx context.Context) {
			// only exporting extant events, so we can close as soon as we're caught up.
			exporter.Close()
		},
		Concurrency:  3,
		MaxBackoff:   time.Millisecond * 600,
		PollInterval: time.Millisecond * 200,
	})
	require.NoError(t, err)
	defer exporter.Close()

	select {
	case <-exporter.Done():
	case <-time.After(30 * time.Second):
		require.FailNow(t, "timeout waiting for exporter to finish")
	}

	require.Equal(t, uint64(8), exportedEvents.Load())
}

// EventPagination covers event search pagination.
func (s *EventsSuite) EventPagination(t *testing.T) {
	// This serves no special purpose except to make querying easier.
	baseTime := time.Now().UTC()

	names := []string{"bob", "jack", "daisy", "evan"}

	for i, name := range names {
		err := s.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: name},
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.UserLoginEvent,
				Time: baseTime.Add(time.Second * time.Duration(i)),
			},
		})
		require.NoError(t, err)
	}

	toTime := baseTime.Add(time.Hour)
	var arr []apievents.AuditEvent
	var err error
	var checkpoint string

	ctx := context.Background()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    100,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})

		assert.NoError(t, err)
		assert.Len(t, arr, 4)
		assert.Empty(t, checkpoint)
	}, 30*time.Second, 500*time.Millisecond)

	for _, name := range names {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Len(t, arr, 1)
		event, ok := arr[0].(*apievents.UserLogin)
		require.True(t, ok)
		require.Equal(t, name, event.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Empty(t, arr)
	}
	require.Empty(t, checkpoint)

	for _, i := range []int{0, 2} {
		nameA := names[i]
		nameB := names[i+1]
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    2,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Len(t, arr, 2)
		eventA, okA := arr[0].(*apievents.UserLogin)
		eventB, okB := arr[1].(*apievents.UserLogin)
		require.True(t, okA)
		require.True(t, okB)
		require.Equal(t, nameA, eventA.User)
		require.Equal(t, nameB, eventB.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Empty(t, arr)
	}
	require.Empty(t, checkpoint)

	for i := len(names) - 1; i >= 0; i-- {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderDescending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Len(t, arr, 1)
		event, ok := arr[0].(*apievents.UserLogin)
		require.True(t, ok)
		require.Equal(t, names[i], event.User)
	}
	if checkpoint != "" {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderDescending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Empty(t, arr)
	}
	require.Empty(t, checkpoint)

	// This serves no special purpose except to make querying easier.
	baseTime2 := time.Now().UTC().AddDate(0, 0, -2)

	for _, name := range names {
		err := s.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: name},
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.UserLoginEvent,
				Time: baseTime2,
			},
		})
		require.NoError(t, err)
	}

Outer:
	for i := 0; i < len(names); i++ {
		arr, checkpoint, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     baseTime2,
			To:       baseTime2.Add(time.Second),
			Limit:    1,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Len(t, arr, 1)
		event, ok := arr[0].(*apievents.UserLogin)
		require.True(t, ok)
		require.Equal(t, event.GetTime(), baseTime2)
		require.True(t, slices.Contains(names, event.User))

		for i, name := range names {
			if name == event.User {
				// delete name from list
				copy(names[i:], names[i+1:])
				names = names[:len(names)-1]
				continue Outer
			}
		}

		t.Fatalf("unexpected event: %#v", event)
	}
}

// SessionEventsCRUD covers session events
func (s *EventsSuite) SessionEventsCRUD(t *testing.T) {
	loginTime := s.Clock.Now().UTC()
	// Bob has logged in
	err := s.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
		Method:       events.LoginMethodSAML,
		Status:       apievents.Status{Success: true},
		UserMetadata: apievents.UserMetadata{User: "bob"},
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Type: events.UserLoginEvent,
			Time: loginTime,
		},
	})
	require.NoError(t, err)

	// For eventually consistent queries
	if s.QueryDelay != 0 {
		time.Sleep(s.QueryDelay)
	}

	var history []apievents.AuditEvent
	ctx := context.Background()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		history, _, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:  loginTime.Add(-1 * time.Hour),
			To:    loginTime.Add(time.Hour),
			Limit: 100,
			Order: types.EventOrderAscending,
		})
		assert.NoError(t, err)
		assert.Len(t, history, 1)
	}, 30*time.Second, 500*time.Millisecond)

	// start the session and emit data stream to it and wrap it up
	sessionID := session.NewID()

	// sessionStartTime must be greater than loginTime, because in search we assume
	// order.
	sessionStartTime := loginTime.Add(1 * time.Minute)
	err = s.Log.EmitAuditEvent(context.Background(), &apievents.SessionStart{
		Metadata: apievents.Metadata{
			ID:    uuid.NewString(),
			Time:  sessionStartTime,
			Index: 0,
			Type:  events.SessionStartEvent,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(sessionID),
		},
		UserMetadata: apievents.UserMetadata{
			Login: "bob",
		},
	})
	require.NoError(t, err)

	sessionEndTime := s.Clock.Now().Add(time.Hour).UTC()
	err = s.Log.EmitAuditEvent(context.Background(), &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			ID:    uuid.NewString(),
			Time:  sessionEndTime,
			Index: 4,
			Type:  events.SessionEndEvent,
		},
		UserMetadata: apievents.UserMetadata{
			Login: "bob",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(sessionID),
		},
		Participants: []string{"bob", "alice"},
	})
	require.NoError(t, err)

	// search for the session event.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		history, _, err = s.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:  s.Clock.Now().UTC().Add(-1 * time.Hour),
			To:    s.Clock.Now().UTC().Add(time.Hour),
			Limit: 100,
			Order: types.EventOrderAscending,
		})

		assert.NoError(t, err)
		assert.Len(t, history, 3)
	}, 30*time.Second, 500*time.Millisecond)

	require.Equal(t, events.SessionStartEvent, history[1].GetType())
	require.Equal(t, events.SessionEndEvent, history[2].GetType())

	history, _, err = s.Log.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
		From:  s.Clock.Now().UTC().Add(-1 * time.Hour),
		To:    s.Clock.Now().UTC().Add(2 * time.Hour),
		Limit: 100,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, history, 1)

	withParticipant := func(participant string) *types.WhereExpr {
		return &types.WhereExpr{Contains: types.WhereExpr2{
			L: &types.WhereExpr{Field: events.SessionParticipants},
			R: &types.WhereExpr{Literal: participant},
		}}
	}

	history, _, err = s.Log.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
		From:  s.Clock.Now().UTC().Add(-1 * time.Hour),
		To:    s.Clock.Now().UTC().Add(2 * time.Hour),
		Limit: 100,
		Order: types.EventOrderAscending,
		Cond:  withParticipant("alice"),
	})
	require.NoError(t, err)
	require.Len(t, history, 1)

	history, _, err = s.Log.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
		From:  s.Clock.Now().UTC().Add(-1 * time.Hour),
		To:    s.Clock.Now().UTC().Add(2 * time.Hour),
		Limit: 100,
		Order: types.EventOrderAscending,
		Cond:  withParticipant("cecile"),
	})
	require.NoError(t, err)
	require.Empty(t, history)

	history, _, err = s.Log.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
		From:  s.Clock.Now().UTC().Add(-1 * time.Hour),
		To:    sessionEndTime.Add(-time.Second),
		Limit: 100,
		Order: types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Empty(t, history)
}

func (s *EventsSuite) SearchSessionEventsBySessionID(t *testing.T) {
	now := time.Now().UTC()
	firstID := uuid.New().String()
	secondID := uuid.New().String()
	thirdID := uuid.New().String()
	for i, id := range []string{firstID, secondID, thirdID} {
		event := &apievents.WindowsDesktopSessionEnd{
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.WindowsDesktopSessionEndEvent,
				Code: events.DesktopSessionEndCode,
				Time: now.Add(time.Duration(i) * time.Second),
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: id,
			},
		}
		err := s.Log.EmitAuditEvent(context.Background(), event)
		require.NoError(t, err)
	}
	from := time.Time{}
	to := now.Add(10 * time.Second)

	// TODO(tobiaszheller): drop running SearchSessionEvents in gorouting and using select for cancelation
	// when ctx is propagated to search calls.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx := context.Background()
		events, _, err := s.Log.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
			From:      from,
			To:        to,
			Limit:     1000,
			Order:     types.EventOrderDescending,
			SessionID: secondID,
		})
		require.NoError(t, err)
		require.Len(t, events, 1)
		e, ok := events[0].(*apievents.WindowsDesktopSessionEnd)
		require.True(t, ok)
		require.Equal(t, e.GetSessionID(), secondID)
	}()

	queryTimeout := s.SearchSessionEvensBySessionIDTimeout
	if queryTimeout == 0 {
		queryTimeout = time.Second * 10
	}

	select {
	case <-time.After(queryTimeout):
		t.Fatalf("Search event query timeout after %s", queryTimeout)
	case <-done:
	}
}
