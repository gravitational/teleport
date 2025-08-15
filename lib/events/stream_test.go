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
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/summarizer"
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

// TestReadCorruptedRecording tests that the streamer can successfully decode the kind of corrupted
// recordings that some older bugged versions of teleport might end up producing when under heavy load/throttling.
func TestReadCorruptedRecording(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := os.Open("testdata/corrupted-session")
	require.NoError(t, err)
	defer f.Close()

	reader := events.NewProtoReader(f, nil)
	defer reader.Close()

	events, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	// verify that the expected number of events are extracted
	require.Len(t, events, 12)
}

func TestSummarization_SSH(t *testing.T) {
	cases := []struct {
		name               string
		useSummarizer      bool
		summarizationError error
	}{
		{
			name:          "noop summarizer",
			useSummarizer: false,
		},
		{
			name:          "successful summarization",
			useSummarizer: true,
		},
		{
			// Since the behavior differs only in logging, this test is here just to
			// make sure an error doesn't cause a panic.
			name:               "summarization error",
			useSummarizer:      true,
			summarizationError: errors.New("summarization error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summarizerProvider := &summarizer.SessionSummarizerProvider{}
			uploader := eventstest.NewMemoryUploader()
			streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
				Uploader:                  uploader,
				SessionSummarizerProvider: summarizerProvider,
			})
			require.NoError(t, err)

			evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1})
			sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

			mockSummarizer := &MockSummarizer{}
			if tc.useSummarizer {
				summarizerProvider.SetSummarizer(mockSummarizer)
				matchesSID := mock.MatchedBy(func(e *apievents.SessionEnd) bool {
					return e.GetSessionID() == string(sid)
				})
				mockSummarizer.
					On("SummarizeSSH", mock.Anything, matchesSID).
					Return(tc.summarizationError).
					Once()
			}

			stream, err := streamer.CreateAuditStream(t.Context(), sid)
			require.NoError(t, err)

			preparer, err := events.NewPreparer(events.PreparerConfig{
				SessionID:   sid,
				Namespace:   apidefaults.Namespace,
				ClusterName: "cluster",
			})
			require.NoError(t, err)

			for _, evt := range evts {
				preparedEvent, err := preparer.PrepareSessionEvent(evt)
				require.NoError(t, err)

				err = stream.RecordEvent(t.Context(), preparedEvent)
				require.NoError(t, err)
			}

			err = stream.Complete(t.Context())
			require.NoError(t, err)

			mockSummarizer.AssertExpectations(t)
		})
	}
}

func TestSummarization_Database(t *testing.T) {
	cases := []struct {
		name               string
		useSummarizer      bool
		summarizationError error
	}{
		{
			name:          "noop summarizer",
			useSummarizer: false,
		},
		{
			name:          "successful summarization",
			useSummarizer: true,
		},
		{
			// Since the behavior differs only in logging, this test is here just to
			// make sure an error doesn't cause a panic.
			name:               "summarization error",
			useSummarizer:      true,
			summarizationError: errors.New("summarization error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summarizerProvider := &summarizer.SessionSummarizerProvider{}
			uploader := eventstest.NewMemoryUploader()
			streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
				Uploader:                  uploader,
				SessionSummarizerProvider: summarizerProvider,
			})
			require.NoError(t, err)

			evts := eventstest.GenerateTestDBSession(eventstest.DBSessionParams{Queries: 1})
			sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

			mockSummarizer := &MockSummarizer{}
			if tc.useSummarizer {
				summarizerProvider.SetSummarizer(mockSummarizer)
				matchesSID := mock.MatchedBy(func(e *apievents.DatabaseSessionEnd) bool {
					return e.GetSessionID() == string(sid)
				})
				mockSummarizer.
					On("SummarizeDatabase", mock.Anything, matchesSID).
					Return(tc.summarizationError).
					Once()
			}

			stream, err := streamer.CreateAuditStream(t.Context(), sid)
			require.NoError(t, err)

			preparer, err := events.NewPreparer(events.PreparerConfig{
				SessionID:   sid,
				Namespace:   apidefaults.Namespace,
				ClusterName: "cluster",
			})
			require.NoError(t, err)

			for _, evt := range evts {
				preparedEvent, err := preparer.PrepareSessionEvent(evt)
				require.NoError(t, err)

				err = stream.RecordEvent(t.Context(), preparedEvent)
				require.NoError(t, err)
			}

			err = stream.Complete(t.Context())
			require.NoError(t, err)

			mockSummarizer.AssertExpectations(t)
		})
	}
}

