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

package events

import (
	"io"
	"sync"

	"github.com/gravitational/trace"
	"github.com/klauspost/compress/gzip"
)

// gzipWriter wraps file, on close close both gzip writer and file.
//
// Session recording uses github.com/klauspost/compress/gzip (imported here as
// gzip) for both the writer and the reader instead of the standard library's
// compress/gzip. Both emit and read standard gzip, so the on-disk recording
// format is unchanged and recordings written by older (stdlib) versions still
// decode.
//
// Writer: at gzip.BestSpeed the stdlib flate compressor embeds two fixed arrays
// by value (hashHead [1<<17]uint32 and hashPrev [1<<15]uint32, ~640KB total)
// that the BestSpeed code path never uses. A writer is pooled and held for the
// lifetime of each open recording slice, so a process with many concurrent
// recordings retains that unused memory per recording. klauspost only allocates
// those arrays for levels 7-9 and retains ~380KB less per writer (see
// BenchmarkNewGzipWriter). The stdlib behavior is tracked upstream in
// https://github.com/golang/go/issues/32371.
type gzipWriter struct {
	*gzip.Writer
	inner io.WriteCloser
}

// Close closes gzip writer and file
func (f *gzipWriter) Close() error {
	var errors []error
	if f.Writer != nil {
		errors = append(errors, f.Writer.Close())
		f.Writer.Reset(io.Discard)
		writerPool.Put(f.Writer)
		f.Writer = nil
	}
	if f.inner != nil {
		errors = append(errors, f.inner.Close())
		f.inner = nil
	}
	return trace.NewAggregate(errors...)
}

// writerPool is a sync.Pool for shared gzip writers.
// each gzip writer allocates a lot of memory
// so it makes sense to reset the writer and reuse the
// internal buffers to avoid too many objects on the heap
var writerPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

func newGzipWriter(writer io.WriteCloser) *gzipWriter {
	g := writerPool.Get().(*gzip.Writer)
	g.Reset(writer)
	return &gzipWriter{
		Writer: g,
		inner:  writer,
	}
}

// gzipReader wraps file, on close close both gzip writer and file
type gzipReader struct {
	io.ReadCloser
	inner io.ReadCloser
}

// Close closes file and gzip writer
func (f *gzipReader) Close() error {
	var errors []error
	if f.ReadCloser != nil {
		errors = append(errors, f.ReadCloser.Close())
		f.ReadCloser = nil
	}
	if f.inner != nil {
		errors = append(errors, f.inner.Close())
		f.inner = nil
	}
	return trace.NewAggregate(errors...)
}

// newGzipReader creates a reader for session recordings. Like the writer it uses
// klauspost/compress/gzip: it reads standard gzip (so recordings written by older
// stdlib-based versions decode unchanged) and is faster with fewer allocations
// than the stdlib reader (see BenchmarkGzipReader).
func newGzipReader(reader io.ReadCloser) (*gzipReader, error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// older bugged versions of teleport would sometimes incorrectly inject padding bytes into
	// the gzip section of the archive. this causes gzip readers with multistream enabled (the
	// default behavior) to fail. we  disable multistream here in order to ensure that the gzip
	// reader halts when it reaches the end of the current (only) valid gzip entry.
	gzReader.Multistream(false)
	return &gzipReader{
		ReadCloser: gzReader,
		inner:      reader,
	}, nil
}
