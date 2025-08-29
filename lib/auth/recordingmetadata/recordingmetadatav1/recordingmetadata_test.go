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
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestProcessSessionRecording_Upload(t *testing.T) {
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
			name:   "kube session",
			events: generateKubernetesSession(startTime),
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
			name:   "desktop session",
			events: generateDesktopSession(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.Duration)
			},
			expectedFrames: func(t *testing.T, frames []*pb.SessionRecordingThumbnail) {
				require.Empty(t, frames)
			},
			expectedThumbnails: func(t *testing.T, thumbnailData []byte) {
				require.Empty(t, thumbnailData)
			},
		},
		{
			name:   "database session",
			events: generateDatabaseSession(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.Duration)
			},
			expectedFrames: func(t *testing.T, frames []*pb.SessionRecordingThumbnail) {
				require.Empty(t, frames)
			},
			expectedThumbnails: func(t *testing.T, thumbnailData []byte) {
				require.Empty(t, thumbnailData)
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

			err = service.ProcessSessionRecording(t.Context(), sessionID)

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

	err = service.ProcessSessionRecording(t.Context(), sessionID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload failed")
}

func generateBasicSession(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				ClusterName: "test-cluster",
				Type:        "session.start",
				Time:        startTime,
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				Protocol: "ssh",
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerHostname: "test-server",
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
