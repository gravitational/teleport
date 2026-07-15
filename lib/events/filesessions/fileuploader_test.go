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

// TestReplayObjectRoundTripAndRange verifies that replay objects can be
// uploaded and read back in full or in byte ranges, and that reading a
// nonexistent replay object returns a "not found" error.
func TestReplayObjectRoundTripAndRange(t *testing.T) {
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)
	defer handler.Close()

	ctx := context.Background()
	sid := session.ID("beam-1")

	_, err = handler.UploadReplayObject(ctx, sid, "blob.0", bytes.NewReader([]byte("0123456789")))
	require.NoError(t, err)

	rc, err := handler.StreamReplayObjectRange(ctx, sid, "blob.0", 0, 0)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, "0123456789", string(got))

	rc, err = handler.StreamReplayObjectRange(ctx, sid, "blob.0", 3, 4)
	require.NoError(t, err)
	got, err = io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, "3456", string(got))

	// "blob.99" is a well-formed replay object name (matches the naming
	// scheme ValidateReplayObjectName enforces) that was simply never
	// uploaded -- distinct from a malformed name, which
	// TestReplayObjectNameRejectsPathTraversal covers below.
	_, err = handler.StreamReplayObjectRange(ctx, sid, "blob.99", 0, 0)
	require.True(t, trace.IsNotFound(err))
}

// TestReplayObjectNameRejectsPathTraversal asserts that UploadReplayObject
// and StreamReplayObjectRange reject any object name outside the
// well-known manifest/index/blob naming scheme used by ReplaySink --
// crucially, names containing path traversal segments, which would
// otherwise resolve straight through filepath.Join(l.Directory, ...) in
// replayPath to read or write files outside the beam's own replay
// artifact.
func TestReplayObjectNameRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
		OpenFile:  os.OpenFile,
	})
	require.NoError(t, err)
	defer handler.Close()

	ctx := context.Background()
	sid := session.ID("beam-1")

	badNames := []string{
		"../x",
		"blob.0/../manifest",
		"/../../etc/passwd",
		"..",
		"blob.-1",
		"blob",
		"manifest.json",
		"",
	}
	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			_, err := handler.UploadReplayObject(ctx, sid, name, bytes.NewReader([]byte("x")))
			require.True(t, trace.IsBadParameter(err), "UploadReplayObject(%q): got %v", name, err)

			_, err = handler.StreamReplayObjectRange(ctx, sid, name, 0, 0)
			require.True(t, trace.IsBadParameter(err), "StreamReplayObjectRange(%q): got %v", name, err)
		})
	}

	goodNames := []string{"manifest", "index.0", "index.12", "blob.0", "blob.12", "commands.0", "commands.7"}
	for _, name := range goodNames {
		t.Run(name, func(t *testing.T) {
			_, err := handler.UploadReplayObject(ctx, sid, name, bytes.NewReader([]byte("ok")))
			require.NoError(t, err)

			rc, err := handler.StreamReplayObjectRange(ctx, sid, name, 0, 0)
			require.NoError(t, err)
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			require.Equal(t, "ok", string(got))
		})
	}
}
