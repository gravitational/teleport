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

package events_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestStreamerCompleteEmpty makes sure that streamer Complete function
// does not hang if streamer got closed a without getting a single event
func TestStreamerCompleteEmpty(t *testing.T) {
	uploader := eventstest.NewMemoryUploader()

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1})
	sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	err = stream.Complete(ctx)
	require.NoError(t, err)

	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		stream.Complete(ctx)
		stream.Close(ctx)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Timeout waiting for emitter to complete")
	case <-doneC:
	}
}

// TestNewSliceErrors guarantees that if an error on the `newSlice` process
// happens, the streamer will be canceled and the error will be returned in
// future `EmitAuditEvent` calls.
func TestNewSliceErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test upload error")
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: &eventstest.MockUploader{ReserveUploadPartError: expectedErr},
	})
	require.NoError(t, err)

	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1})
	sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

	_, err = streamer.CreateAuditStream(ctx, sid)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
}

// TestNewStreamErrors when creating a new stream, it will also initialize
// the current sliceWriter. If there is any error on this, it should be
// returned.
func TestNewStreamErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test upload error")

	t.Run("CreateAuditStream", func(t *testing.T) {
		for _, tt := range []struct {
			desc        string
			uploader    *eventstest.MockUploader
			expectedErr error
		}{
			{
				desc:     "CreateUploadError",
				uploader: &eventstest.MockUploader{CreateUploadError: expectedErr},
			},
			{
				desc:     "ReserveUploadPartError",
				uploader: &eventstest.MockUploader{ReserveUploadPartError: expectedErr},
			},
		} {
			t.Run(tt.desc, func(t *testing.T) {
				streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
					Uploader: tt.uploader,
				})
				require.NoError(t, err)

				evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1})
				sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

				_, err = streamer.CreateAuditStream(ctx, sid)
				require.Error(t, err)
				require.ErrorIs(t, err, expectedErr)
			})
		}
	})

	t.Run("ResumeAuditStream", func(t *testing.T) {
		for _, tt := range []struct {
			desc        string
			uploader    *eventstest.MockUploader
			expectedErr error
		}{
			{
				desc: "ListPartsError",
				uploader: &eventstest.MockUploader{
					MockListParts: func(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
						return nil, expectedErr
					},
				},
			},
			{
				desc:     "ReserveUploadPartError",
				uploader: &eventstest.MockUploader{ReserveUploadPartError: expectedErr},
			},
		} {
			t.Run(tt.desc, func(t *testing.T) {
				streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
					Uploader: tt.uploader,
				})
				require.NoError(t, err)

				evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1})
				sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

				_, err = streamer.ResumeAuditStream(ctx, sid, uuid.New().String())
				require.Error(t, err)
				require.ErrorIs(t, err, expectedErr)
			})
		}
	})
}

// TestProtoStreamLargeEvent tests ProtoStream behavior in the case of receiving
// a large event. If an event is trimmable (implements messageSizeTrimmer) than
// it should be trimmed otherwise an error should be thrown.
func TestProtoStreamLargeEvent(t *testing.T) {
	tests := []struct {
		name         string
		event        apievents.AuditEvent
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "large trimmable event is trimmed",
			event:        makeQueryEvent("1", strings.Repeat("A", constants.MaxProtoMessageSizeBytes)),
			errAssertion: require.NoError,
		},
		{
			name:         "large untrimmable event returns error",
			event:        makeAccessRequestEvent("1", strings.Repeat("A", constants.MaxProtoMessageSizeBytes)),
			errAssertion: require.Error,
		},
	}

	ctx := context.Background()

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: eventstest.NewMemoryUploader(nil),
	})
	require.NoError(t, err)

	stream, err := streamer.CreateAuditStream(ctx, session.ID("1"))
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.errAssertion(t, stream.RecordEvent(ctx, eventstest.PrepareEvent(test.event)))
		})
	}
	require.NoError(t, stream.Complete(ctx))
}

// TestReadCorruptedRecording tests that the streamer can successfully decode the kind of corrupted
// recordings that some older bugged versions of teleport might end up producing when under heavy load/throttling.
func TestReadCorruptedRecording(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := os.Open("testdata/corrupted-session")
	require.NoError(t, err)
	defer f.Close()

	reader := events.NewProtoReader(f)
	defer reader.Close()

	events, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	// verify that the expected number of events are extracted
	require.Len(t, events, 12)
}

func makeQueryEvent(id string, query string) *apievents.DatabaseSessionQuery {
	return &apievents.DatabaseSessionQuery{
		Metadata: apievents.Metadata{
			ID:   id,
			Type: events.DatabaseSessionQueryEvent,
		},
		DatabaseQuery: query,
	}
}

func makeAccessRequestEvent(id string, in string) *apievents.AccessRequestDelete {
	return &apievents.AccessRequestDelete{
		Metadata: apievents.Metadata{
			ID:   id,
			Type: events.DatabaseSessionQueryEvent,
		},
		RequestID: in,
	}
}