// TestSummarization_Unknown tests summarizing unknown session types by
// simulating a situation where the stream picked up after the session end
// event.
func TestSummarization_Unknown(t *testing.T) {
	cases := []struct {
		name               string
		useSummarizer      bool
		summarizationError error
	}{
		{
			name:          "noop summarizer",
			useSummarizer: false,
		},
		{
			name:          "successful summarization",
			useSummarizer: true,
		},
		{
			// Since the behavior differs only in logging, this test is here just to
			// make sure an error doesn't cause a panic.
			name:               "summarization error",
			useSummarizer:      true,
			summarizationError: errors.New("summarization error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summarizerProvider := &summarizer.SessionSummarizerProvider{}
			uploader := eventstest.NewMemoryUploader()
			streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
				Uploader:                  uploader,
				SessionSummarizerProvider: summarizerProvider,
			})
			require.NoError(t, err)

			// The only event in the stream is a SessionLeave event, which occurs
			// after the session end.
			sid := session.ID("a9db3c23-ba28-4155-8d28-9a7cc8aeb22c")
			event := &apievents.SessionLeave{
				Metadata: apievents.Metadata{
					Index:       123,
					Type:        events.SessionLeaveEvent,
					ID:          "b2ac5c41-280c-4b8c-9a5d-f6dcc4e9d6ef",
					Code:        events.SessionLeaveCode,
					Time:        time.Now().UTC(),
					ClusterName: "foo",
				},
				UserMetadata: apievents.UserMetadata{
					User: "admin", Login: "alice", UserKind: apievents.UserKind_USER_KIND_HUMAN,
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID:        sid.String(),
					PrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerNamespace: "default",
					ServerID:        "e021b146-a383-4391-91c3-e5e31d6964d3", ServerHostname: "node-1",
					ServerVersion: teleport.Version,
				},
			}

			mockSummarizer := &MockSummarizer{}
			if tc.useSummarizer {
				summarizerProvider.SetSummarizer(mockSummarizer)
				mockSummarizer.
					On("SummarizeWithoutEndEvent", mock.Anything, sid).
					Return(tc.summarizationError).
					Once()
			}

			stream, err := streamer.CreateAuditStream(t.Context(), sid)
			require.NoError(t, err)

			preparer, err := events.NewPreparer(events.PreparerConfig{
				SessionID:   sid,
				Namespace:   apidefaults.Namespace,
				ClusterName: "cluster",
			})
			require.NoError(t, err)

			preparedEvent, err := preparer.PrepareSessionEvent(event)
			require.NoError(t, err)

			err = stream.RecordEvent(t.Context(), preparedEvent)
			require.NoError(t, err)

			err = stream.Complete(t.Context())
			require.NoError(t, err)

			mockSummarizer.AssertExpectations(t)
		})
	}
}

