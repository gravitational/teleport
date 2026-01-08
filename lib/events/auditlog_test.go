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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestNew(t *testing.T) {
	alog := makeLog(t, clockwork.NewFakeClock())

	// Close twice.
	require.NoError(t, alog.Close())
	require.NoError(t, alog.Close())
}

// TestLogRotation makes sure that logs are rotated
// on the day boundary and symlinks are created and updated
func TestLogRotation(t *testing.T) {
	ctx := context.Background()
	start := time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)
	clock := clockwork.NewFakeClockAt(start)

	// create audit log, write a couple of events into it, close it
	alog := makeLog(t, clock)
	defer func() {
		require.NoError(t, alog.Close())
	}()

	for _, duration := range []time.Duration{0, time.Hour * 25} {
		// advance time and emit audit event
		now := start.Add(duration)
		clock.Advance(duration)

		// emit regular event:
		event := &apievents.Resize{
			Metadata:     apievents.Metadata{Type: "resize", Time: now},
			TerminalSize: "10:10",
		}
		err := alog.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
		logfile := alog.CurrentFile()

		// make sure that file has the same date as the event
		dt, err := events.ParseFileTime(filepath.Base(logfile))
		require.NoError(t, err)
		require.Equal(t, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), dt)

		// read back what's been written:
		bytes, err := os.ReadFile(logfile)
		require.NoError(t, err)
		contents, err := json.Marshal(event)
		contents = append(contents, '\n')
		require.NoError(t, err)
		require.Equal(t, string(bytes), string(contents))

		// read back the contents using symlink
		bytes, err = os.ReadFile(alog.CurrentFileSymlink())
		require.NoError(t, err)
		require.Equal(t, string(bytes), string(contents))

		found, _, err := alog.SearchEvents(ctx, events.SearchEventsRequest{
			From:  now.Add(-time.Hour),
			To:    now.Add(time.Hour),
			Order: types.EventOrderAscending,
		})
		require.NoError(t, err)
		require.Len(t, found, 1)

		foundUnstructured, _, err := alog.SearchUnstructuredEvents(ctx, events.SearchEventsRequest{
			From:  now.Add(-time.Hour),
			To:    now.Add(time.Hour),
			Order: types.EventOrderAscending,
		})
		require.NoError(t, err)
		require.Len(t, foundUnstructured, 1)
	}
}

func TestConcurrentStreaming(t *testing.T) {
	uploader := eventstest.NewMemoryUploader()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		Clock:         clockwork.NewFakeClock(),
		ServerID:      "remote",
		UploadHandler: uploader,
	})
	require.NoError(t, err)
	t.Cleanup(func() { alog.Close() })

	ctx := context.Background()
	sid := session.ID("abc123")

	// upload a bogus session so that we can try to stream its events
	// (this is not valid protobuf, so the stream is not expected to succeed)
	_, err = uploader.Upload(ctx, sid, io.NopCloser(strings.NewReader(`asdfasdfasdfasdfasdef`)))
	require.NoError(t, err)

	// run multiple concurrent streams, which forces the second one to wait
	// on the download that the first one started
	streams := 2
	errors := make(chan error, streams)
	for range streams {
		go func() {
			eventsC, errC := alog.StreamSessionEvents(ctx, sid, 0)
			for {
				select {
				case err := <-errC:
					errors <- err
				case _, ok := <-eventsC:
					if !ok {
						errors <- nil
						return
					}
				}
			}
		}()
	}

	// This test just verifies that the streamer does not panic when multiple
	// concurrent streams are waiting on the same download to complete.
	for range streams {
		<-errors
	}
}

