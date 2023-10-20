// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filesessions

import (
	"bytes"
	"context"
	"io"
	"os"
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
		{
			desc:            "OnlyReservation",
			expectedContent: []byte{},
			partsFunc: func(t *testing.T, handler *Handler, upload *events.StreamUpload) {
				createPart(t, handler, upload, int64(1), []byte{})
				createPart(t, handler, upload, int64(2), []byte{})
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			handler, err := NewHandler(Config{
				Directory: t.TempDir(),
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
			uploadPath := handler.path(upload.SessionID)
			f, err := os.Open(uploadPath)
			require.NoError(t, err)

			contents, err := io.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, test.expectedContent, contents)

			// Part files directory should no longer exists.
			_, err = os.ReadDir(handler.uploadRootPath(*upload))
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))
		})
	}
}
