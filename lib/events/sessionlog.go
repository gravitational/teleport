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
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/session"
)

const (
	fileTypeChunks = "chunks"
	fileTypeEvents = "events"

	// eventsSuffix is the suffix of the archive that contains session events.
	eventsSuffix = "events.gz"

	// chunksSuffix is the suffix of the archive that contains session chunks.
	chunksSuffix = "chunks.gz"
)

// eventsFileName consists of session id and the first global event index
// recorded. Optionally for enhanced session recording events, the event type.
func eventsFileName(dataDir string, sessionID session.ID, eventType string, eventIndex int64) string {
	if eventType != "" {
		return filepath.Join(dataDir, fmt.Sprintf("%v-%v.%v-%v", sessionID.String(), eventIndex, eventType, eventsSuffix))
	}
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.%v", sessionID.String(), eventIndex, eventsSuffix))
}

// chunksFileName consists of session id and the first global offset recorded
func chunksFileName(dataDir string, sessionID session.ID, offset int64) string {
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.%v", sessionID.String(), offset, chunksSuffix))
}

type indexEntry struct {
	FileName   string `json:"file_name"`
	Type       string `json:"type"`
	Index      int64  `json:"index"`
	Offset     int64  `json:"offset,"`
	authServer string
}

// gzipWriter wraps file, on close close both gzip writer and file
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
	New: func() interface{} {
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
	inner io.Closer
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

func newGzipReader(reader io.ReadCloser) (*gzipReader, error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gzipReader{
		ReadCloser: gzReader,
		inner:      reader,
	}, nil
}
