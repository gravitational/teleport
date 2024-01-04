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
	"fmt"
	"os"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
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
		Bucket:   fmt.Sprintf("teleport-test-%v", uuid.New().String()),
	}, server.Client())
	require.NoError(t, err)
	defer handler.Close()

	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
