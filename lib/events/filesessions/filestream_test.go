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

package filesessions

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestReserveUploadPart(t *testing.T) {
	ctx := context.Background()
	partNumber := int64(1)
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	err = handler.ReserveUploadPart(ctx, *upload, partNumber)
	require.NoError(t, err)

	fi, err := os.Stat(handler.reservationPath(*upload, partNumber))
	require.NoError(t, err)
	require.GreaterOrEqual(t, fi.Size(), int64(minUploadBytes))
}

// TestReserveUploadPartPathTraversal verifies that a crafted upload whose
// session ID contains path separators cannot reserve a part outside the
// upload directory.
func TestReserveUploadPartPathTraversal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	// reservationPath joins the upload dir with string(upload.SessionID), so
	// "../outside" would escape the upload directory before any validation.
	upload.SessionID = session.ID("../outside")

	err = handler.ReserveUploadPart(ctx, *upload, int64(1))
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %T: %v", err, err)
}

// TestListUploadsSkipsInvalidSessionDir verifies that an on-disk upload whose
// session subdirectory is not a valid UUID is skipped by ListUploads instead
// of being returned, so a stale or corrupt directory cannot make the upload
// completer fail on every pass.
func TestListUploadsSkipsInvalidSessionDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	// Plant an upload directory with a valid upload ID but a non-UUID
	// session subdirectory.
	badUpload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)
	require.NoError(t, os.Rename(
		handler.uploadPath(*badUpload),
		filepath.Join(handler.uploadRootPath(*badUpload), "not-a-uuid"),
	))

	uploads, err := handler.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, 1)
	require.Equal(t, upload.ID, uploads[0].ID)
}

func TestUploadPart(t *testing.T) {
	ctx := context.Background()
	partNumber := int64(1)
	dir := t.TempDir()
	expectedContent := []byte("upload part contents")

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, session.NewID())
	require.NoError(t, err)

	err = handler.ReserveUploadPart(ctx, *upload, partNumber)
	require.NoError(t, err)

	_, err = handler.UploadPart(ctx, *upload, partNumber, bytes.NewReader(expectedContent))
	require.NoError(t, err)

	partFile, err := os.Open(handler.partPath(*upload, partNumber))
	require.NoError(t, err)
	defer partFile.Close()

	fd, err := partFile.Stat()
	require.NoError(t, err)
	require.Equal(t, int64(len(expectedContent)), fd.Size())

	partFileContent, err := io.ReadAll(partFile)
	require.NoError(t, err)
	require.True(t, bytes.Equal(expectedContent, partFileContent))
}

func TestCompleteUpload(t *testing.T) {
	ctx := context.Background()

	// Create some upload parts using reserve + write.
	createPart := func(t *testing.T, handler *Handler, upload *events.StreamUpload, partNumber int64, content []byte) events.StreamPart {
		err := handler.ReserveUploadPart(ctx, *upload, partNumber)
		require.NoError(t, err)

		if len(content) > 0 {
			part, err := handler.UploadPart(ctx, *upload, partNumber, bytes.NewReader(content))
			require.NoError(t, err)
			return *part
		}

		return events.StreamPart{Number: partNumber}
	}

	for _, test := range []struct {
		desc            string
		expectedContent []byte
		partsFunc       func(t *testing.T, handler *Handler, upload *events.StreamUpload)
	}{
		{
			desc:            "PartsWithContent",
			expectedContent: []byte("helloworld"),
			partsFunc: func(t *testing.T, handler *Handler, upload *events.StreamUpload) {
				createPart(t, handler, upload, int64(1), []byte("hello"))
				createPart(t, handler, upload, int64(2), []byte("world"))
			},
		},
		{
			desc:            "ReservationParts",
			expectedContent: []byte("helloworldwithreservation"),
			partsFunc: func(t *testing.T, handler *Handler, upload *events.StreamUpload) {
				createPart(t, handler, upload, int64(1), []byte{})
				createPart(t, handler, upload, int64(2), []byte("hello"))
				createPart(t, handler, upload, int64(3), []byte("world"))
				createPart(t, handler, upload, int64(4), []byte{})
				createPart(t, handler, upload, int64(5), []byte("withreservation"))
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			handler, err := NewHandler(Config{
				Directory: t.TempDir(),
				OpenFile:  os.OpenFile,
			})
			require.NoError(t, err)

			upload, err := handler.CreateUpload(ctx, session.NewID())
			require.NoError(t, err)

			// Create upload parts.
			test.partsFunc(t, handler, upload)

			parts, err := handler.ListParts(ctx, *upload)
			require.NoError(t, err)

			err = handler.CompleteUpload(ctx, *upload, parts)
			require.NoError(t, err)

			// Check upload contents
			uploadPath := handler.recordingPath(upload.SessionID)
			f, err := os.Open(uploadPath)
			require.NoError(t, err)

			contents, err := io.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, test.expectedContent, contents)

			require.NoDirExists(t, handler.uploadRootPath(*upload))
		})
	}
}

func TestCleanupEmptyUpload(t *testing.T) {
	ctx := t.Context()

	handler, err := NewHandler(Config{
		Directory: t.TempDir(),
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	sessionID := session.NewID()

	// Create a completed upload.
	upload, err := handler.CreateUpload(ctx, sessionID)
	require.NoError(t, err)

	err = handler.ReserveUploadPart(ctx, *upload, 1)
	require.NoError(t, err)

	content := []byte("hello world")
	part, err := handler.UploadPart(ctx, *upload, 1, bytes.NewReader(content))
	require.NoError(t, err)

	err = handler.CompleteUpload(ctx, *upload, []events.StreamPart{*part})
	require.NoError(t, err)

	// Create an empty upload with the same session ID and try to complete it.
	emptyUpload, err := handler.CreateUpload(ctx, sessionID)
	require.NoError(t, err)

	err = handler.CompleteUpload(ctx, *emptyUpload, []events.StreamPart{})
	require.NoError(t, err)

	// The empty upload should be cleaned up without impacting the original completed upload.
	uploadPath := handler.recordingPath(upload.SessionID)
	f, err := os.Open(uploadPath)
	require.NoError(t, err)

	gotContent, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, content, gotContent)

	require.NoDirExists(t, handler.uploadRootPath(*upload))
	require.NoDirExists(t, handler.uploadRootPath(*emptyUpload))
}
