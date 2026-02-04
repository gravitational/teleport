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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/session"
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

func TestRetryOnRateLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	attempts := 0
	rateLimitedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		attempts++
		if attempts == 5 {
			cancel()
		}
	}))

	client, err := storage.NewClient(
		ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(rateLimitedServer.URL),
	)
	require.NoError(t, err)
	// Shorten backoff to shorten the test.
	client.SetRetry(storage.WithBackoff(gax.Backoff{Initial: time.Millisecond}))
	handler, err := NewHandler(ctx, cancel, Config{
		Endpoint: rateLimitedServer.URL,
		Bucket:   fmt.Sprintf("teleport-test-%v", uuid.New().String()),
	}, client)
	require.NoError(t, err)
	defer handler.Close()

	// Send a request that can trigger rate limiting. The client should retry
	// automatically until the context is canceled.
	_, err = handler.UploadPart(ctx, events.StreamUpload{
		ID:        uuid.NewString(),
		SessionID: session.ID(uuid.NewString()),
	}, 0, bytes.NewReader([]byte("foo")))
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 5, attempts)
}