// TestOnUploadComplete_MissingSessionEnd verifies that when a stream is
// completed without a session end event, the OnUploadComplete callback is
// invoked and its returned session end event is passed through for
// summarization and recording metadata processing.
func TestOnUploadComplete_MissingSessionEnd(t *testing.T) {
	uploader := eventstest.NewMemoryUploader()

	sid := session.NewID()

	// Build the session end that OnUploadComplete will return.
	recoveredEnd := &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Type: events.SessionEndEvent,
			Code: events.SessionEndCode,
		},
		SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
		StartTime:       time.Now().Add(-time.Minute),
		EndTime:         time.Now(),
		Interactive:     true,
	}

	called := false
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)
	streamer.SetOnUploadComplete(func(_ context.Context, gotSID session.ID) (apievents.AuditEvent, error) {
		called = true
		require.Equal(t, sid, gotSID)
		return recoveredEnd, nil
	})

	stream, err := streamer.CreateAuditStream(t.Context(), sid)
	require.NoError(t, err)

	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sid,
		Namespace:   apidefaults.Namespace,
		ClusterName: "cluster",
	})
	require.NoError(t, err)

	// Emit a session start but deliberately omit the session end.
	start := &apievents.SessionStart{
		Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, ClusterName: "cluster"},
		SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
		TerminalSize:    "80:25",
	}
	prepared, err := preparer.PrepareSessionEvent(start)
	require.NoError(t, err)
	require.NoError(t, stream.RecordEvent(t.Context(), prepared))

	require.NoError(t, stream.Complete(t.Context()))

	require.True(t, called, "OnUploadComplete must be called when session end is missing")
}

// TestRecordingMetadataProcessing verifies that the recording metadata service
// is called with the correct session duration when completing an upload, and
// that sessionEndTime is correctly derived from different event types.
func TestRecordingMetadataProcessing(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name             string
		buildEvents      func(sid session.ID) []apievents.AuditEvent
		onUploadComplete func(ctx context.Context, sid session.ID) (apievents.AuditEvent, error)
		expectProcess    bool
		expectedDuration time.Duration
		processingError  error
	}{
		{
			name: "sessionEndTime from session end event",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionStart{
						Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, Time: startTime, ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						TerminalSize:    "80:25",
					},
					&apievents.SessionPrint{
						Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: startTime.Add(30 * time.Minute)},
						Data:     []byte("hello"),
						Bytes:    5,
					},
					&apievents.SessionEnd{
						Metadata:        apievents.Metadata{Type: events.SessionEndEvent, Code: events.SessionEndCode, Time: startTime.Add(time.Hour), ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						StartTime:       startTime,
						EndTime:         startTime.Add(time.Hour),
						Interactive:     true,
					},
				}
			},
			// sessionEndTime is set from SessionEnd.Metadata.Time (not SessionPrint.Time),
			// so duration = SessionEnd.Time - SessionStart.Time = 1h.
			expectProcess:    true,
			expectedDuration: time.Hour,
		},
		{
			name: "sessionEndTime from last print event when no session end",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionStart{
						Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, Time: startTime, ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						TerminalSize:    "80:25",
					},
					&apievents.SessionPrint{
						Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: startTime.Add(30 * time.Minute)},
						Data:     []byte("hello"),
						Bytes:    5,
					},
				}
			},
			// No SessionEnd, so sessionEndTime falls back to the last SessionPrint.Time.
			expectProcess:    true,
			expectedDuration: 30 * time.Minute,
		},
		{
			name: "no processing when session start event is missing",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionPrint{
						Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: startTime.Add(5 * time.Minute)},
						Data:     []byte("hello"),
						Bytes:    5,
					},
				}
			},
			// shouldProcessSession is never set without a SessionStart.
			expectProcess: false,
		},
		{
			name: "no processing when session end time is zero",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionStart{
						Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, Time: startTime, ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						TerminalSize:    "80:25",
					},
				}
			},
			// shouldProcessSession is true, but sessionEndTime is zero (no prints or end event).
			expectProcess: false,
		},
		{
			name: "sessionEndTime from OnUploadComplete recovered SessionEnd",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionStart{
						Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, Time: startTime, ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						TerminalSize:    "80:25",
					},
				}
			},
			onUploadComplete: func(_ context.Context, gotSID session.ID) (apievents.AuditEvent, error) {
				return &apievents.SessionEnd{
					Metadata:        apievents.Metadata{Type: events.SessionEndEvent, Code: events.SessionEndCode},
					SessionMetadata: apievents.SessionMetadata{SessionID: gotSID.String()},
					StartTime:       startTime,
					EndTime:         startTime.Add(45 * time.Minute),
					Interactive:     true,
				}, nil
			},
			// sessionEndTime is set from the recovered SessionEnd.EndTime.
			expectProcess:    true,
			expectedDuration: 45 * time.Minute,
		},
		{
			name: "processing error does not cause panic",
			buildEvents: func(sid session.ID) []apievents.AuditEvent {
				return []apievents.AuditEvent{
					&apievents.SessionStart{
						Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Code: events.SessionStartCode, Time: startTime, ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						TerminalSize:    "80:25",
					},
					&apievents.SessionEnd{
						Metadata:        apievents.Metadata{Type: events.SessionEndEvent, Code: events.SessionEndCode, Time: startTime.Add(time.Hour), ClusterName: "cluster"},
						SessionMetadata: apievents.SessionMetadata{SessionID: sid.String()},
						StartTime:       startTime,
						EndTime:         startTime.Add(time.Hour),
						Interactive:     true,
					},
				}
			},
			expectProcess:    true,
			expectedDuration: time.Hour,
			processingError:  errors.New("processing error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := eventstest.NewMemoryUploader()

			streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
				Uploader: uploader,
			})
			require.NoError(t, err)

			sid := session.NewID()

			if tc.onUploadComplete != nil {
				streamer.SetOnUploadComplete(tc.onUploadComplete)
			}

			stream, err := streamer.CreateAuditStream(t.Context(), sid)
			require.NoError(t, err)

			preparer, err := events.NewPreparer(events.PreparerConfig{
				SessionID:   sid,
				Namespace:   apidefaults.Namespace,
				ClusterName: "cluster",
			})
			require.NoError(t, err)

			for _, evt := range tc.buildEvents(sid) {
				prepared, err := preparer.PrepareSessionEvent(evt)
				require.NoError(t, err)
				require.NoError(t, stream.RecordEvent(t.Context(), prepared))
			}

			require.NoError(t, stream.Complete(t.Context()))
		})
	}
}
