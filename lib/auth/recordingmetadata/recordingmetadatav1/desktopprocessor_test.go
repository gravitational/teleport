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
			events: generateCompleteDesktopSession(t, startTime, 64, 48, nil),
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

				require.Equal(t, int32(64), thumbnail.ScreenWidth)
				require.Equal(t, int32(48), thumbnail.ScreenHeight)
			},
		},
		{
			name: "recovered session without start event populates metadata from end event",
			events: []apievents.AuditEvent{
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 64, 48),
				desktopFastPathEvent(t, startTime.Add(1*time.Second), rdpstatetest.BuildBitmapPDU(0, 0, 4, 2, rdpstatetest.RGB565White)),
				desktopRecoveredSessionEndEvent(startTime, startTime.Add(10*time.Second)),
			},
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.NotNil(t, metadata)
				require.NotNil(t, metadata.StartTime)
				require.NotNil(t, metadata.EndTime)

				require.True(t, metadata.StartTime.AsTime().Equal(startTime))
				require.Equal(t, "test-cluster", metadata.ClusterName)
				require.Equal(t, "test-user", metadata.User)
				require.Equal(t, "test-desktop", metadata.ResourceName)
				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP, metadata.Type)
				require.Equal(t, 10*time.Second, metadata.Duration.AsDuration())
			},
			expectedThumbnail: func(t *testing.T, thumbnail *pb.SessionRecordingThumbnail) {
				require.NotNil(t, thumbnail)
				require.NotEmpty(t, thumbnail.Png)
			},
		},
		{
			name: "unhandled events are silently ignored",
			events: generateCompleteDesktopSession(t, startTime, 64, 48, []apievents.AuditEvent{
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

			processor := newTestDesktopProcessor(startTime, duration)
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
	events := generateCompleteDesktopSession(t, startTime, 64, 48, nil)

	lastEventTime := events[len(events)-1].GetTime()
	duration := lastEventTime.Sub(startTime)

	var buf bytes.Buffer
	processor := newRecordingProcessor(&nopCloser{&buf}, slog.Default(), recordingmetadata.SessionTypeDesktop, startTime, duration)
	defer processor.release()

	for _, evt := range events {
		require.NoError(t, processor.handleEvent(evt))
	}

	processor.collect()

	require.NotEmpty(t, buf.Bytes(), "thumbnails should have been written to the writer")
}

func TestDesktopRecordingProcessor_InactivityEvents(t *testing.T) {
	startTime := time.Now()

	// bigFrame paints a 64x64 region (~4096px), far above the 256x256 activity threshold (~65px).
	bigFrame := func(at time.Time, color uint16) *apievents.DesktopRecording {
		return desktopFastPathEvent(t, at, rdpstatetest.BuildBitmapPDU(0, 0, 64, 64, color))
	}
	// smallFrame paints a 2x2 region (~4px) at a fixed position, far below the threshold: a ticking
	// clock or blinking caret that keeps repainting the same spot.
	smallFrame := func(at time.Time) *apievents.DesktopRecording {
		return desktopFastPathEvent(t, at, rdpstatetest.BuildBitmapPDU(0, 0, 2, 2, rdpstatetest.RGB565Red))
	}
	// typingFrame paints a 2x2 region at an advancing x offset: a glyph repaint marching across the
	// screen as the user types. Below the per-frame threshold, but always in a new place.
	typingFrame := func(at time.Time, x int) *apievents.DesktopRecording {
		return desktopFastPathEvent(t, at, rdpstatetest.BuildBitmapPDU(x, 0, 2, 2, rdpstatetest.RGB565Red))
	}

	type offsets struct{ start, end time.Duration }

	tests := []struct {
		name     string
		events   []apievents.AuditEvent
		expected []offsets
	}{
		{
			name: "idle gap between activity frames produces one inactivity event",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				bigFrame(startTime.Add(15*time.Second), rdpstatetest.RGB565Red),
				desktopSessionEndEvent(startTime.Add(16 * time.Second)),
			},
			expected: []offsets{{start: 1 * time.Second, end: 15 * time.Second}},
		},
		{
			name: "incidental small changes do not interrupt inactivity",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				smallFrame(startTime.Add(5 * time.Second)),
				smallFrame(startTime.Add(9 * time.Second)),
				smallFrame(startTime.Add(13 * time.Second)),
				desktopSessionEndEvent(startTime.Add(20 * time.Second)),
			},
			expected: []offsets{{start: 1 * time.Second, end: 20 * time.Second}},
		},
		{
			name: "consecutive activity frames produce no inactivity events",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				bigFrame(startTime.Add(5*time.Second), rdpstatetest.RGB565Red),
				bigFrame(startTime.Add(8*time.Second), rdpstatetest.RGB565White),
				desktopSessionEndEvent(startTime.Add(9 * time.Second)),
			},
			expected: nil,
		},
		{
			name: "cursor movement counts as activity; a no-op move does not",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				desktopMouseMoveEvent(t, startTime.Add(15*time.Second), 100, 100),
				desktopMouseMoveEvent(t, startTime.Add(30*time.Second), 100, 100),
				desktopSessionEndEvent(startTime.Add(40 * time.Second)),
			},
			expected: []offsets{
				{start: 1 * time.Second, end: 15 * time.Second},
				{start: 15 * time.Second, end: 40 * time.Second},
			},
		},
		{
			name: "session with no activity is fully inactive",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				smallFrame(startTime.Add(5 * time.Second)),
				smallFrame(startTime.Add(11 * time.Second)),
				desktopSessionEndEvent(startTime.Add(20 * time.Second)),
			},
			expected: []offsets{{start: 0, end: 20 * time.Second}},
		},
		{
			// Typing without the mouse reaches the processor only as small repaints that land in new
			// places frame after frame. Each is below the area threshold, but together they're activity.
			name: "continuous typing without mouse movement is not inactive",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				typingFrame(startTime.Add(1*time.Second), 0),
				typingFrame(startTime.Add(2*time.Second), 4),
				typingFrame(startTime.Add(3*time.Second), 8),
				typingFrame(startTime.Add(4*time.Second), 12),
				typingFrame(startTime.Add(5*time.Second), 16),
				typingFrame(startTime.Add(6*time.Second), 20),
				typingFrame(startTime.Add(7*time.Second), 24),
				typingFrame(startTime.Add(8*time.Second), 28),
				typingFrame(startTime.Add(9*time.Second), 32),
				typingFrame(startTime.Add(10*time.Second), 36),
				typingFrame(startTime.Add(11*time.Second), 40),
				typingFrame(startTime.Add(12*time.Second), 44),
				desktopSessionEndEvent(startTime.Add(13 * time.Second)),
			},
			expected: nil,
		},
		{
			// A taskbar clock repaints the same spot once per second while the user is away. It stays in
			// one place, so the whole span remains inactive.
			name: "clock ticking in a fixed position stays inactive",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				smallFrame(startTime.Add(3 * time.Second)),
				smallFrame(startTime.Add(4 * time.Second)),
				smallFrame(startTime.Add(5 * time.Second)),
				smallFrame(startTime.Add(6 * time.Second)),
				smallFrame(startTime.Add(7 * time.Second)),
				smallFrame(startTime.Add(8 * time.Second)),
				smallFrame(startTime.Add(9 * time.Second)),
				smallFrame(startTime.Add(10 * time.Second)),
				smallFrame(startTime.Add(11 * time.Second)),
				smallFrame(startTime.Add(12 * time.Second)),
				smallFrame(startTime.Add(13 * time.Second)),
				desktopSessionEndEvent(startTime.Add(14 * time.Second)),
			},
			expected: []offsets{{start: 1 * time.Second, end: 14 * time.Second}},
		},
		{
			// Typing resumes after a long idle. The inactive span must end when the user resumed (20s),
			// not a few keystrokes later when the run crosses the threshold.
			name: "typing after idle ends the inactive span at the first repaint",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				typingFrame(startTime.Add(20*time.Second), 0),
				typingFrame(startTime.Add(22*time.Second), 4),
				typingFrame(startTime.Add(24*time.Second), 8),
				typingFrame(startTime.Add(26*time.Second), 12),
				desktopSessionEndEvent(startTime.Add(30 * time.Second)),
			},
			expected: []offsets{{start: 1 * time.Second, end: 20 * time.Second}},
		},
		{
			// A clock ticks once early in a long idle, then typing resumes much later. The stale tick must
			// not be folded into the run, or the span would end at the tick (3s) not at the typing (20s).
			name: "clock tick during idle does not shorten a later typing run's inactive span",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				smallFrame(startTime.Add(3 * time.Second)),
				typingFrame(startTime.Add(20*time.Second), 10),
				typingFrame(startTime.Add(21*time.Second), 14),
				typingFrame(startTime.Add(22*time.Second), 18),
				typingFrame(startTime.Add(23*time.Second), 22),
				desktopSessionEndEvent(startTime.Add(30 * time.Second)),
			},
			expected: []offsets{{start: 1 * time.Second, end: 20 * time.Second}},
		},
		{
			// A mouse click is recorded as input but doesn't move the cursor or repaint; it must still
			// count as activity.
			name: "mouse click without a repaint counts as activity",
			events: []apievents.AuditEvent{
				desktopSessionStartEvent(startTime),
				desktopServerHelloEvent(t, startTime.Add(100*time.Millisecond), 256, 256),
				bigFrame(startTime.Add(1*time.Second), rdpstatetest.RGB565White),
				desktopMouseButtonEvent(t, startTime.Add(15*time.Second)),
				desktopSessionEndEvent(startTime.Add(40 * time.Second)),
			},
			expected: []offsets{
				{start: 1 * time.Second, end: 15 * time.Second},
				{start: 15 * time.Second, end: 40 * time.Second},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastEventTime := tt.events[len(tt.events)-1].GetTime()
			processor := newTestDesktopProcessor(startTime, lastEventTime.Sub(startTime))
			defer processor.release()

			for _, evt := range tt.events {
				require.NoError(t, processor.handleEvent(evt))
			}

			metadata, _ := processor.collect()
			require.NotNil(t, metadata)

			got := inactivityEvents(metadata)
			require.Len(t, got, len(tt.expected))
			for i, want := range tt.expected {
				require.Equal(t, want.start, got[i].StartOffset.AsDuration(), "event %d start", i)
				require.Equal(t, want.end, got[i].EndOffset.AsDuration(), "event %d end", i)
			}
		})
	}
}

