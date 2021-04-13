/*
Copyright 2020 Gravitational, Inc.

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

package gcssessions

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events/test"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/gravitational/trace"
)

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	uri := os.Getenv(teleport.GCSTestURI)
	if uri == "" {
		t.Skip(
			fmt.Sprintf("Skipping GCS tests, set env var %q, details here: https://goteleport.com/teleport/docs/gcp-guide/",
				teleport.GCSTestURI))
	}
	u, err := url.Parse(uri)
	require.Nil(t, err)

	config := Config{}
	err = config.SetFromURL(u)
	require.NoError(t, err)

	handler, err := DefaultNewHandler(config)
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

		composeCount := atomic.NewUint64(0)

		config.OnComposerRun = func(ctx context.Context, composer *storage.Composer) (*storage.ObjectAttrs, error) {
			if composeCount.Inc() <= 1 {
				return nil, trace.ConnectionProblem(nil, "simulate timeout %v", composeCount.Load())
			}
			return composer.Run(ctx)
		}

		handler, err := DefaultNewHandler(config)
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

		deleteFailed := atomic.NewUint64(0)

		config.AfterObjectDelete = func(ctx context.Context, object *storage.ObjectHandle, err error) error {
			if err != nil {
				return err
			}
			// delete the object, but still simulate failure
			if deleteFailed.CAS(0, 1) == true {
				return trace.ConnectionProblem(nil, "simulate delete failure %v", deleteFailed.Load())
			}
			return nil
		}

		handler, err := DefaultNewHandler(config)
		require.NoError(t, err)
		defer handler.Close()

		test.StreamResumeManyParts(t, handler)
	})
}