func TestStreamSessionEvents(t *testing.T) {
	uploader := eventstest.NewMemoryUploader()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		Clock:         clockwork.NewFakeClock(),
		ServerID:      "remote",
		UploadHandler: uploader,
	})
	require.NoError(t, err)
	t.Cleanup(func() { alog.Close() })

	ctx := context.Background()
	sid := session.NewID()
	sessionEvents := []apievents.AuditEvent{
		&apievents.DatabaseSessionStart{
			Metadata: apievents.Metadata{
				Type:  events.DatabaseSessionStartEvent,
				Code:  events.DatabaseSessionStartCode,
				Index: 0,
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: sid.String(),
			},
		},
		&apievents.DatabaseSessionEnd{
			Metadata: apievents.Metadata{
				Type:  events.DatabaseSessionEndEvent,
				Code:  events.DatabaseSessionEndCode,
				Index: 1,
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: sid.String(),
			},
		},
	}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)
	for _, event := range sessionEvents {
		require.NoError(t, stream.RecordEvent(ctx, eventstest.PrepareEvent(event)))
	}
	require.NoError(t, stream.Complete(ctx))

	type callbackResult struct {
		event apievents.AuditEvent
		err   error
	}

	t.Run("Success", func(t *testing.T) {
		for name, withCallback := range map[string]bool{
			"WithCallback":    true,
			"WithoutCallback": false,
		} {
			t.Run(name, func(t *testing.T) {
				streamCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				callbackCh := make(chan callbackResult, 1)
				if withCallback {
					streamCtx = events.ContextWithSessionStartCallback(streamCtx, func(ae apievents.AuditEvent, err error) {
						callbackCh <- callbackResult{ae, err}
					})
				}

				ch, _ := alog.StreamSessionEvents(streamCtx, sid, 0)
				for _, event := range sessionEvents {
					select {
					case receivedEvent := <-ch:
						require.NotNil(t, receivedEvent)
						require.Equal(t, event.GetCode(), receivedEvent.GetCode())
						require.Equal(t, event.GetType(), receivedEvent.GetType())
					case <-time.After(10 * time.Second):
						require.Fail(t, "expected to receive session event %q but got nothing", event.GetType())
					}
				}

				if withCallback {
					select {
					case res := <-callbackCh:
						require.NoError(t, res.err)
						require.Equal(t, sessionEvents[0].GetCode(), res.event.GetCode())
						require.Equal(t, sessionEvents[0].GetType(), res.event.GetType())
					case <-time.After(10 * time.Second):
						require.Fail(t, "expected to receive callback result but got nothing")
					}
				}
			})
		}
	})

	t.Run("Error", func(t *testing.T) {
		for name, withCallback := range map[string]bool{
			"WithCallback":    true,
			"WithoutCallback": false,
		} {
			t.Run(name, func(t *testing.T) {
				streamCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				callbackCh := make(chan callbackResult, 1)
				if withCallback {
					streamCtx = events.ContextWithSessionStartCallback(streamCtx, func(ae apievents.AuditEvent, err error) {
						callbackCh <- callbackResult{ae, err}
					})
				}

				_, errCh := alog.StreamSessionEvents(streamCtx, session.ID("random"), 0)
				select {
				case err := <-errCh:
					require.Error(t, err)
				case <-time.After(10 * time.Second):
					require.Fail(t, "expected to get error while stream but got nothing")
				}

				if withCallback {
					select {
					case res := <-callbackCh:
						require.Error(t, res.err)
					case <-time.After(10 * time.Second):
						require.Fail(t, "expected to receive callback result but got nothing")
					}
				}
			})
		}
	})
}

func TestExternalLog(t *testing.T) {
	m := &eventstest.MockAuditLog{
		Emitter: &eventstest.MockRecorderEmitter{},
	}

	fakeClock := clockwork.NewFakeClock()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		Clock:         fakeClock,
		ServerID:      "remote",
		UploadHandler: eventstest.NewMemoryUploader(),
		ExternalLog:   m,
	})
	require.NoError(t, err)
	defer alog.Close()

	evt := &apievents.SessionConnect{}
	require.NoError(t, alog.EmitAuditEvent(context.Background(), evt))

	require.Len(t, m.Emitter.Events(), 1)
	require.Equal(t, m.Emitter.Events()[0], evt)
}

func TestUploadEncryptedRecording(t *testing.T) {
	ctx := t.Context()
	uploader := eventstest.NewMemoryUploader()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		ServerID:      "server1",
		UploadHandler: uploader,
	})
	require.NoError(t, err)
	defer alog.Close()

	parts := [][]byte{
		[]byte("123"),
		[]byte("456"),
		[]byte("789"),
	}
	partIter := func(yield func([]byte, error) bool) {
		for _, part := range parts {
			if part == nil {
				if !yield(nil, errors.New("invalid part")) {
					return
				}
			} else {
				if !yield(part, nil) {
					return
				}
			}
		}
	}
	sessionID, err := uuid.NewV7()
	require.NoError(t, err)

	err = alog.UploadEncryptedRecording(ctx, sessionID.String(), partIter)
	require.NoError(t, err)

	uploads, err := uploader.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, 1)

	uploaded, err := uploader.ListParts(ctx, uploads[0])
	require.NoError(t, err)

	require.Len(t, uploaded, len(parts))
	for idx, part := range uploaded {
		// uploaded part numbers should increment starting with 1
		require.Equal(t, int64(idx+1), part.Number)
	}
}

func makeLog(t *testing.T, clock clockwork.Clock) *events.AuditLog {
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		ServerID:      "server1",
		Clock:         clock,
		UIDGenerator:  utils.NewFakeUID(),
		UploadHandler: eventstest.NewMemoryUploader(),
	})
	require.NoError(t, err)

	return alog
}

