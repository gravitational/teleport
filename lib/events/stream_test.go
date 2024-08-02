// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
			event:        makeQueryEvent("1", strings.Repeat("A", events.MaxProtoMessageSizeBytes)),
			errAssertion: require.NoError,
		},
		{
			name:         "large untrimmable event returns error",
			event:        makeAccessRequestEvent("1", strings.Repeat("A", events.MaxProtoMessageSizeBytes)),
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
