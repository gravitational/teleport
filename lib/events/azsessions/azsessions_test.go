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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestAbortUploadRetriesCleanupWithoutMarker(t *testing.T) {
	for _, failure := range []string{"list parts", "delete part"} {
		t.Run(failure, func(t *testing.T) {
			upload := events.StreamUpload{ID: uuid.NewString(), SessionID: session.NewID()}
			markerPath := "/inprogress/" + uploadMarkerName(upload)
			partPath := "/inprogress/" + partName(upload, 1)
			markerExists, partExists, failOnce := true, true, true
			transport := abortUploadTransport(func(req *http.Request) (*http.Response, error) {
				status := http.StatusAccepted
				body := ""
				switch {
				case req.Method == http.MethodDelete && req.URL.Path == markerPath:
					if !markerExists {
						status = http.StatusNotFound
					}
					markerExists = false
				case req.Method == http.MethodHead && req.URL.Path == markerPath:
					status = http.StatusOK
					if !markerExists {
						status = http.StatusNotFound
					}
				case req.Method == http.MethodGet && req.URL.Query().Get("comp") == "list":
					require.False(t, markerExists, "remove the marker before listing parts")
					require.Equal(t, partPrefix(upload), req.URL.Query().Get("prefix"))
					status = http.StatusOK
					if failure == "list parts" && failOnce {
						failOnce = false
						status = http.StatusServiceUnavailable
					} else {
						if partExists {
							body = fmt.Sprintf("<Blob><Name>%s</Name><Properties/></Blob>", partName(upload, 1))
						}
						body = "<EnumerationResults><Blobs>" + body + "</Blobs><NextMarker/></EnumerationResults>"
					}
				case req.Method == http.MethodDelete && req.URL.Path == partPath:
					if failure == "delete part" && failOnce {
						failOnce = false
						status = http.StatusServiceUnavailable
					} else {
						partExists = false
					}
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				header := http.Header{"Content-Type": {"application/xml"}}
				if status == http.StatusNotFound {
					header.Set("x-ms-error-code", "BlobNotFound")
				}
				if status == http.StatusServiceUnavailable {
					header.Set("x-ms-error-code", "ServerBusy")
				}
				return &http.Response{
					StatusCode: status,
					Header:     header,
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})
			client, err := container.NewClientWithNoCredential("https://storage.example/inprogress", &container.ClientOptions{
				ClientOptions: policy.ClientOptions{
					Transport: transport,
					Retry:     policy.RetryOptions{MaxRetries: -1},
				},
			})
			require.NoError(t, err)
			handler := &Handler{inprogress: client}

			require.Error(t, handler.AbortUpload(t.Context(), upload))
			require.False(t, markerExists, "a cleanup failure must not expose the failed upload")
			require.True(t, partExists)
			_, err = handler.ListParts(t.Context(), upload)
			require.True(t, trace.IsNotFound(err), "an aborted upload must not be resumable")

			require.NoError(t, handler.AbortUpload(t.Context(), upload))
			require.False(t, partExists, "retry must clean up parts even without a marker")
			require.NoError(t, handler.AbortUpload(t.Context(), upload))
		})
	}
}

type abortUploadTransport func(*http.Request) (*http.Response, error)

func (f abortUploadTransport) Do(req *http.Request) (*http.Response, error) {
	return f(req)
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
	t.Run("UploadDownloadSummary", func(t *testing.T) {
		test.UploadDownloadSummary(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
