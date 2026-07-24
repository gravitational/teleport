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

// Package compress provides HTTP middleware for transparently decompressing
// request bodies based on the Content-Encoding header.
package compress

import (
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/klauspost/compress/zstd"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

type config struct {
	writeErrorFunc func(*http.Request, http.ResponseWriter, error)
}

// Option configures the behavior of [Middleware].
type Option func(*config)

// WithWriteError sets the function used to write middleware errors to the
// client, allowing callers to render errors in a protocol-specific format.
func WithWriteError(f func(*http.Request, http.ResponseWriter, error)) Option {
	return func(c *config) {
		c.writeErrorFunc = f
	}
}

// Middleware wraps next so that compressed request bodies are transparently
// decompressed before being served. Requests without a Content-Encoding
// header pass through untouched.
// Unsupported encodings are rejected without calling next.
func Middleware(next http.Handler, opts ...Option) http.Handler {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	errorFunc := func(r *http.Request, w http.ResponseWriter, err error) {
		if cfg.writeErrorFunc != nil {
			cfg.writeErrorFunc(r, w, err)
			return
		}

		trace.WriteError(w, err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// By definition the encoding header can contain the "chain" of compression,
		// however we're not supporting them at the moment, so it is safe to
		// directly assert the encoding.
		//
		// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Encoding
		switch encoding := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))); encoding {
		case "zstd":
			downstreamReader, err := newZstdReader(
				// We also need to enforce the limited reader here because the internal
				// zstd limits apply only to effectively decoded bytes (not actual
				// bytes read).
				utils.LimitReader(r.Body, teleport.MaxHTTPRequestSize),
				r.Body,
			)
			if err != nil {
				errorFunc(r, w, err)
				return
			}
			r.Body = downstreamReader
			// The body is no longer compressed, and its decompressed size is
			// unknown, so drop the encoding/length headers to keep the request
			// consistent for handlers that forward it.
			r.Header.Del("Content-Encoding")
			r.Header.Del("Content-Length")
			r.ContentLength = -1
		case "":
			// Default non compressed body, nothing to do here.
		default:
			errorFunc(r, w, trace.NotImplemented("encoding format %q not supported, currently only non-compressed or 'zstd' is supported", encoding))
			return
		}

		next.ServeHTTP(w, r)
	})
}

type zstdReader struct {
	decoder *zstd.Decoder
	closer  io.Closer
}

// Close implements [io.ReadCloser].
func (z *zstdReader) Close() error {
	z.decoder.Close()
	// Just be sure the original buffer is also closed.
	return trace.Wrap(z.closer.Close())
}

// Read implements [io.ReadCloser].
func (z *zstdReader) Read(p []byte) (n int, err error) {
	return z.decoder.Read(p)
}

func newZstdReader(orig io.Reader, closer io.Closer) (io.ReadCloser, error) {
	dec, err := zstd.NewReader(
		orig,
		zstd.WithDecoderLowmem(true),
		// This setting works more like a limit of how much memory it can hold
		// while decompressing the requests.
		zstd.WithDecoderMaxWindow(teleport.MaxHTTPRequestSize),
		zstd.WithDecoderConcurrency(1),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &zstdReader{
		decoder: dec,
		closer:  closer,
	}, nil
}
