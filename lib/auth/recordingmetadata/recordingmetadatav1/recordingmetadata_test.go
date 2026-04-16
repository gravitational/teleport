/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestProcessSessionRecording_StreamError(t *testing.T) {
	sessionID := session.NewID()

	streamer := &mockStreamerErrorOnly{
		err: errors.New("stream error"),
	}
	uploadHandler := newMockUploadHandler()

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(t.Context(), sessionID, recordingmetadata.SessionTypeTTY, 10*time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stream error")
}

func TestProcessSessionRecording_UploadError(t *testing.T) {
	startTime := time.Now()
	sessionID := session.NewID()

	streamer := &mockStreamer{
		events:       generateBasicSession(startTime),
		errorOnEvent: -1,
	}
	uploadHandler := newMockUploadHandler()
	uploadHandler.uploadError = errors.New("upload failed")

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(t.Context(), sessionID, recordingmetadata.SessionTypeTTY, 10*time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload failed")
}

func TestProcessSessionRecording_ContextCancellation(t *testing.T) {
	sessionID := session.NewID()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	streamer := newMockStreamerNeverSends()

	uploadHandler := newMockUploadHandler()

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	processDone := make(chan error, 1)
	go func() {
		processDone <- service.ProcessSessionRecording(ctx, sessionID, recordingmetadata.SessionTypeTTY, 10*time.Second)
	}()

	streamer.WaitUntilBlocking()

	cancel()

	select {
	case err := <-processDone:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out - ProcessSessionRecording did not exit after context cancellation")
	}
}

func TestProcessSessionRecording_UnsupportedSessionTypes(t *testing.T) {
	sessionID := session.NewID()

	streamer := &mockStreamer{
		errorOnEvent: -1,
	}
	uploadHandler := newMockUploadHandler()

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(t.Context(), sessionID, recordingmetadata.SessionTypeUnspecified, 10*time.Second)

	require.NoError(t, err)

	uploadHandler.mu.Lock()
	metadataLen := len(uploadHandler.metadata)
	thumbnailsLen := len(uploadHandler.thumbnails)
	uploadHandler.mu.Unlock()

	require.Equal(t, 0, metadataLen, "metadata should be empty")
	require.Equal(t, 0, thumbnailsLen, "thumbnails should be empty")
}

func TestProcessSessionRecording_MalformedResizeEvent(t *testing.T) {
	startTime := time.Now()
	sessionID := session.NewID()

	evts := []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionPrintEvent(startTime.Add(1*time.Second), "Hello\n"),
		resizeEvent(startTime.Add(2*time.Second), "invalid:terminal:size"),
		sessionEndEvent(startTime, startTime.Add(10*time.Second)),
	}

	streamer := &mockStreamer{
		events:       evts,
		errorOnEvent: -1,
	}
	uploadHandler := newMockUploadHandler()

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(t.Context(), sessionID, recordingmetadata.SessionTypeTTY, 10*time.Second)

	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing terminal size")

	uploadHandler.mu.Lock()
	defer uploadHandler.mu.Unlock()

	require.Empty(t, uploadHandler.metadata, "no metadata should be uploaded after cancelUpload")
}

func TestProcessSessionRecording_UploadFailsDuringProcessing(t *testing.T) {
	startTime := time.Now()
	sessionID := session.NewID()

	evts := []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionPrintEvent(startTime.Add(1*time.Second), "Hello\n"),
		sessionPrintEvent(startTime.Add(2*time.Second), "World\n"),
		sessionEndEvent(startTime, startTime.Add(10*time.Second)),
	}

	streamer := &mockStreamer{
		events:       evts,
		errorOnEvent: -1,
	}

	uploadHandler := &mockUploadHandlerFailAfterRead{
		failAfterBytes: 10,
	}

	service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
		Streamer:      streamer,
		UploadHandler: uploadHandler,
	})
	require.NoError(t, err)

	err = service.ProcessSessionRecording(t.Context(), sessionID, recordingmetadata.SessionTypeTTY, 10*time.Second)

	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated upload failure")
}

func sessionStartEvent(t time.Time, size string) *apievents.SessionStart {
	return &apievents.SessionStart{
		Metadata:     apievents.Metadata{Type: "session.start", Time: t},
		TerminalSize: size,
	}
}

func sessionPrintEvent(t time.Time, data string) *apievents.SessionPrint {
	return &apievents.SessionPrint{
		Metadata: apievents.Metadata{Type: "print", Time: t},
		Data:     []byte(data),
	}
}

func resizeEvent(t time.Time, size string) *apievents.Resize {
	return &apievents.Resize{
		Metadata:     apievents.Metadata{Type: "resize", Time: t},
		TerminalSize: size,
	}
}

