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
	"time"

	"github.com/gravitational/teleport/lib/session"
)

// UploadHandler uploads and downloads session-related files.
type UploadHandler interface {
	// Upload uploads a session recording and returns a URL with uploaded file in
	// case of success.
	Upload(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// Download downloads a session recording and writes it to a writer.
	Download(ctx context.Context, sessionID session.ID, writer RandomAccessWriter) error
	// UploadSummary uploads a session summary and returns a URL with uploaded
	// file in case of success.
	UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// DownloadSummary downloads a session summary and writes it to a writer.
	DownloadSummary(ctx context.Context, sessionID session.ID, writer RandomAccessWriter) error
	// UploadMetadata uploads session metadata and returns a URL with the uploaded
	// file in case of success. Session metadata is a file with a [recordingmetadatav1.SessionRecordingMetadata]
	// protobuf message containing info about the session (duration, events, etc), as well as
	// multiple [recordingmetadatav1.SessionRecordingThumbnail] messages (thumbnails).
	UploadMetadata(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// DownloadMetadata downloads session metadata and writes it to a writer.
	DownloadMetadata(ctx context.Context, sessionID session.ID, writer RandomAccessWriter) error
	// UploadThumbnail uploads a session thumbnail and returns a URL with uploaded
	// file in case of success. A thumbnail is [recordingmetadatav1.SessionRecordingThumbnail]
	// protobuf message which contains the thumbnail as an SVG, and some basic details about the
	// state of the terminal at the time of the thumbnail capture (terminal size, cursor position).
	UploadThumbnail(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// DownloadThumbnail downloads a session thumbnail and writes it to a writer.
	DownloadThumbnail(ctx context.Context, sessionID session.ID, writer RandomAccessWriter) error
}

type RandomAccessWriter interface {
	io.Writer
	io.WriterAt
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
