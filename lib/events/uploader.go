/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events

import (
	"context"
	"io"
	"regexp"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/session"
)

// replayObjectNameRE matches the only object names ReplaySink ever writes
// for a beam's replay artifact: the manifest, and blob/index/command-index
// parts numbered from 0 (see partName/indexPageName/commandsPageName/
// manifestObjectName in e/lib/beams/v1/replay).
var replayObjectNameRE = regexp.MustCompile(`^(manifest|index\.[0-9]+|blob\.[0-9]+|commands\.[0-9]+)$`)

// ValidateReplayObjectName returns a trace.BadParameter error unless name is
// one of the well-known beam-replay artifact object names (the manifest, or
// a numbered index/blob/command-index part).
//
// UploadHandler implementations must call this before using a caller- or
// client-influenced name to build a storage path/key: name ultimately
// derives from a PayloadRef.PartName that travels over the wire in a
// FetchPayloadRequest, so without this check a forged name containing path
// traversal segments (e.g. "../other-session.summary.json") could escape
// the beam's own object namespace once joined into a filepath.Join or
// path.Join call.
func ValidateReplayObjectName(name string) error {
	if !replayObjectNameRE.MatchString(name) {
		return trace.BadParameter("invalid replay object name %q", name)
	}
	return nil
}

// UploadHandler uploads and downloads session-related files.
type UploadHandler interface {
	// Upload uploads a session recording and returns a URL with uploaded file in
	// case of success.
	Upload(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// StreamSessionRecording streams a session recording and returns a ReadCloser for the content.
	StreamSessionRecording(ctx context.Context, sessionID session.ID) (io.ReadCloser, error)
	// UploadPendingSummary uploads a pending session summary and returns a URL
	// with uploaded file in case of success. This function can be called
	// multiple times for a given sessionID to update the state. A pending
	// session summary is any summary state that can still be later overwritten.
	// It should still be contained in the same structure as the final one, but
	// missing some data (in particular, the summary content itself).
	UploadPendingSummary(ctx context.Context, sesisonID session.ID, readCloser io.Reader) (string, error)
	// UploadSummary uploads a final session summary and returns a URL with
	// uploaded file in case of success. This function can be called only once
	// for a given sessionID; subsequent calls will return an error.
	UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// StreamSessionSummary streams a session summary and returns a ReadCloser for
	// the content. Returns a "not found" error if there's no such summary.
	StreamSessionSummary(ctx context.Context, sessionID session.ID) (io.ReadCloser, error)
	// UploadReplayObject writes a named beam-replay artifact object (e.g.
	// "manifest", "index.0", "blob.0") for a beam and returns its URL.
	UploadReplayObject(ctx context.Context, sessionID session.ID, name string, readCloser io.Reader) (string, error)
	// StreamReplayObjectRange returns a ranged reader over a replay object.
	// A length <= 0 reads to the end of the object.
	StreamReplayObjectRange(ctx context.Context, sessionID session.ID, name string, offset, length int64) (io.ReadCloser, error)
	// UploadMetadata uploads session metadata and returns a URL with the uploaded
	// file in case of success. Session metadata is a file with a [recordingmetadatav1.SessionRecordingMetadata]
	// protobuf message containing info about the session (duration, events, etc), as well as
	// multiple [recordingmetadatav1.SessionRecordingThumbnail] messages (thumbnails).
	UploadMetadata(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// StreamSessionMetadata streams session metadata and returns a ReadCloser for the content.
	StreamSessionMetadata(ctx context.Context, sessionID session.ID) (io.ReadCloser, error)
	// UploadThumbnail uploads a session thumbnail and returns a URL with uploaded
	// file in case of success. A thumbnail is [recordingmetadatav1.SessionRecordingThumbnail]
	// protobuf message which contains the thumbnail as an SVG, and some basic details about the
	// state of the terminal at the time of the thumbnail capture (terminal size, cursor position).
	UploadThumbnail(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// StreamSessionThumbnail streams a session thumbnail and returns a ReadCloser for the content.
	StreamSessionThumbnail(ctx context.Context, sessionID session.ID) (io.ReadCloser, error)
}

// MultipartHandler handles both multipart and standalone uploads and
// downloads.
type MultipartHandler interface {
	UploadHandler
	MultipartUploader
}

// UploadEvent is emitted by uploader and is used in tests
type UploadEvent struct {
	// SessionID is a session ID
	SessionID string
	// UploadID specifies upload ID for a successful upload
	UploadID string
	// Error is set in case if event resulted in error
	Error error
	// Created is a time of when the event has been created
	Created time.Time
}
