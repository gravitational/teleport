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
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// thumbnailGenerator receives events from a single session recording and uses
// them to produce the preview frames and final thumbnail for the session.
type thumbnailGenerator interface {
	// handleEvent receives an audit event from a recording and updates
	// the generator's internal state. If it returns an error then the
	// metadata generation process is aborted for this recording.
	handleEvent(event apievents.AuditEvent) error
	// produceThumbnail creates a thumbnail using the current state of the generator.
	produceThumbnail() (*pb.SessionRecordingThumbnail, error)
	// release releases any resources held by the generator. It should be called after thumbnail generation is complete.
	release()
}
