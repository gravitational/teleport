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
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

type desktopProcessor struct {
	baseRecordingProcessor
}

func newDesktopProcessor(base baseRecordingProcessor) *desktopProcessor {
	base.thumbnailGenerator = newDesktopThumbnailGenerator()

	return &desktopProcessor{
		baseRecordingProcessor: base,
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

	d.metadata.ClusterName = evt.ClusterName
	d.metadata.User = evt.User
	d.metadata.ResourceName = evt.DesktopName
	d.metadata.Type = pb.SessionRecordingType_SESSION_RECORDING_TYPE_WINDOWS_DESKTOP

	return nil
}

func (d *desktopProcessor) handleDesktopRecording(evt *apievents.DesktopRecording) error {
	if err := d.thumbnailGenerator.handleEvent(evt); err != nil {
		return trace.Wrap(err)
	}

	d.captureThumbnailIfNeeded(evt.GetTime())

	return nil
}

func (d *desktopProcessor) handleWindowsDesktopSessionEnd(evt *apievents.WindowsDesktopSessionEnd) error {
	d.captureThumbnailIfNeeded(evt.GetTime())

	return nil
}

func (d *desktopProcessor) collect() (*pb.SessionRecordingMetadata, *pb.SessionRecordingThumbnail) {
	if d.lastEvent == nil {
		return nil, nil
	}

	d.metadata.Duration = durationpb.New(d.lastEvent.GetTime().Sub(d.startTime))
	d.metadata.StartTime = timestamppb.New(d.startTime)
	d.metadata.EndTime = timestamppb.New(d.lastEvent.GetTime())

	return d.metadata, d.thumbnail
}
