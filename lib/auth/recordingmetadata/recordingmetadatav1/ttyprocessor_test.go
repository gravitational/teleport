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
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
)

func newTestTTYProcessor(duration time.Duration) recordingProcessor {
	return newRecordingProcessor(&nopCloser{io.Discard}, slog.Default(), recordingmetadata.SessionTypeTTY, duration)
}

func TestTTYRecordingProcessor(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name              string
		events            []apievents.AuditEvent
		expectedMetadata  func(t *testing.T, metadata *pb.SessionRecordingMetadata)
		expectedThumbnail func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail)
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
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)
				require.NotEmpty(t, thumbnail.Svg)
			},
		},
		{
			name:   "immediate resize before print updates start size",
			events: generateBasicSessionWithImmediateResize(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.Duration)
				require.Equal(t, int32(100), metadata.StartCols)
				require.Equal(t, int32(30), metadata.StartRows)
			},
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)
				require.NotEmpty(t, thumbnail.Svg)
			},
		},
		{
			name:   "resize events are recorded in metadata",
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
			name:   "inactivity periods are detected",
			events: generateSessionWithInactivity(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				var inactivityCount int
				for _, event := range metadata.Events {
					if event.GetInactivity() != nil {
						inactivityCount++

						require.Equal(t, 1*time.Second, event.StartOffset.AsDuration(), "inactivity should start at last activity time")
						require.Equal(t, 21*time.Second, event.EndOffset.AsDuration(), "inactivity should end when activity resumes")
					}
				}
				require.Equal(t, 1, inactivityCount, "expected exactly one inactivity event")
			},
		},
		{
			name:   "join and leave events are tracked",
			events: generateSessionWithJoinLeave(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)

				joins := make(map[string]*pb.SessionRecordingEvent)
				for _, event := range metadata.Events {
					if join := event.GetJoin(); join != nil {
						joins[join.User] = event
					}
				}

				require.Len(t, joins, 2, "expected join events for alice and bob")

				require.Equal(t, 2*time.Second, joins["alice"].StartOffset.AsDuration())
				require.Equal(t, 6*time.Second, joins["alice"].EndOffset.AsDuration())

				require.Equal(t, 4*time.Second, joins["bob"].StartOffset.AsDuration())
				require.Equal(t, 10*time.Second, joins["bob"].EndOffset.AsDuration())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastEventTime := tt.events[len(tt.events)-1].GetTime()
			duration := lastEventTime.Sub(startTime)

			processor := newTestTTYProcessor(duration)

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

func TestTTYRecordingProcessor_MalformedResizeEvent(t *testing.T) {
	startTime := time.Now()

	processor := newTestTTYProcessor(10 * time.Second)

	require.NoError(t, processor.handleEvent(sessionStartEvent(startTime, "80:24")))
	require.NoError(t, processor.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "Hello\n")))

	err := processor.handleEvent(resizeEvent(startTime.Add(2*time.Second), "invalid:terminal:size"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing terminal size")
}

func generateBasicSessionWithImmediateResize(startTime time.Time) []apievents.AuditEvent {
	events := generateBasicSession(startTime)

	// Insert a resize event immediately after session start
	events = append([]apievents.AuditEvent{events[0], resizeEvent(startTime.Add(500*time.Millisecond), "100:30")}, events[1:]...)

	return events
}

func generateBasicSession(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionPrintEvent(startTime.Add(1*time.Second), "Hello World\n"),
		sessionPrintEvent(startTime.Add(2*time.Second), "$ ls -la\n"),
		sessionEndEvent(startTime, startTime.Add(10*time.Second)),
	}
}

func generateSessionWithResize(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionPrintEvent(startTime.Add(1*time.Second), "Initial output\n"),
		resizeEvent(startTime.Add(2*time.Second), "120:40"),
		sessionPrintEvent(startTime.Add(3*time.Second), "After resize\n"),
		sessionEndEvent(startTime, startTime.Add(10*time.Second)),
	}
}

func generateSessionWithInactivity(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionPrintEvent(startTime.Add(1*time.Second), "Active\n"),
		// 20 second gap
		sessionPrintEvent(startTime.Add(21*time.Second), "After inactivity\n"),
		sessionEndEvent(startTime, startTime.Add(20*time.Second)),
	}
}

func generateSessionWithJoinLeave(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		sessionStartEvent(startTime, "80:24"),
		sessionJoinEvent(startTime.Add(2*time.Second), "alice"),
		sessionPrintEvent(startTime.Add(3*time.Second), "Alice joined\n"),
		sessionJoinEvent(startTime.Add(4*time.Second), "bob"),
		sessionLeaveEvent(startTime.Add(6*time.Second), "alice"),
		sessionEndEvent(startTime, startTime.Add(10*time.Second)),
	}
}
