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

package web

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/require"
)

func TestBrotliHandler(t *testing.T) {
	t.Parallel()

	appJS := []byte("console.log('app');")
	var compressed bytes.Buffer
	bw := brotli.NewWriter(&compressed)
	_, err := bw.Write(appJS)
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	fs := http.FS(fstest.MapFS{
		// Only the pre-compressed file exists, as in production builds.
		"app/app.js.br": {Data: compressed.Bytes()},
		// Small files are not compressed.
		"app/small.js": {Data: []byte("small")},
	})
	handler := makeBrotliHandler(http.FileServer(fs), fs)

	tests := []struct {
		name                string
		method              string
		path                string
		acceptEncoding      string
		wantStatus          int
		wantBody            []byte
		wantContentEncoding string
	}{
		{
			name:                "client accepts brotli",
			method:              http.MethodGet,
			path:                "/app/app.js",
			acceptEncoding:      "gzip, deflate, br",
			wantStatus:          http.StatusOK,
			wantBody:            compressed.Bytes(),
			wantContentEncoding: "br",
		},
		{
			name:       "client without accept-encoding",
			method:     http.MethodGet,
			path:       "/app/app.js",
			wantStatus: http.StatusOK,
			wantBody:   appJS,
		},
		{
			name:           "client accepts only gzip",
			method:         http.MethodGet,
			path:           "/app/app.js",
			acceptEncoding: "gzip",
			wantStatus:     http.StatusOK,
			wantBody:       appJS,
		},
		{
			name:       "HEAD without accept-encoding",
			method:     http.MethodHead,
			path:       "/app/app.js",
			wantStatus: http.StatusOK,
		},
		{
			name:           "uncompressed file",
			method:         http.MethodGet,
			path:           "/app/small.js",
			acceptEncoding: "br",
			wantStatus:     http.StatusOK,
			wantBody:       []byte("small"),
		},
		{
			name:       "missing file",
			method:     http.MethodGet,
			path:       "/app/missing.js",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.Equal(t, tt.wantContentEncoding, resp.Header.Get("Content-Encoding"))
			if tt.wantStatus != http.StatusOK {
				return
			}
			require.Equal(t, mime.TypeByExtension(".js"), resp.Header.Get("Content-Type"))
			if tt.method == http.MethodHead {
				require.Empty(t, body)
				return
			}
			require.Equal(t, tt.wantBody, body)
		})
	}
}
