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

// UploadHandler is a function supplied by the user, it will upload
// the file
type UploadHandler interface {
	// Upload uploads session tarball and returns URL with uploaded file
	// in case of success.
	Upload(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// Download downloads session tarball and writes it to writer
	Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error
}

// MultipartHandler handles both multipart uploads and downloads
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
