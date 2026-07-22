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
	"image"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

type desktopProcessor struct {
	baseRecordingProcessor
	gen               *desktopThumbnailGenerator
	thumbnailInterval time.Duration

	lastCursorX, lastCursorY uint16
	cursorInitialized        bool

	// The current run of repaints landing in new places, used to tell typing (new places, frame after
	// frame) from a clock or caret (one place repainted over and over).
	activityFootprint  []image.Rectangle
	activityFrames     int       // frames that changed a place not already in activityFootprint
	activityStartedAt  time.Time // first such frame in the run
	activityLastSeenAt time.Time // most recent such frame
}

func newDesktopProcessor(base baseRecordingProcessor, duration time.Duration) *desktopProcessor {
	gen := newDesktopThumbnailGenerator()
	base.thumbnailGenerator = gen

	return &desktopProcessor{
		baseRecordingProcessor: base,
		gen:                    gen,
		thumbnailInterval:      calculateThumbnailInterval(duration, maxThumbnails, desktopMinThumbnailInterval),
	}
}

func (d *desktopProcessor) handleEvent(evt apievents.AuditEvent) error {
	d.lastEvent = evt

	switch e := evt.(type) {
	case *apievents.WindowsDesktopSessionStart:
		return d.handleWindowsDesktopSessionStart(e)

	case *apievents.DesktopRecording:
		return d.handleDesktopRecording(e)

	case *apievents.WindowsDesktopSessionEnd:
		return d.handleWindowsDesktopSessionEnd(e)
	}

	return nil
}

func (d *desktopProcessor) handleWindowsDesktopSessionStart(evt *apievents.WindowsDesktopSessionStart) error {
	d.startTime = evt.GetTime()
	d.lastActivityTime = evt.GetTime()

	d.metadata.SetClusterName(evt.ClusterName)
	d.metadata.SetUser(evt.User)
	d.metadata.SetResourceName(evt.DesktopName)
	d.metadata.SetType(pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP)

	return nil
}

func (d *desktopProcessor) handleDesktopRecording(evt *apievents.DesktopRecording) error {
	if err := d.thumbnailGenerator.handleEvent(evt); err != nil {
		return trace.Wrap(err)
	}

	d.trackActivity(evt.GetTime())
	d.captureThumbnailIfNeeded(evt.GetTime(), d.thumbnailInterval)

	return nil
}

// trackActivity classifies the frame just handled as activity or incidental noise and, when it is activity,
// closes out any preceding inactivity gap and advances lastActivityTime.
func (d *desktopProcessor) trackActivity(eventTime time.Time) {
	fa := d.gen.consumeFrameActivity()
	if fa.screenW == 0 || fa.screenH == 0 {
		return
	}

	cursorMoved := d.cursorInitialized && (fa.cursorX != d.lastCursorX || fa.cursorY != d.lastCursorY)
	d.lastCursorX, d.lastCursorY = fa.cursorX, fa.cursorY
	d.cursorInitialized = true

	// Abandon a stale run so a later typing burst dates from its own first repaint, not earlier
	// incidental noise like a clock tick.
	if !d.activityLastSeenAt.IsZero() && eventTime.Sub(d.activityLastSeenAt) > inactivityThreshold {
		d.resetActivityRun()
	}

	addedLocation := false
	for _, r := range fa.regions {
		if !overlapsAny(d.activityFootprint, r) {
			d.activityFootprint = append(d.activityFootprint, r)
			addedLocation = true
		}
	}
	if addedLocation {
		if d.activityStartedAt.IsZero() {
			d.activityStartedAt = eventTime
		}
		d.activityLastSeenAt = eventTime
		d.activityFrames++
	}

	bigRepaint := fa.changedPixels >= desktopActivityMinPixels(fa.screenW, fa.screenH)
	typing := d.activityFrames >= desktopActivityMinLocations

	if !cursorMoved && !fa.mouseButton && !bigRepaint && !typing {
		// Incidental change (clock tick, blinking caret) confined to a static spot, not activity.
		return
	}

	// Date typing from its first repaint, not the keystroke that crossed the threshold, so the inactive
	// span ends when the user resumed.
	activityTime := eventTime
	if typing {
		activityTime = d.activityStartedAt
	}

	if !d.lastActivityTime.IsZero() && activityTime.Sub(d.lastActivityTime) > inactivityThreshold {
		d.addInactivityEvent(d.lastActivityTime, activityTime)
	}
	d.lastActivityTime = eventTime
	d.resetActivityRun()
}

func (d *desktopProcessor) resetActivityRun() {
	d.activityFootprint = d.activityFootprint[:0]
	d.activityFrames = 0
	d.activityStartedAt = time.Time{}
	d.activityLastSeenAt = time.Time{}
}

// overlapsAny reports whether r intersects any region in the footprint. Edge-adjacent repaints (e.g.
// consecutive glyphs while typing) don't intersect, so they count as new places.
func overlapsAny(footprint []image.Rectangle, r image.Rectangle) bool {
	for _, existing := range footprint {
		if existing.Overlaps(r) {
			return true
		}
	}
	return false
}

// desktopActivityMinPixels is the minimum changed-pixel area for a frame to count as activity on a screen of
// the given size, derived from desktopActivityAreaFraction. Clamped to at least 1px so tiny screens still have
// a positive threshold.
func desktopActivityMinPixels(screenW, screenH uint16) int {
	return max(1, int(float64(int(screenW)*int(screenH))*desktopActivityAreaFraction))
}

func (d *desktopProcessor) handleWindowsDesktopSessionEnd(evt *apievents.WindowsDesktopSessionEnd) error {
	if d.metadata.GetType() == pb.SessionRecordingType_SESSION_RECORDING_TYPE_UNSPECIFIED {
		d.metadata.SetClusterName(evt.ClusterName)
		d.metadata.SetUser(evt.User)
		d.metadata.SetResourceName(evt.DesktopName)
		d.metadata.SetType(pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP)
	}

	// Without the decoder, lastActivityTime never advances past the session start, so this would mark the whole
	// recording inactive even when the user was active.
	if d.gen.decoderAvailable() && !d.lastActivityTime.IsZero() && evt.GetTime().Sub(d.lastActivityTime) > inactivityThreshold {
		d.addInactivityEvent(d.lastActivityTime, evt.GetTime())
	}

	d.captureThumbnailIfNeeded(evt.GetTime(), d.thumbnailInterval)

	return nil
}

func (d *desktopProcessor) collect() (*pb.SessionRecordingMetadata, *pb.SessionRecordingThumbnail) {
	if d.lastEvent == nil {
		return nil, nil
	}

	d.metadata.SetDuration(durationpb.New(d.lastEvent.GetTime().Sub(d.startTime)))
	d.metadata.SetStartTime(timestamppb.New(d.startTime))
	d.metadata.SetEndTime(timestamppb.New(d.lastEvent.GetTime()))

	return d.metadata, d.thumbnail
}
