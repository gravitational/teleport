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
	"time"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

type sessionProcessor struct {
	sessionID        session.ID
	startTime        time.Time
	lastEvent        apievents.AuditEvent
	lastActivityTime time.Time
	activeUsers      map[string]time.Duration
	vt               vt10x.Terminal
	metadata         *pb.SessionRecordingMetadata
	sampler          *thumbnailBucketSampler
}

func newSessionProcessor(sessionID session.ID) *sessionProcessor {
	return &sessionProcessor{
		sessionID:   sessionID,
		activeUsers: make(map[string]time.Duration),
		vt:          vt10x.New(),
		metadata:    &pb.SessionRecordingMetadata{},
		sampler:     newThumbnailBucketSampler(maxThumbnails, 1*time.Second),
	}
}

func (p *sessionProcessor) processEventStream(ctx context.Context, evts <-chan apievents.AuditEvent, errors <-chan error) error {
	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				return p.verifyEventsFound()
			}

			p.lastEvent = evt

			if err := p.handleEvent(evt); err != nil {
				return trace.Wrap(err)
			}

		case err, ok := <-errors:
			if err != nil {
				return trace.Wrap(err)
			}
			if !ok {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *sessionProcessor) handleEvent(evt apievents.AuditEvent) error {
	switch e := evt.(type) {
	case *apievents.DatabaseSessionStart:
		p.handleDatabaseSessionStart(e)

	case *apievents.WindowsDesktopSessionStart:
		p.handleWindowsDesktopSessionStart(e)

	case *apievents.SessionStart:
		return p.handleSessionStart(e)

	case *apievents.Resize:
		return p.handleResize(e)

	case *apievents.SessionPrint:
		return p.handleSessionPrint(e)

	case *apievents.SessionJoin:
		p.handleSessionJoin(e)

	case *apievents.SessionLeave:
		p.handleSessionLeave(e)

	case *apievents.DatabaseSessionEnd:
		return p.handleSessionEnd(e.EndTime, false /* thumbnail */)

	case *apievents.WindowsDesktopSessionEnd:
		return p.handleSessionEnd(e.EndTime, false /* thumbnail */)

	case *apievents.SessionEnd:
		return p.handleSessionEnd(e.EndTime, true /* thumbnail */)
	}

	return nil
}

func (p *sessionProcessor) handleDatabaseSessionStart(e *apievents.DatabaseSessionStart) {
	p.setCommonMetadata(e.Time, e.ClusterName, e.DatabaseUser, e.DatabaseName, pb.SessionRecordingType_SESSION_RECORDING_TYPE_DATABASE)
}

func (p *sessionProcessor) handleWindowsDesktopSessionStart(e *apievents.WindowsDesktopSessionStart) {
	p.setCommonMetadata(e.Time, e.ClusterName, e.WindowsUser, e.DesktopName, pb.SessionRecordingType_SESSION_RECORDING_TYPE_DESKTOP)
}

func (p *sessionProcessor) handleSessionStart(e *apievents.SessionStart) error {
	var resourceName string
	var recordingType pb.SessionRecordingType
	switch e.Protocol {
	case events.EventProtocolSSH:
		resourceName = e.ServerHostname
		recordingType = pb.SessionRecordingType_SESSION_RECORDING_TYPE_SSH
	case events.EventProtocolKube:
		resourceName = e.KubernetesCluster
		recordingType = pb.SessionRecordingType_SESSION_RECORDING_TYPE_KUBERNETES
	}

	p.setCommonMetadata(e.Time, e.ClusterName, e.User, resourceName, recordingType)

	size, err := session.UnmarshalTerminalParams(e.TerminalSize)
	if err != nil {
		return trace.Wrap(err, "parsing terminal size %q for session %v", e.TerminalSize, p.sessionID)
	}

	p.metadata.StartCols = int32(size.W)
	p.metadata.StartRows = int32(size.H)

	p.vt.Resize(size.W, size.H)

	return nil
}

func (p *sessionProcessor) handleResize(e *apievents.Resize) error {
	size, err := session.UnmarshalTerminalParams(e.TerminalSize)
	if err != nil {
		return trace.Wrap(err, "parsing terminal size %q for session %v", e.TerminalSize, p.sessionID)
	}

	p.metadata.Events = append(p.metadata.Events, &pb.SessionRecordingEvent{
		StartOffset: durationpb.New(e.Time.Sub(p.startTime)),
		Event: &pb.SessionRecordingEvent_Resize{
			Resize: &pb.SessionRecordingResizeEvent{
				Cols: int32(size.W),
				Rows: int32(size.H),
			},
		},
	})

	p.vt.Resize(size.W, size.H)

	return nil
}

