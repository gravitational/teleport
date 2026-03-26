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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// ttyRecordingProcessor handles TTY-based sessions (SSH and Kubernetes). It tracks terminal state through a virtual
// terminal emulator to generate SVG thumbnails, and records resize events, user join/leave activity, and inactivity periods.
type ttyRecordingProcessor struct {
	baseRecordingProcessor
	activeUsers       map[string]time.Duration
	hasSeenPrintEvent bool
}

func newTTYRecordingProcessor(base baseRecordingProcessor) *ttyRecordingProcessor {
	base.thumbnailGenerator = newTTYThumbnailGenerator()

	return &ttyRecordingProcessor{
		baseRecordingProcessor: base,
		activeUsers:            make(map[string]time.Duration),
	}
}

func (t *ttyRecordingProcessor) handleEvent(evt apievents.AuditEvent) error {
	t.lastEvent = evt

	switch e := evt.(type) {
	case *apievents.SessionStart:
		return t.handleSessionStart(e)

	case *apievents.Resize:
		return t.handleResize(e)

	case *apievents.SessionPrint:
		return t.handleSessionPrint(e)

	case *apievents.SessionJoin:
		return t.handleSessionJoin(e)

	case *apievents.SessionLeave:
		return t.handleSessionLeave(e)

	case *apievents.SessionEnd:
		return t.handleSessionEnd(e)
	}

	return nil
}

func (t *ttyRecordingProcessor) handleSessionStart(evt *apievents.SessionStart) error {
	t.lastActivityTime = evt.Time
	t.startTime = evt.Time

	size, err := session.UnmarshalTerminalParams(evt.TerminalSize)
	if err != nil {
		return trace.Wrap(err, "parsing terminal size %q", evt.TerminalSize)
	}

	// store the initial terminal size, this is typically 80:24 and is resized immediately
	t.metadata.StartCols = int32(size.W)
	t.metadata.StartRows = int32(size.H)

	t.metadata.ClusterName = evt.ClusterName
	t.metadata.User = evt.User

	switch evt.Protocol {
	case events.EventProtocolSSH:
		t.metadata.ResourceName = evt.ServerHostname
		t.metadata.Type = pb.SessionRecordingType_SESSION_RECORDING_TYPE_SSH

	case events.EventProtocolKube:
		t.metadata.ResourceName = evt.KubernetesCluster
		t.metadata.Type = pb.SessionRecordingType_SESSION_RECORDING_TYPE_KUBERNETES
	}

	return t.thumbnailGenerator.handleEvent(evt)
}

func (t *ttyRecordingProcessor) handleResize(evt *apievents.Resize) error {
	size, err := session.UnmarshalTerminalParams(evt.TerminalSize)
	if err != nil {
		return trace.Wrap(err, "parsing terminal size %q", evt.TerminalSize)
	}

	// if we haven't seen a print event yet, update the starting size to the latest resize
	// this handles cases where the initial terminal size is not 80x24 and is resized immediately
	// before any output is printed
	if !t.hasSeenPrintEvent {
		t.metadata.StartCols = int32(size.W)
		t.metadata.StartRows = int32(size.H)
	}

	t.metadata.Events = append(t.metadata.Events, &pb.SessionRecordingEvent{
		StartOffset: durationpb.New(evt.Time.Sub(t.startTime)),
		Event: &pb.SessionRecordingEvent_Resize{
			Resize: &pb.SessionRecordingResizeEvent{
				Cols: int32(size.W),
				Rows: int32(size.H),
			},
		},
	})

	return t.thumbnailGenerator.handleEvent(evt)
}

func (t *ttyRecordingProcessor) handleSessionPrint(evt *apievents.SessionPrint) error {
	// mark that we've seen the first print event so we don't update the starting size anymore
	if !t.hasSeenPrintEvent {
		t.hasSeenPrintEvent = true
	}

	if !t.lastActivityTime.IsZero() && evt.Time.Sub(t.lastActivityTime) > inactivityThreshold {
		t.addInactivityEvent(t.lastActivityTime, evt.Time)
	}

	t.lastActivityTime = evt.Time

	if err := t.thumbnailGenerator.handleEvent(evt); err != nil {
		return trace.Wrap(err)
	}

	t.captureThumbnailIfNeeded(evt.Time)

	return nil
}

func (t *ttyRecordingProcessor) handleSessionJoin(evt *apievents.SessionJoin) error {
	t.activeUsers[evt.User] = evt.Time.Sub(t.startTime)

	return nil
}

func (t *ttyRecordingProcessor) handleSessionLeave(evt *apievents.SessionLeave) error {
	if joinTime, ok := t.activeUsers[evt.User]; ok {
		t.metadata.Events = append(t.metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: durationpb.New(joinTime),
			EndOffset:   durationpb.New(evt.Time.Sub(t.startTime)),
			Event: &pb.SessionRecordingEvent_Join{
				Join: &pb.SessionRecordingJoinEvent{
					User: evt.User,
				},
			},
		})

		delete(t.activeUsers, evt.User)
	}

	return nil
}

func (t *ttyRecordingProcessor) handleSessionEnd(evt *apievents.SessionEnd) error {
	if !t.lastActivityTime.IsZero() && evt.Time.Sub(t.lastActivityTime) > inactivityThreshold {
		t.addInactivityEvent(t.lastActivityTime, evt.Time)
	}

	t.captureThumbnailIfNeeded(evt.Time)

	return nil
}

func (t *ttyRecordingProcessor) addInactivityEvent(start, end time.Time) {
	inactivityStart := durationpb.New(start.Sub(t.startTime))
	inactivityEnd := durationpb.New(end.Sub(t.startTime))

	t.metadata.Events = append(t.metadata.Events, &pb.SessionRecordingEvent{
		StartOffset: inactivityStart,
		EndOffset:   inactivityEnd,
		Event: &pb.SessionRecordingEvent_Inactivity{
			Inactivity: &pb.SessionRecordingInactivityEvent{},
		},
	})
}

func (t *ttyRecordingProcessor) collect() (*pb.SessionRecordingMetadata, *pb.SessionRecordingThumbnail) {
	// Finish off any remaining activity events
	for user, userStartOffset := range t.activeUsers {
		t.metadata.Events = append(t.metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: durationpb.New(userStartOffset),
			EndOffset:   durationpb.New(t.lastEvent.GetTime().Sub(t.startTime)),
			Event: &pb.SessionRecordingEvent_Join{
				Join: &pb.SessionRecordingJoinEvent{
					User: user,
				},
			},
		})
	}

	t.metadata.Duration = durationpb.New(t.lastEvent.GetTime().Sub(t.startTime))
	t.metadata.StartTime = timestamppb.New(t.startTime)
	t.metadata.EndTime = timestamppb.New(t.lastEvent.GetTime())

	return t.metadata, t.thumbnail
}