func TestCallingSummarizerMetadata(t *testing.T) {
	ctx := t.Context()

	parts := generateParts(t)
	sessionID, err := uuid.NewV7()
	require.NoError(t, err)
	metadataProvider := recordingmetadata.NewProvider()
	recorderMetadata := &fakeRecordingMetadata{}
	recorderMetadata.On("ProcessSessionRecording", mock.Anything, session.ID(sessionID.String()), mock.Anything).
		Return(nil).Once()
	metadataProvider.SetService(recorderMetadata)

	summarizerProvider := summarizer.NewSessionSummarizerProvider()
	sessionSummarizer := &fakeSummarizer{}
	sessionSummarizer.On("SummarizeSSH", mock.Anything, mock.Anything).
		Return(nil).Once()
	summarizerProvider.SetSummarizer(sessionSummarizer)

	uploader := eventstest.NewMemoryUploader()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:                   t.TempDir(),
		ServerID:                  "server1",
		UploadHandler:             uploader,
		SessionSummarizerProvider: summarizerProvider,
		RecordingMetadataProvider: metadataProvider,
	})
	require.NoError(t, err)
	defer alog.Close()

	partIter := func(yield func([]byte, error) bool) {
		for _, part := range parts {
			if part == nil {
				if !yield(nil, errors.New("invalid part")) {
					return
				}
			} else {
				if !yield(part, nil) {
					return
				}
			}
		}
	}

	err = alog.UploadEncryptedRecording(ctx, sessionID.String(), partIter)
	require.NoError(t, err)

	recorderMetadata.AssertExpectations(t)
	sessionSummarizer.AssertExpectations(t)
}

type fakeRecordingMetadata struct {
	mock.Mock
}

func (f *fakeRecordingMetadata) ProcessSessionRecording(ctx context.Context, sessionID session.ID, duration time.Duration) error {
	args := f.Called(ctx, sessionID, duration)
	return args.Error(0)
}

type fakeSummarizer struct {
	mock.Mock
}

func (f *fakeSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	args := f.Called(ctx, sessionID)
	return args.Error(0)
}

func generateParts(t *testing.T) [][]byte {
	uploader := eventstest.NewMemoryUploader()

	ctx := t.Context()
	sid := session.NewID()
	sessionEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		PrintEvents: 1000,
		UserName:    "alice",
		SessionID:   string(sid),
		ServerID:    "testcluster",
		PrintData:   []string{"net", "stat"},
	})

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)
	for _, event := range sessionEvents {
		require.NoError(t, stream.RecordEvent(ctx, eventstest.PrepareEvent(event)))
	}
	require.NoError(t, stream.Complete(ctx))

	uploads, err := uploader.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, 1)
	parts, err := uploader.GetParts(uploads[0].ID)
	require.NoError(t, err)
	return parts
}

func TestPadUploadPart(t *testing.T) {
	partData := bytes.Repeat([]byte{1, 2, 3}, 10)
	partHeader := events.PartHeader{
		ProtoVersion: events.V2,
		PartSize:     uint64(len(partData)),
		PaddingSize:  0,
	}
	headerBytes := partHeader.Bytes()
	part := append(headerBytes, partData...)

	// Pad the upload part to double the size.
	minSize := len(part) * 2
	paddedPart := events.PadUploadPart(part, minSize)
	require.Len(t, paddedPart, minSize)

	// Padding the upload part again with the same minimum should add a single header in size.
	paddedPart = events.PadUploadPart(paddedPart, minSize)
	require.Len(t, paddedPart, minSize+events.ProtoStreamV2PartHeaderSize)

	// Ensure we can read out each part.
	r := bytes.NewReader(paddedPart)
	h1, err := events.ParsePartHeader(r)
	require.NoError(t, err)
	require.Equal(t, partHeader, h1)
	gotData, err := io.ReadAll(io.LimitReader(r, int64(h1.PartSize)))
	require.NoError(t, err)
	require.Equal(t, partData, gotData)
	io.Copy(io.Discard, io.LimitReader(r, int64(h1.PaddingSize)))

	h2, err := events.ParsePartHeader(r)
	require.NoError(t, err)
	require.Equal(t, events.PartHeader{
		ProtoVersion: events.V2,
		PaddingSize:  uint64(len(part) - events.ProtoStreamV2PartHeaderSize),
	}, h2)
	io.Copy(io.Discard, io.LimitReader(r, int64(h2.PaddingSize)))

	h3, err := events.ParsePartHeader(r)
	require.NoError(t, err)
	require.Equal(t, events.PartHeader{
		ProtoVersion: events.V2,
		PaddingSize:  0,
	}, h3)

	_, err = r.Read(nil)
	require.ErrorIs(t, err, io.EOF)
}
