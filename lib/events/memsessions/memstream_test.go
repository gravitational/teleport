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

package memsessions

import (
	"os"
	"testing"

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
	// Stream with handler and many parts
	t.Run("StreamManyParts", func(t *testing.T) {
		test.StreamManyParts(t, events.NewMemoryUploader())
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, events.NewMemoryUploader())
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, events.NewMemoryUploader())
	})
}
