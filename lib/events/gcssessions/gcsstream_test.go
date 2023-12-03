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

package gcssessions

import (
	"context"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events/test"
)

func TestUploadFromPath(t *testing.T) {
	for _, test := range []struct {
		path                string
		sessionID, uploadID string
		assertErr           require.ErrorAssertionFunc
	}{
		{
			path:      "uploads/73de0358-2a40-4940-ae26-0c06877e35d9/cf9e08d5-6651-4ddd-a472-52d2286d6bb4.upload",
			sessionID: "cf9e08d5-6651-4ddd-a472-52d2286d6bb4",
			uploadID:  "73de0358-2a40-4940-ae26-0c06877e35d9",
			assertErr: require.NoError,
		},
		{
			path:      "uploads/73de0358-2a40-4940-ae26-0c06877e35d9/cf9e08d5-6651-4ddd-a472-52d2286d6bb4.BADEXTENSION",
			assertErr: require.Error,
		},
		{
			path:      "no-dir.upload",
			assertErr: require.Error,
		},
	} {
		t.Run(test.path, func(t *testing.T) {
			upload, err := uploadFromPath(test.path)
			test.assertErr(t, err)
			if test.sessionID != "" {
				require.Equal(t, test.sessionID, string(upload.SessionID))
			}
			if test.uploadID != "" {
				require.Equal(t, test.uploadID, upload.ID)
			}
		})
	}
}

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	ctx := context.Background()
	uri := os.Getenv(teleport.GCSTestURI)
	if uri == "" {
		t.Skipf("Skipping GCS tests, set env var %q, details here: https://goteleport.com/teleport/docs/gcp-guide/",
			teleport.GCSTestURI)
	}
	u, err := url.Parse(uri)
	require.NoError(t, err)

	config := Config{}
	err = config.SetFromURL(u)
	require.NoError(t, err)

	handler, err := DefaultNewHandler(ctx, config)
	require.NoError(t, err)
	defer handler.Close()

	// Stream with handler and many parts
	t.Run("StreamManyParts", func(t *testing.T) {
		test.StreamManyParts(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
	// This tests makes sure that resume works
	// if the first attempt to compose object failed
	t.Run("ResumeOnComposeFailure", func(t *testing.T) {
		config := Config{}
		err = config.SetFromURL(u)
		require.NoError(t, err)

		var composeCount atomic.Uint64

		config.OnComposerRun = func(ctx context.Context, composer *storage.Composer) (*storage.ObjectAttrs, error) {
			if composeCount.Add(1) <= 1 {
				return nil, trace.ConnectionProblem(nil, "simulate timeout %v", composeCount.Load())
			}
			return composer.Run(ctx)
		}

		handler, err := DefaultNewHandler(ctx, config)
		require.NoError(t, err)
		defer handler.Close()

		test.StreamResumeManyParts(t, handler)
	})
	// This test makes sure that resume works
	// if the attempt to delete the object on cleanup failed
	t.Run("ResumeOnCleanupFailure", func(t *testing.T) {
		config := Config{}
		err = config.SetFromURL(u)
		require.NoError(t, err)

		var deleteFailed atomic.Uint64

		config.AfterObjectDelete = func(ctx context.Context, object *storage.ObjectHandle, err error) error {
			if err != nil {
				return err
			}
			// delete the object, but still simulate failure
			if deleteFailed.CompareAndSwap(0, 1) == true {
				return trace.ConnectionProblem(nil, "simulate delete failure %v", deleteFailed.Load())
			}
			return nil
		}

		handler, err := DefaultNewHandler(ctx, config)
		require.NoError(t, err)
		defer handler.Close()

		test.StreamResumeManyParts(t, handler)
	})
}
