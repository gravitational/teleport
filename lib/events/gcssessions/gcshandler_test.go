/*

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
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestFakeStreams tests various streaming upload scenarios
// using fake GCS background
func TestFakeStreams(t *testing.T) {
	server := *fakestorage.NewServer([]fakestorage.Object{})
	defer server.Stop()

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	handler, err := NewHandler(ctx, cancelFunc, Config{
		Endpoint: server.URL(),
		Bucket:   fmt.Sprintf("teleport-test-%v", uuid.New()),
	}, server.Client())
	require.Nil(t, err)
	defer handler.Close()

	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