func (p *sessionProcessor) handleSessionPrint(e *apievents.SessionPrint) error {
	if !p.lastActivityTime.IsZero() && e.Time.Sub(p.lastActivityTime) > inactivityThreshold {
		p.addInactivityEvent(p.lastActivityTime, e.Time)
	}

	if _, err := p.vt.Write(e.Data); err != nil {
		return trace.Errorf("writing data to terminal: %w", err)
	}

	if p.sampler.shouldCapture(e.Time) {
		p.recordThumbnail(e.Time)
	}

	p.lastActivityTime = e.Time
	return nil
}

func (p *sessionProcessor) handleSessionJoin(e *apievents.SessionJoin) {
	p.activeUsers[e.User] = e.Time.Sub(p.startTime)
}

func (p *sessionProcessor) handleSessionLeave(e *apievents.SessionLeave) {
	if joinTime, ok := p.activeUsers[e.User]; ok {
		p.metadata.Events = append(p.metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: durationpb.New(joinTime),
			EndOffset:   durationpb.New(e.Time.Sub(p.startTime)),
			Event: &pb.SessionRecordingEvent_Join{
				Join: &pb.SessionRecordingJoinEvent{
					User: e.User,
				},
			},
		})

		delete(p.activeUsers, e.User)
	}
}

func (p *sessionProcessor) handleSessionEnd(endTime time.Time, thumbnail bool) error {
	if !p.lastActivityTime.IsZero() && endTime.Sub(p.lastActivityTime) > inactivityThreshold {
		p.addInactivityEvent(p.lastActivityTime, endTime)
	}

	if thumbnail {
		p.recordThumbnail(endTime)
	}

	return nil
}

func (p *sessionProcessor) addInactivityEvent(start, end time.Time) {
	p.metadata.Events = append(p.metadata.Events, &pb.SessionRecordingEvent{
		StartOffset: durationpb.New(start.Sub(p.startTime)),
		EndOffset:   durationpb.New(end.Sub(p.startTime)),
		Event: &pb.SessionRecordingEvent_Inactivity{
			Inactivity: &pb.SessionRecordingInactivityEvent{},
		},
	})
}

func (p *sessionProcessor) recordThumbnail(t time.Time) {
	state := p.vt.DumpState()
	p.sampler.add(&state, t)
}

func (p *sessionProcessor) verifyEventsFound() error {
	if p.lastEvent == nil {
		return trace.NotFound("no events found for session %v", p.sessionID)
	}
	return nil
}

func (p *sessionProcessor) setCommonMetadata(eventTime time.Time, clusterName, userName string, resourceName string, recordingType pb.SessionRecordingType) {
	p.startTime = eventTime
	p.lastActivityTime = eventTime

	p.metadata.ClusterName = clusterName
	p.metadata.User = userName
	p.metadata.ResourceName = resourceName
	p.metadata.Type = recordingType
}

func (p *sessionProcessor) collect() (*pb.SessionRecordingMetadata, []*thumbnailEntry) {
	// Finish off any remaining activity events
	for user, userStartOffset := range p.activeUsers {
		p.metadata.Events = append(p.metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: durationpb.New(userStartOffset),
			EndOffset:   durationpb.New(getEventOffset(p.lastEvent, p.startTime)),
			Event: &pb.SessionRecordingEvent_Join{
				Join: &pb.SessionRecordingJoinEvent{
					User: user,
				},
			},
		})
	}

	p.metadata.Duration = durationpb.New(getEventOffset(p.lastEvent, p.startTime))
	p.metadata.StartTime = timestamppb.New(p.startTime)
	p.metadata.EndTime = timestamppb.New(p.lastEvent.GetTime())

	return p.metadata, p.sampler.result()
}

// getEventOffset extracts the time of the event relative to the session start time.
func getEventOffset(evt apievents.AuditEvent, startTime time.Time) time.Duration {
	switch evt := evt.(type) {
	case *apievents.SessionPrint:
		return time.Duration(evt.DelayMilliseconds) * time.Millisecond
	case *apievents.SessionEnd:
		return evt.EndTime.Sub(evt.StartTime)
	case *apievents.DatabaseSessionEnd:
		return evt.EndTime.Sub(evt.StartTime)
	case *apievents.WindowsDesktopSessionEnd:
		return evt.EndTime.Sub(evt.StartTime)
	default:
		return evt.GetTime().Sub(startTime)
	}
}
