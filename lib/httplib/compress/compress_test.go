// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package compress

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMiddleware(t *testing.T) {
	const (
		defaultReply      = "REPLY"
		defaultStatusCode = http.StatusOK
	)
	expectUnchangedResponse := func(tt require.TestingT, i1 any, i2 ...any) {
		require.NotNil(tt, i1, i2...)
		rec, _ := i1.(*httptest.ResponseRecorder)
		res := rec.Result()
		body, err := io.ReadAll(res.Body)
		require.NoError(tt, err, i2...)
		require.Equal(tt, defaultReply, string(body), i2...)
		require.Equal(tt, defaultStatusCode, res.StatusCode)
	}

	for name, tc := range map[string]struct {
		newRequestFunc func() *http.Request
		writeErrorFunc func(*http.Request, http.ResponseWriter, error)
		expectRequest  require.ValueAssertionFunc
		expectResponse require.ValueAssertionFunc
	}{
		"compressed request is received decompressed by next handler": {
			newRequestFunc: func() *http.Request {
				encoded := zstd.EncodeTo([]byte{}, []byte("non compressed req"))
				req, _ := http.NewRequest(
					http.MethodPost,
					"",
					bytes.NewReader(encoded),
				)
				req.Header.Add("Content-Encoding", "zstd")
				return req
			},
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				req, _ := i1.(*http.Request)
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err, i2...)
				require.Equal(tt, []byte("non compressed req"), body, i2...)
				// Since the body is no longer compressed, the encoding/length
				// headers must be reset.
				require.Empty(tt, req.Header.Get("Content-Encoding"), i2...)
				require.Empty(tt, req.Header.Get("Content-Length"), i2...)
				require.EqualValues(tt, -1, req.ContentLength, i2...)
			},
			expectResponse: expectUnchangedResponse,
		},
		"compressed request exceeds max size": {
			newRequestFunc: func() *http.Request {
				gen := rand.New(rand.NewPCG(uint64(1), uint64(1)))
				raw := make([]byte, teleport.MaxHTTPRequestSize+1024*1024)
				for offset := 0; offset+8 <= len(raw); offset += 8 {
					binary.LittleEndian.PutUint64(raw[offset:offset+8], gen.Uint64())
				}
				req, _ := http.NewRequest(
					http.MethodPost,
					"",
					bytes.NewReader(zstd.EncodeTo([]byte{}, raw)),
				)
				req.Header.Add("Content-Encoding", "zstd")
				return req
			},
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				req, _ := i1.(*http.Request)
				// Since the compressed version exceeds the size, we expect an
				// error while reading it.
				_, err := io.ReadAll(req.Body)
				require.Error(tt, err, i2...)
			},
			// Next handler will still be called, it is where the limit exceeded
			// error will be raised.
			expectResponse: expectUnchangedResponse,
		},
		"compressed request does not exceed max but decompressed does": {
			newRequestFunc: func() *http.Request {
				// Generate a "very compressible" body.
				gen := "body " + strings.Repeat("a", teleport.MaxHTTPRequestSize)
				req, _ := http.NewRequest(
					http.MethodPost,
					"",
					bytes.NewReader(zstd.EncodeTo([]byte{}, []byte(gen))),
				)
				req.Header.Add("Content-Encoding", "zstd")
				return req
			},
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				req, _ := i1.(*http.Request)
				// The full body will still be available on the next handler,
				// but it should itself force the request limits.
				_, err := io.ReadAll(utils.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
				require.Error(tt, err, i2...)
			},
			// Next handler will still be called even when the body is empty,
			// it is up to the handler to gracefully handle this case.
			expectResponse: expectUnchangedResponse,
		},
		"unsupported encoding format": {
			newRequestFunc: func() *http.Request {
				req, _ := http.NewRequest(
					http.MethodPost,
					"",
					strings.NewReader("random data"),
				)
				req.Header.Add("Content-Encoding", "random")
				return req
			},
			// Next handler won't be called, so we expect no request, and error
			// message written into the recorder.
			expectRequest: require.Nil,
			expectResponse: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				rec, _ := i1.(*httptest.ResponseRecorder)
				res := rec.Result()
				body, err := io.ReadAll(res.Body)
				require.NoError(tt, err, i2...)

				middlewareErr := trace.ReadError(res.StatusCode, body)
				require.Error(tt, middlewareErr, i2...)
				require.True(tt, trace.IsNotImplemented(middlewareErr), "expected NotImplementedError but got %T", middlewareErr)
			},
		},
		"custom write error function": {
			newRequestFunc: func() *http.Request {
				req, _ := http.NewRequest(
					http.MethodPost,
					"",
					strings.NewReader("random data"),
				)
				req.Header.Add("Content-Encoding", "random")
				return req
			},
			writeErrorFunc: func(_ *http.Request, w http.ResponseWriter, err error) {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "my custom error")
			},
			// Next handler won't be called, so we expect no request, and error
			// message written into the recorder.
			expectRequest: require.Nil,
			expectResponse: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				rec, _ := i1.(*httptest.ResponseRecorder)
				res := rec.Result()
				body, err := io.ReadAll(res.Body)
				require.NoError(tt, err, i2...)

				require.Equal(tt, http.StatusInternalServerError, res.StatusCode, i2...)
				require.Equal(tt, "my custom error", string(body), i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			var (
				nextReq *http.Request
			)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextReq = r
				w.WriteHeader(defaultStatusCode)
				io.WriteString(w, defaultReply)
			})

			Middleware(next, WithWriteError(tc.writeErrorFunc)).ServeHTTP(writer, tc.newRequestFunc())
			tc.expectRequest(t, nextReq)
			tc.expectResponse(t, writer)
		})
	}
}
