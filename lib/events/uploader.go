/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
