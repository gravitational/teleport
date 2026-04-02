//go:build desktop_access_rdp || rust_rdp_decoder

/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"image/png"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestDesktopRecordingProcessor(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name              string
		events            []apievents.AuditEvent
		expectedMetadata  func(t *testing.T, metadata *pb.SessionRecordingMetadata)
		expectedThumbnail func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail)
	}{
		{
			name:   "complete session populates metadata and produces thumbnail",
			events: generateCompleteDesktopSession(t, startTime, 800, 600, nil),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.StartTime)
				require.NotNil(t, metadata.EndTime)

				require.Equal(t, "test-cluster", metadata.ClusterName)
				require.Equal(t, "test-user", metadata.User)
				require.Equal(t, "test-desktop", metadata.ResourceName)
				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP, metadata.Type)
				require.Equal(t, 10*time.Second, metadata.Duration.AsDuration())
			},
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)
				require.NotEmpty(t, thumbnail.Png)

				_, err := png.Decode(bytes.NewReader(thumbnail.Png))
				require.NoError(t, err)

				require.Equal(t, int32(800), thumbnail.ScreenWidth)
				require.Equal(t, int32(600), thumbnail.ScreenHeight)
			},
		},
		{
			name: "cursor state is captured in thumbnail",
			events: generateCompleteDesktopSession(t, startTime, 800, 600, []apievents.AuditEvent{
				desktopFastPathEvent(t, startTime.Add(2*time.Second), rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed)),
				desktopFastPathEvent(t, startTime.Add(3*time.Second), rdpstatetest.BuildPointerPositionPDU(400, 300)),
			}),
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)

				require.True(t, thumbnail.CursorVisible)
				require.Equal(t, int32(400), thumbnail.CursorX)
				require.Equal(t, int32(300), thumbnail.CursorY)
			},
		},
		{
			name: "unhandled events are silently ignored",
			events: generateCompleteDesktopSession(t, startTime, 800, 600, []apievents.AuditEvent{
				&apievents.SessionJoin{Metadata: apievents.Metadata{Time: startTime.Add(2 * time.Second)}},
				&apievents.SessionLeave{Metadata: apievents.Metadata{Time: startTime.Add(3 * time.Second)}},
			}),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				require.Equal(t, "test-cluster", metadata.ClusterName)
				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP, metadata.Type)
			},
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)
				require.NotEmpty(t, thumbnail.Png)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastEventTime := tt.events[len(tt.events)-1].GetTime()
			duration := lastEventTime.Sub(startTime)

			processor := newTestDesktopProcessor(duration)
			defer processor.release()

			for _, evt := range tt.events {
				require.NoError(t, processor.handleEvent(evt))
			}

			metadata, thumbnail := processor.collect()

			if tt.expectedMetadata != nil {
				tt.expectedMetadata(t, metadata)
			}

			if tt.expectedThumbnail != nil {
				tt.expectedThumbnail(t, thumbnail)
			}
		})
	}
}

func TestDesktopRecordingProcessor_ThumbnailsWrittenToWriter(t *testing.T) {
	startTime := time.Now()
	events := generateCompleteDesktopSession(t, startTime, 800, 600, nil)

	lastEventTime := events[len(events)-1].GetTime()
	duration := lastEventTime.Sub(startTime)

	var buf bytes.Buffer
	processor := newRecordingProcessor(&nopCloser{&buf}, slog.Default(), recordingmetadata.SessionTypeDesktop, duration)
	defer processor.release()

	for _, evt := range events {
		require.NoError(t, processor.handleEvent(evt))
	}

	processor.collect()

	require.NotEmpty(t, buf.Bytes(), "thumbnails should have been written to the writer")
}

func newTestDesktopProcessor(duration time.Duration) recordingProcessor {
	return newRecordingProcessor(&nopCloser{io.Discard}, slog.Default(), recordingmetadata.SessionTypeDesktop, duration)
}

func generateCompleteDesktopSession(t *testing.T, startTime time.Time, width, height uint32, extraEvents []apievents.AuditEvent) []apievents.AuditEvent {
	t.Helper()

	events := []apievents.AuditEvent{
		desktopSessionStartEvent(startTime),
		desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), width, height),
		desktopFastPathEvent(t, startTime.Add(1*time.Second), rdpstatetest.BuildBitmapPDU(0, 0, 4, 2, rdpstatetest.RGB565White)),
	}

	events = append(events, extraEvents...)

	events = append(events,
		desktopFastPathEvent(t, startTime.Add(8*time.Second), rdpstatetest.BuildBitmapPDU(10, 10, 4, 2, rdpstatetest.RGB565Red)),
		desktopSessionEndEvent(startTime.Add(10*time.Second)),
	)

	return events
}

func desktopSessionStartEvent(eventTime time.Time) *apievents.WindowsDesktopSessionStart {
	return &apievents.WindowsDesktopSessionStart{
		Metadata: apievents.Metadata{
			ClusterName: "test-cluster",
			Time:        eventTime,
		},
		UserMetadata: apievents.UserMetadata{
			User: "test-user",
		},
		DesktopName: "test-desktop",
	}
}

func desktopSessionEndEvent(eventTime time.Time) *apievents.WindowsDesktopSessionEnd {
	return &apievents.WindowsDesktopSessionEnd{
		Metadata: apievents.Metadata{
			Time: eventTime,
		},
	}
}