func TestPartHeader(t *testing.T) {
	cases := []struct {
		name               string
		partHeader         events.PartHeader
		expectedErr        error
		expectedPartHeader *events.PartHeader // if different than starting part
	}{
		{
			name: "v1 part header",
			partHeader: events.PartHeader{
				ProtoVersion: events.ProtoStreamV1,
				PartSize:     1234,
				PaddingSize:  4321,
				Flags:        events.ProtoStreamFlagEncrypted,
			},
			expectedErr: nil,
			expectedPartHeader: &events.PartHeader{
				ProtoVersion: events.ProtoStreamV1,
				PartSize:     1234,
				PaddingSize:  4321,
				// no flags
			},
		},
		{
			name: "v2 part header encrypted",
			partHeader: events.PartHeader{
				ProtoVersion: events.ProtoStreamV2,
				PartSize:     1234,
				PaddingSize:  4321,
				Flags:        events.ProtoStreamFlagEncrypted,
			},
			expectedErr:        nil,
			expectedPartHeader: nil,
		},
		{
			name: "v2 part header unencrypted",
			partHeader: events.PartHeader{
				ProtoVersion: events.ProtoStreamV2,
				PartSize:     1234,
				PaddingSize:  4321,
			},
			expectedErr:        nil,
			expectedPartHeader: nil,
		},
		{
			name: "invalid version",
			partHeader: events.PartHeader{
				ProtoVersion: 3,
				PartSize:     1234,
				PaddingSize:  4321,
				Flags:        events.ProtoStreamFlagEncrypted,
			},
			expectedErr:        trace.BadParameter("unsupported protocol version %v", 3),
			expectedPartHeader: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := bytes.NewBuffer(c.partHeader.Bytes())
			switch c.partHeader.ProtoVersion {
			case events.ProtoStreamV1:
				require.Equal(t, events.ProtoStreamV1PartHeaderSize, buf.Len())
			case events.ProtoStreamV2:
				require.Equal(t, events.ProtoStreamV2PartHeaderSize, buf.Len())
			}

			header, err := events.ParsePartHeader(buf)
			if c.expectedErr != nil {
				require.ErrorIs(t, err, c.expectedErr)
				return
			}
			expected := c.partHeader
			if c.expectedPartHeader != nil {
				expected = *c.expectedPartHeader
			}
			require.Equal(t, expected, header)
		})
	}
}

func TestEncryptedRecordingIO(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	uploader := eventstest.NewMemoryUploader()
	encryptedIO := &fakeEncryptedIO{}
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:  uploader,
		Encrypter: encryptedIO,
	})
	require.NoError(t, err)

	const eventCount = 10
	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: eventCount})
	sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sid,
		Namespace:   apidefaults.Namespace,
		ClusterName: "cluster",
	})
	require.NoError(t, err)

	for _, evt := range evts {
		preparedEvent, err := preparer.PrepareSessionEvent(evt)
		require.NoError(t, err)

		err = stream.RecordEvent(ctx, preparedEvent)
		require.NoError(t, err)
	}

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

	out := fakeWriterAt{
		buf: &bytes.Buffer{},
	}
	err = uploader.Download(ctx, sid, out)
	require.NoError(t, err)

	reader := events.NewProtoReader(out.buf, encryptedIO)

	decryptedEvents, err := reader.ReadAll(ctx)
	require.NoError(t, err)
	require.Len(t, decryptedEvents, eventCount+2)
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

type MockSummarizer struct {
	mock.Mock
}

func (m *MockSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	args := m.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (m *MockSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	args := m.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (m *MockSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

// encryptedIO is really just a reversible transform, so we fake encryption by encoding/decoding as hex
type fakeEncryptedIO struct {
	err error
}

type fakeEncrypter struct {
	inner  io.WriteCloser
	writer io.Writer
}

func (f *fakeEncrypter) Write(out []byte) (int, error) {
	return f.writer.Write(out)
}

func (f *fakeEncrypter) Close() error {
	return f.inner.Close()
}

func (f *fakeEncryptedIO) WithEncryption(ctx context.Context, writer io.WriteCloser) (io.WriteCloser, error) {
	hexWriter := hex.NewEncoder(writer)
	encrypter := &fakeEncrypter{
		inner:  writer,
		writer: hexWriter,
	}

	return encrypter, f.err
}

func (f *fakeEncryptedIO) WithDecryption(ctx context.Context, reader io.Reader) (io.Reader, error) {
	return hex.NewDecoder(reader), f.err
}

type fakeWriterAt struct {
	buf *bytes.Buffer
}

func (f fakeWriterAt) Write(p []byte) (int, error) {
	return f.buf.Write(p)
}

func (f fakeWriterAt) WriteAt(p []byte, offset int64) (int, error) {
	return f.Write(p)
}
