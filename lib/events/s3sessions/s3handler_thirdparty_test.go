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

package s3sessions

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
)

// TestThirdpartyStreams tests various streaming upload scenarios
// implemented by third party backends using fake backend
func TestThirdpartyStreams(t *testing.T) {
	var timeSource gofakes3.TimeSource
	backend := s3mem.New(s3mem.WithTimeSource(timeSource))
	faker := gofakes3.New(backend, gofakes3.WithLogger(gofakes3.GlobalLog()))
	server := httptest.NewServer(faker.Server())

	handler, err := NewHandler(context.Background(), Config{
		Credentials:                 credentials.NewStaticCredentials("YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", ""),
		Region:                      "us-west-1",
		Path:                        "/test/",
		Bucket:                      fmt.Sprintf("teleport-test-%v", uuid.New().String()),
		Endpoint:                    server.URL,
		DisableServerSideEncryption: true,
	})
	require.NoError(t, err)

	defer func() {
		if err := handler.deleteBucket(context.Background()); err != nil {
			t.Fatalf("Failed to delete bucket: %#v", trace.DebugReport(err))
		}
	}()

	// Stream with handler and many parts
	t.Run("StreamManyParts", func(t *testing.T) {
		test.Stream(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
