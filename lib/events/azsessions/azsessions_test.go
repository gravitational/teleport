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
	require.NoError(t, err)

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
