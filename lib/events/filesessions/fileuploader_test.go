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
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

// TestUploadAndStreamHAR verifies a round-trip upload and stream of the HAR archive.
func TestUploadAndStreamHAR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)
	defer handler.Close()

	sid := session.NewID()
	const payload = `{"log":{}}`

	_, err = handler.UploadHAR(context.Background(), sid, strings.NewReader(payload))
	require.NoError(t, err)

	rc, err := handler.StreamHAR(context.Background(), sid)
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, payload, string(data))
}

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)
	defer handler.Close()

	t.Run("Stream", func(t *testing.T) {
		test.Stream(t, handler)
	})
	t.Run("Resume", func(t *testing.T) {
		var completeCount atomic.Uint64
		handler, err := NewHandler(Config{
			Directory: dir,
			OpenFile:  os.OpenFile,
			OnBeforeComplete: func(ctx context.Context, upload events.StreamUpload) error {
				if completeCount.Add(1) <= 1 {
					return trace.ConnectionProblem(nil, "simulate failure %v", completeCount.Load())
				}
				return nil
			},
		})
		require.NoError(t, err)
		defer handler.Close()

		test.StreamResumeManyParts(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("UploadDownloadSummary", func(t *testing.T) {
		test.UploadDownloadSummary(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
