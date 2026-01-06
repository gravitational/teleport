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
	"strings"
	"testing"

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
			version, err := handler.GetRecordingVersion(ctx, upload.SessionID, "")
			require.NoError(t, err)
			require.NotEmpty(t, version)

			// Check upload contents
			uploadPath := handler.recordingPath(events.StreamUpload{
				SessionID: upload.SessionID,
			})
			f, err := os.Open(uploadPath)
			require.NoError(t, err)

			contents, err := io.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, test.expectedContent, contents)

			require.NoDirExists(t, handler.uploadRootPath(*upload))
		})
	}
}

func TestCompleteTemporaryUpload(t *testing.T) {
	ctx := t.Context()
	handler, err := NewHandler(Config{
		Directory: t.TempDir(),
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)

	// Create the upload.
	upload, err := handler.CreateUpload(ctx, session.NewID(), events.WithUploadTemporary())
	require.NoError(t, err)
	uploads, err := handler.ListUploads(ctx)
	require.NoError(t, err)
	require.Len(t, uploads, 1)
	upload.Initiated = uploads[0].Initiated
	require.Equal(t, *upload, uploads[0])

	// Create some upload parts using reserve + write.
	createPart := func(partNumber int64, content []byte) events.StreamPart {
		err := handler.ReserveUploadPart(ctx, *upload, partNumber)
		require.NoError(t, err)

		if len(content) > 0 {
			part, err := handler.UploadPart(ctx, *upload, partNumber, bytes.NewReader(content))
			require.NoError(t, err)
			return *part
		}

		return events.StreamPart{Number: partNumber}
	}
	parts := []events.StreamPart{
		createPart(0, []byte("hello ")),
		createPart(1, []byte("world")),
		createPart(2, []byte("!")),
	}
	// Complete the upload. Verify that the recording was written to the temporary
	// upload location and not the final location.
	require.NoError(t, handler.CompleteUpload(ctx, *upload, parts))
	require.NoFileExists(t, handler.recordingPath(events.StreamUpload{
		SessionID: upload.SessionID,
	}))
	require.FileExists(t, handler.recordingPath(*upload))
	// Verify that completing the upload with no parts deletes the temp recording.
	require.NoError(t, handler.CompleteUpload(ctx, *upload, nil))
	require.NoFileExists(t, handler.recordingPath(*upload))
}

func TestRejectVersionMismatch(t *testing.T) {
	t.Parallel()
	handler, err := NewHandler(Config{Directory: t.TempDir()})
	require.NoError(t, err)

	createPart := func(upload events.StreamUpload, content string) events.StreamPart {
		err := handler.ReserveUploadPart(t.Context(), upload, 1)
		require.NoError(t, err)
		part, err := handler.UploadPart(t.Context(), upload, 1, strings.NewReader(content))
		require.NoError(t, err)
		return *part
	}

	// Create initial upload.
	sessionID := session.NewID()
	initialUpload, err := handler.CreateUpload(t.Context(), sessionID)
	require.NoError(t, err)
	part := createPart(*initialUpload, "foo")
	require.NoError(t, handler.CompleteUpload(t.Context(), *initialUpload, []events.StreamPart{part}))
	version, err := handler.GetRecordingVersion(t.Context(), sessionID, "")
	require.NoError(t, err)
	require.NotEmpty(t, version)

	// Create two uploads targeting the same version.
	firstReupload, err := handler.CreateUpload(t.Context(), sessionID, events.WithUploadReplace(version))
	require.NoError(t, err)
	secondReupload, err := handler.CreateUpload(t.Context(), sessionID, events.WithUploadReplace(version))
	require.NoError(t, err)
	firstPart := createPart(*firstReupload, "bar")
	secondPart := createPart(*secondReupload, "baz")

	// Verify that the first upload succeeds, but the second upload fails because
	// the version has changed.
	require.NoError(t, handler.CompleteUpload(t.Context(), *firstReupload, []events.StreamPart{firstPart}))
	require.Error(t, handler.CompleteUpload(t.Context(), *secondReupload, []events.StreamPart{secondPart}))
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
	uploadPath := handler.recordingPath(events.StreamUpload{
		SessionID: upload.SessionID,
	})
	f, err := os.Open(uploadPath)
	require.NoError(t, err)

	gotContent, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, content, gotContent)

	require.NoDirExists(t, handler.uploadRootPath(*upload))
	require.NoDirExists(t, handler.uploadRootPath(*emptyUpload))
}
