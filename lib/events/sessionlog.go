/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/session"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

const (
	fileTypeChunks = "chunks"
	fileTypeEvents = "events"

	// eventsSuffix is the suffix of the archive that contains session events.
	eventsSuffix = "events.gz"

	// chunksSuffix is the suffix of the archive that contains session chunks.
	chunksSuffix = "chunks.gz"
)

// sessionLogger is an interface that all session loggers must implement.
type SessionLogger interface {
	// LogEvent logs events associated with this session.
	LogEvent(fields EventFields) error

	// Close is called when clients close on the requested "session writer".
	// We ignore their requests because this writer (file) should be closed only
	// when the session logger is closed.
	Close() error

	// Finalize is called by the session when it's closing. This is where we're
	// releasing audit resources associated with the session
	Finalize() error
}

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

// EventFromChunk returns event converted from session chunk
func EventFromChunk(sessionID string, chunk *SessionChunk) (EventFields, error) {
	var fields EventFields
	eventStart := time.Unix(0, chunk.Time).In(time.UTC).Round(time.Millisecond)
	err := json.Unmarshal(chunk.Data, &fields)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fields[SessionEventID] = sessionID
	fields[EventIndex] = chunk.EventIndex
	fields[EventTime] = eventStart
	fields[EventType] = chunk.EventType
	if fields[EventID] == "" {
		fields[EventID] = uuid.New().String()
	}
	return fields, nil
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