func TestDesktopThumbnailGenerator_ConsumeFrameActivity(t *testing.T) {
	startTime := time.Now()

	gen := newDesktopThumbnailGenerator()
	defer gen.release()

	// ServerHello initializes the decoder: dimensions are set, nothing has changed yet.
	require.NoError(t, gen.handleEvent(desktopServerHelloEvent(t, startTime, 256, 256)))

	fa := gen.consumeFrameActivity()
	require.Equal(t, uint16(256), fa.screenW)
	require.Equal(t, uint16(256), fa.screenH)
	require.Zero(t, fa.changedPixels)

	// A 32x32 bitmap update marks a changed region well above zero.
	require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(time.Second),
		rdpstatetest.BuildBitmapPDU(0, 0, 32, 32, rdpstatetest.RGB565White))))

	fa = gen.consumeFrameActivity()
	require.Positive(t, fa.changedPixels)

	// consumeFrameActivity reset the accumulator, so a subsequent call with no new frame reports zero.
	fa = gen.consumeFrameActivity()
	require.Zero(t, fa.changedPixels)
}

func newTestDesktopProcessor(startTime time.Time, duration time.Duration) recordingProcessor {
	return newRecordingProcessor(&nopCloser{io.Discard}, slog.Default(), recordingmetadata.SessionTypeDesktop, startTime, duration)
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

func desktopRecoveredSessionEndEvent(sessionStart, eventTime time.Time) *apievents.WindowsDesktopSessionEnd {
	return &apievents.WindowsDesktopSessionEnd{
		Metadata: apievents.Metadata{
			ClusterName: "test-cluster",
			Time:        eventTime,
		},
		UserMetadata: apievents.UserMetadata{
			User: "test-user",
		},
		DesktopName: "test-desktop",
		StartTime:   sessionStart,
	}
}

// inactivityEvents returns just the inactivity events from the metadata, in order.
func inactivityEvents(metadata *pb.SessionRecordingMetadata) []*pb.SessionRecordingEvent {
	var out []*pb.SessionRecordingEvent
	for _, e := range metadata.Events {
		if e.GetInactivity() != nil {
			out = append(out, e)
		}
	}
	return out
}

// desktopMouseMoveEvent builds a DesktopRecording carrying a cursor move to (x, y) at eventTime. A mouse move
// repositions the cursor without painting the framebuffer, so it produces no changed-region area.
func desktopMouseMoveEvent(t *testing.T, eventTime time.Time, x, y uint32) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.LegacyMouseMove(x, y)
	require.NoError(t, err)

	evt.Metadata.Time = eventTime

	return evt
}

// desktopMouseButtonEvent builds a DesktopRecording carrying a mouse button press at eventTime.
func desktopMouseButtonEvent(t *testing.T, eventTime time.Time) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBMouseButton(true)
	require.NoError(t, err)

	evt.Metadata.Time = eventTime

	return evt
}
