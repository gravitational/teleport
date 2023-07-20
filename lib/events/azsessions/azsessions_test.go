// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azsessions

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestStreams runs the standard events test suite over azsessions, if a
// configuration URL is specified in the appropriate envvar.
func TestStreams(t *testing.T) {
	ctx := context.Background()

	envURL := os.Getenv(teleport.AZBlobTestURI)
	if envURL == "" {
		t.Skipf("Skipping azsessions tests as %q is not set.", teleport.AZBlobTestURI)
	}

	u, err := url.Parse(envURL)
	require.NoError(t, err)

	var config Config
	err = config.SetFromURL(u)
	require.NoError(t, err)

	handler, err := NewHandler(ctx, config)
	require.Nil(t, err)

	t.Run("StreamManyParts", func(t *testing.T) {
		test.StreamManyParts(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
