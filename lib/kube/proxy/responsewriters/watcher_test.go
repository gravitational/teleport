/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package responsewriters

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func TestWatcherResponseWriter_ErrorResponse(t *testing.T) {
	t.Parallel()
	for _, http2 := range []bool{false, true} {
		for _, code := range []int{http.StatusForbidden, http.StatusGone, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
			for _, emptyBody := range []bool{false, true} {
				t.Run(fmt.Sprintf("http2=%v/status=%d/empty=%v", http2, code, emptyBody), func(t *testing.T) {
					t.Parallel()
					body := fmt.Sprintf(`{"kind":"Status","apiVersion":"v1","status":"Failure","code":%d}`, code)
					if emptyBody {
						body = ""
					}
					closed := make(chan error, 1)
					server := httptest.NewUnstartedServer(http.HandlerFunc(func(target http.ResponseWriter, _ *http.Request) {
						codecs := serializer.NewCodecFactory(runtime.NewScheme())
						writer, err := NewWatcherResponseWriter(target, runtime.NewClientNegotiator(codecs, schema.GroupVersion{}), nil)
						if err != nil {
							closed <- err
							return
						}
						writer.Header().Set("Content-Type", "application/json")
						writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
						writer.WriteHeader(code)
						// Forward the error in chunks, as a reverse proxy does.
						middle := len(body) / 2
						for _, chunk := range []string{body[:middle], body[middle:]} {
							if _, err := io.WriteString(writer, chunk); err != nil {
								closed <- err
								return
							}
						}
						closed <- writer.Close()
					}))
					server.EnableHTTP2 = http2
					server.StartTLS()
					t.Cleanup(server.Close)

					response, err := server.Client().Get(server.URL)
					require.NoError(t, err)
					defer response.Body.Close()
					received, err := io.ReadAll(response.Body)
					require.NoError(t, err)
					require.NoError(t, <-closed, "closing a forwarded error must not trigger another error response")
					require.Equal(t, code, response.StatusCode)
					require.Equal(t, body, string(received))
					require.Equal(t, int64(len(body)), response.ContentLength)
					if http2 {
						require.Equal(t, 2, response.ProtoMajor)
					} else {
						require.Equal(t, 1, response.ProtoMajor)
					}
				})
			}
		}
	}
}

func TestWatcherResponseWriter_PreservesWriteError(t *testing.T) {
	t.Parallel()
	writeError := errors.New("downstream write failed")
	target := &failingResponseWriter{header: make(http.Header), err: writeError}
	codecs := serializer.NewCodecFactory(runtime.NewScheme())
	writer, err := NewWatcherResponseWriter(target, runtime.NewClientNegotiator(codecs, schema.GroupVersion{}), nil)
	require.NoError(t, err)
	writer.WriteHeader(http.StatusServiceUnavailable)
	_, err = io.WriteString(writer, "service unavailable")
	require.NoError(t, err)
	require.ErrorIs(t, writer.Close(), writeError)
}

type failingResponseWriter struct {
	header http.Header
	err    error
}

func (w *failingResponseWriter) Header() http.Header       { return w.header }
func (w *failingResponseWriter) WriteHeader(int)           {}
func (w *failingResponseWriter) Write([]byte) (int, error) { return 0, w.err }
