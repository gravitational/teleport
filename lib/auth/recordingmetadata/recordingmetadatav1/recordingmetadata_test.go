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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
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
