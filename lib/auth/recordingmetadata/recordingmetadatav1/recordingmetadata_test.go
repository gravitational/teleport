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
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestProcessSessionRecording(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name               string
		events             []apievents.AuditEvent
		expectError        bool
		expectedThumbnails func(t *testing.T, thumbnailData []byte)
		expectedMetadata   func(t *testing.T, metadata *pb.SessionRecordingMetadata)
		expectedFrames     func(t *testing.T, frames []*pb.SessionRecordingThumbnail)
	}{
		{
			name:   "basic session with print events",
			events: generateBasicSession(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.Duration)
				require.Equal(t, int32(80), metadata.StartCols)
				require.Equal(t, int32(24), metadata.StartRows)
			},
			expectedFrames: func(t *testing.T, frames []*pb.SessionRecordingThumbnail) {
				require.NotEmpty(t, frames)
			},
			expectedThumbnails: func(t *testing.T, thumbnailData []byte) {
				var thumbnail pb.SessionRecordingThumbnail
				err := proto.Unmarshal(thumbnailData, &thumbnail)
				require.NoError(t, err)
				require.NotEmpty(t, thumbnail.Svg)
			},
		},
		{
			name:   "basic session with print events with immediate resize after start",
			events: generateBasicSessionWithImmediateResize(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.Duration)
				require.Equal(t, int32(100), metadata.StartCols)
				require.Equal(t, int32(30), metadata.StartRows)
			},
			expectedFrames: func(t *testing.T, frames []*pb.SessionRecordingThumbnail) {
				require.NotEmpty(t, frames)
			},
			expectedThumbnails: func(t *testing.T, thumbnailData []byte) {
				var thumbnail pb.SessionRecordingThumbnail
				err := proto.Unmarshal(thumbnailData, &thumbnail)
				require.NoError(t, err)
				require.NotEmpty(t, thumbnail.Svg)
			},
		},
		{
			name:   "session with resize events",
			events: generateSessionWithResize(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				var hasResize bool
				for _, event := range metadata.Events {
					if resize := event.GetResize(); resize != nil {
						hasResize = true
						require.Equal(t, int32(120), resize.Cols)
						require.Equal(t, int32(40), resize.Rows)
					}
				}
				require.True(t, hasResize, "expected resize event")
			},
		},
		{
			name:   "session with inactivity periods",
			events: generateSessionWithInactivity(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				var hasInactivity bool
				for _, event := range metadata.Events {
					if event.GetInactivity() != nil {
						hasInactivity = true
						require.NotNil(t, event.StartOffset)
						require.NotNil(t, event.EndOffset)
					}
				}
				require.True(t, hasInactivity, "expected inactivity event")
			},
		},
		{
			name:   "session with join and leave events",
			events: generateSessionWithJoinLeave(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				joinUsers := make(map[string]bool)
				for _, event := range metadata.Events {
					if join := event.GetJoin(); join != nil {
						joinUsers[join.User] = true
					}
				}
				require.True(t, joinUsers["alice"], "expected alice join event")
				require.True(t, joinUsers["bob"], "expected bob join event")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := session.NewID()

			streamer := &mockStreamer{
				events:       tt.events,
				errorOnEvent: -1,
			}
			uploadHandler := newMockUploadHandler()

			service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
				Streamer:      streamer,
				UploadHandler: uploadHandler,
			})
			require.NoError(t, err)

			ctx := context.Background()
			err = service.ProcessSessionRecording(ctx, sessionID)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				uploadHandler.mu.Lock()
				metadataData, ok := uploadHandler.metadata[string(sessionID)]
				uploadHandler.mu.Unlock()
				require.True(t, ok, "metadata should be uploaded")

				metadata, frames, err := unmarshalMetadata(metadataData)
				require.NoError(t, err)

				if tt.expectedMetadata != nil {
					tt.expectedMetadata(t, metadata)
				}

				if tt.expectedFrames != nil {
					tt.expectedFrames(t, frames)
				}

				uploadHandler.mu.Lock()
				thumbnailData, ok := uploadHandler.thumbnails[string(sessionID)]
				uploadHandler.mu.Unlock()
				if ok && tt.expectedThumbnails != nil {
					tt.expectedThumbnails(t, thumbnailData)
				}
			}
		})
	}
}

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

	ctx := context.Background()
	err = service.ProcessSessionRecording(ctx, sessionID)
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

	ctx := context.Background()
	err = service.ProcessSessionRecording(ctx, sessionID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload failed")
}

