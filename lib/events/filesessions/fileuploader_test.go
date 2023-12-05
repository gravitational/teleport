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
	"os"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	dir := t.TempDir()

	handler, err := NewHandler(Config{
		Directory: dir,
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
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
