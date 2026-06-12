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

package proxy

import (
	"compress/gzip"
	"io"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
)

const (
	// contentEncodingHeader is the HTTP header used to specify the
	// content encoding of the response.
	contentEncodingHeader = "Content-Encoding"
	// contentEncodingGZIP is the value for the Content-Encoding header when
	// the response is compressed with gzip.
	contentEncodingGZIP = "gzip"

	// defaultGzipContentEncodingLevel is set to 1 which uses least CPU compared to higher levels, yet offers
	// similar compression ratios (off by at most 1.5x, but typically within 1.1x-1.3x). For further details see -
	// https://github.com/kubernetes/kubernetes/issues/112296
	defaultGzipContentEncodingLevel = 1
)

var gzipPool = &sync.Pool{
	New: func() any {
		gw, err := gzip.NewWriterLevel(nil, defaultGzipContentEncodingLevel)
		if err != nil {
			// This should never happen.
			panic(err)
		}
		return gw
	},
}

type (
	// compressionFunc is a function that decompresses data.
	decompressionFunc func(dst io.Writer, src io.Reader) error
	// compressionFunc is a function that returns a WriteCloser that compresses data
	// written to it into the provided io.Writer.
	compressionFunc func(dst io.Writer) io.WriteCloser
)

// getResponseCompressorDecompressor returns a compression and decompression function based on the
// Content-Encoding header.
func getResponseCompressorDecompressor(headers http.Header) (compressor compressionFunc, decompressor decompressionFunc, err error) {
	encoding := headers.Get(contentEncodingHeader)
	switch encoding {
	case contentEncodingGZIP:
		compressor = func(dst io.Writer) io.WriteCloser {
			gzw := gzipPool.Get().(*gzip.Writer)
			gzw.Reset(dst)
			return &gzipWrapper{gzw}
		}
		decompressor = func(dst io.Writer, src io.Reader) error {
			gzr, err := gzip.NewReader(src)
			if err != nil {
				return trace.Wrap(err)
			}
			defer gzr.Close()
			_, err = io.Copy(dst, gzr)
			return trace.Wrap(err)
		}
		return
	case "":
		compressor = func(dst io.Writer) io.WriteCloser {
			return &nopCloserWrapper{dst}
		}
		decompressor = func(dst io.Writer, src io.Reader) error {
			_, err := io.Copy(dst, src)
			return trace.Wrap(err)
		}
		return
	default:
		return nil, nil, trace.BadParameter("unknown encoding %q", encoding)
	}
}

// gzipWrapper wraps a gzip.Writer to implement io.WriteCloser.
// When Close is called, the underlying gzip.Writer is returned to the pool.
type gzipWrapper struct {
	*gzip.Writer
}

// Close closes the underlying writter and returns it to the pool.
func (w *gzipWrapper) Close() error {
	err := w.Writer.Close()
	w.Writer.Reset(nil)
	gzipPool.Put(w.Writer)
	w.Writer = nil
	return trace.Wrap(err)
}

// nopCloserWrapper wraps an io.Writer to implement io.WriteCloser.
type nopCloserWrapper struct {
	io.Writer
}

// Close has no action on the underlying writer.
func (*nopCloserWrapper) Close() error {
	return nil
}