func TestProcessSessionRecording_ContextCancellation(t *testing.T) {
	sessionID := session.NewID()

	ctx, cancel := context.WithCancel(context.Background())
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
		processDone <- service.ProcessSessionRecording(ctx, sessionID)
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
	startTime := time.Now()
	sessionID := session.NewID()

	tests := []struct {
		name   string
		events []apievents.AuditEvent
	}{
		{
			name: "database session",
			events: []apievents.AuditEvent{
				&apievents.DatabaseSessionStart{
					Metadata: apievents.Metadata{
						Type: "db.session.start",
						Time: startTime,
					},
				},
			},
		},
		{
			name: "windows desktop session",
			events: []apievents.AuditEvent{
				&apievents.WindowsDesktopSessionStart{
					Metadata: apievents.Metadata{
						Type: "windows.desktop.session.start",
						Time: startTime,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamer := &mockStreamer{
				events:       tt.events,
				errorOnEvent: -1,
			}
			uploadHandler := newMockUploadHandler()

			service, err := NewRecordingMetadataService(RecordingMetadataServiceConfig{
				Streamer:      streamer,
				UploadHandler: uploadHandler,
			})
			require.NoError(t, err)

			ctx := context.Background()
			err = service.ProcessSessionRecording(ctx, sessionID)

			require.NoError(t, err)

			require.Empty(t, uploadHandler.metadata)
			require.Empty(t, uploadHandler.thumbnails)
		})
	}
}

func generateBasicSessionWithImmediateResize(startTime time.Time) []apievents.AuditEvent {
	events := generateBasicSession(startTime)

	// Insert a resize event immediately after session start
	resizeEvent := &apievents.Resize{
		Metadata: apievents.Metadata{
			Type: "resize",
			Time: startTime.Add(500 * time.Millisecond),
		},
		TerminalSize: "100:30",
	}

	events = append([]apievents.AuditEvent{events[0], resizeEvent}, events[1:]...)

	return events
}

func generateBasicSession(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Hello World\n"),
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(2 * time.Second),
			},
			Data: []byte("$ ls -la\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func generateSessionWithResize(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Initial output\n"),
		},
		&apievents.Resize{
			Metadata: apievents.Metadata{
				Type: "resize",
				Time: startTime.Add(2 * time.Second),
			},
			TerminalSize: "120:40",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(3 * time.Second),
			},
			Data: []byte("After resize\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func generateSessionWithInactivity(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Active\n"),
		},
		// 20 second gap
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(21 * time.Second),
			},
			Data: []byte("After inactivity\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(20 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(20 * time.Second),
		},
	}
	return events
}

func generateSessionWithJoinLeave(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionJoin{
			Metadata: apievents.Metadata{
				Type: "session.join",
				Time: startTime.Add(2 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "alice",
			},
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(3 * time.Second),
			},
			Data: []byte("Alice joined\n"),
		},
		&apievents.SessionJoin{
			Metadata: apievents.Metadata{
				Type: "session.join",
				Time: startTime.Add(4 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "bob",
			},
		},
		&apievents.SessionLeave{
			Metadata: apievents.Metadata{
				Type: "session.leave",
				Time: startTime.Add(6 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "alice",
			},
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func unmarshalMetadata(data []byte) (*pb.SessionRecordingMetadata, []*pb.SessionRecordingThumbnail, error) {
	reader := bytes.NewReader(data)

	metadata := &pb.SessionRecordingMetadata{}
	err := protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(reader, metadata)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var frames []*pb.SessionRecordingThumbnail

	for {
		frame := &pb.SessionRecordingThumbnail{}
		err := protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(reader, frame)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		frames = append(frames, frame)
	}

	return metadata, frames, nil
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
	if m.uploadError != nil {
		return "", m.uploadError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	path := "metadata/" + string(sessionID)
	m.metadata[string(sessionID)] = data
	m.metadataPaths[string(sessionID)] = path
	return path, nil
}

func (m *mockUploadHandler) UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	if m.uploadError != nil {
		return "", m.uploadError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	path := "thumbnail/" + string(sessionID)
	m.thumbnails[string(sessionID)] = data
	m.thumbnailPaths[string(sessionID)] = path
	return path, nil
}

func (m *mockUploadHandler) Download(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return nil
}

func (m *mockUploadHandler) DownloadSummary(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return nil
}

func (m *mockUploadHandler) DownloadMetadata(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return nil
}

func (m *mockUploadHandler) DownloadThumbnail(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return nil
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