func sessionEndEvent(startTime time.Time, endTime time.Time) *apievents.SessionEnd {
	return &apievents.SessionEnd{
		Metadata:  apievents.Metadata{Type: "session.end", Time: endTime},
		StartTime: startTime,
		EndTime:   endTime,
	}
}

func sessionJoinEvent(t time.Time, user string) *apievents.SessionJoin {
	return &apievents.SessionJoin{
		Metadata:     apievents.Metadata{Type: "session.join", Time: t},
		UserMetadata: apievents.UserMetadata{User: user},
	}
}

func sessionLeaveEvent(t time.Time, user string) *apievents.SessionLeave {
	return &apievents.SessionLeave{
		Metadata:     apievents.Metadata{Type: "session.leave", Time: t},
		UserMetadata: apievents.UserMetadata{User: user},
	}
}

// mockUploadHandlerFailAfterRead simulates an upload failure after reading some bytes
type mockUploadHandlerFailAfterRead struct {
	failAfterBytes int
	bytesRead      int
	mu             sync.Mutex
}

func (m *mockUploadHandlerFailAfterRead) UploadMetadata(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			// Drain the reader and return the context error
			_, _ = io.Copy(io.Discard, reader)
			return "", ctx.Err()
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			m.mu.Lock()
			m.bytesRead += n
			shouldFail := m.bytesRead >= m.failAfterBytes
			m.mu.Unlock()

			if shouldFail {
				_, _ = io.Copy(io.Discard, reader)
				return "", errors.New("simulated upload failure")
			}
		}

		if errors.Is(err, io.EOF) {
			return "metadata/success", nil
		}

		if err != nil {
			return "", err
		}
	}
}

func (m *mockUploadHandlerFailAfterRead) UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "thumbnail/success", nil
}

// mockStreamer implements player.Streamer for testing
type mockStreamer struct {
	events       []apievents.AuditEvent
	errorOnEvent int
	err          error
}

func (m *mockStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)
		defer close(errors)

		for i, evt := range m.events {
			if m.errorOnEvent == i {
				errors <- m.err
				return
			}
			select {
			case <-ctx.Done():
				return
			case events <- evt:
			}
		}
	}()

	return events, errors
}

// mockStreamerErrorOnly immediately sends an error without any events
type mockStreamerErrorOnly struct {
	err error
}

func (m *mockStreamerErrorOnly) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		errors <- m.err
	}()

	return events, errors
}

// mockStreamerNeverSends never sends any events - just blocks immediately
type mockStreamerNeverSends struct {
	called chan struct{}
}

func newMockStreamerNeverSends() *mockStreamerNeverSends {
	return &mockStreamerNeverSends{called: make(chan struct{})}
}

func (m *mockStreamerNeverSends) StreamSessionEvents(ctx context.Context, _ session.ID, _ int64) (chan apievents.AuditEvent, chan error) {
	evts := make(chan apievents.AuditEvent)
	errs := make(chan error, 1)

	close(m.called)

	go func() {
		<-ctx.Done()

		errs <- ctx.Err()

		close(errs)
	}()

	return evts, errs
}

func (m *mockStreamerNeverSends) WaitUntilBlocking() {
	<-m.called
}

// mockUploadHandler implements events.UploadHandler for testing
type mockUploadHandler struct {
	metadata       map[string][]byte
	thumbnails     map[string][]byte
	uploadError    error
	metadataPaths  map[string]string
	thumbnailPaths map[string]string
	mu             sync.Mutex
}

func newMockUploadHandler() *mockUploadHandler {
	return &mockUploadHandler{
		metadata:       make(map[string][]byte),
		thumbnails:     make(map[string][]byte),
		metadataPaths:  make(map[string]string),
		thumbnailPaths: make(map[string]string),
	}
}

func (m *mockUploadHandler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "", nil
}

func (m *mockUploadHandler) UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	return "", nil
}

func (m *mockUploadHandler) UploadMetadata(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	if m.uploadError != nil {
		return "", m.uploadError
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := "metadata/" + string(sessionID)
	m.metadata[string(sessionID)] = data
	m.metadataPaths[string(sessionID)] = path
	return path, nil
}

func (m *mockUploadHandler) UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	if m.uploadError != nil {
		return "", m.uploadError
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := "thumbnail/" + string(sessionID)
	m.thumbnails[string(sessionID)] = data
	m.thumbnailPaths[string(sessionID)] = path
	return path, nil
}

func (m *mockUploadHandler) Complete(ctx context.Context, upload events.StreamUpload) error {
	return nil
}

func (m *mockUploadHandler) Reserve(ctx context.Context, upload events.StreamUpload) error {
	return nil
}

func (m *mockUploadHandler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	return nil, nil
}

func (m *mockUploadHandler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	return nil, nil
}

func (m *mockUploadHandler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	return nil, nil
}
