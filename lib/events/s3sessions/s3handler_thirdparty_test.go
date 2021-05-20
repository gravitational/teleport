/*
Copyright 2019 Gravitational, Inc.

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

package s3sessions

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/lib/events/test"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
)

// TestThirdpartyStreams tests various streaming upload scenarios
// implemented by third party backends using fake backend
func TestThirdpartyStreams(t *testing.T) {
	var timeSource gofakes3.TimeSource
	backend := s3mem.New(s3mem.WithTimeSource(timeSource))
	faker := gofakes3.New(backend, gofakes3.WithLogger(gofakes3.GlobalLog()))
	server := httptest.NewServer(faker.Server())

	handler, err := NewHandler(Config{
		Credentials:                 credentials.NewStaticCredentials("YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", ""),
		Region:                      "us-west-1",
		Path:                        "/test/",
		Bucket:                      fmt.Sprintf("teleport-test-%v", uuid.New()),
		Endpoint:                    server.URL,
		DisableServerSideEncryption: true,
	})
	require.Nil(t, err)

	defer func() {
		if err := handler.deleteBucket(); err != nil {
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
