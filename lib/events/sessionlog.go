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
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
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

	// PostSessionSlice posts session slice
	PostSessionSlice(slice SessionSlice) error
}

// DiskSessionLoggerConfig sets up parameters for disk session logger
// associated with the session ID
type DiskSessionLoggerConfig struct {
	// SessionID is the session id of the logger
	SessionID session.ID
	// DataDir is data directory for session events files
	DataDir string
	// Clock is the clock replacement
	Clock clockwork.Clock
	// RecordSessions controls if sessions are recorded along with audit events.
	RecordSessions bool
}

// NewDiskSessionLogger creates new disk based session logger
func NewDiskSessionLogger(cfg DiskSessionLoggerConfig) (*DiskSessionLogger, error) {
	var err error

	indexFile, err := os.OpenFile(filepath.Join(cfg.DataDir, fmt.Sprintf("%v.index", cfg.SessionID.String())), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionLogger := &DiskSessionLogger{
		DiskSessionLoggerConfig: cfg,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
			trace.ComponentFields: log.Fields{
				"sid": cfg.SessionID,
			},
		}),
		indexFile:      indexFile,
		lastEventIndex: -1,
		lastChunkIndex: -1,
	}
	return sessionLogger, nil
}

// DiskSessionLogger implements a disk based session logger. The imporant
// property of the disk based logger is that it never fails and can be used as
// a fallback implementation behind more sophisticated loggers.
type DiskSessionLogger struct {
	DiskSessionLoggerConfig

	*log.Entry

	sync.Mutex

	sid session.ID

	indexFile  *os.File
	eventsFile *gzipWriter
	chunksFile *gzipWriter

	lastEventIndex int64
	lastChunkIndex int64
}

// LogEvent logs an event associated with this session
func (sl *DiskSessionLogger) LogEvent(fields EventFields) error {
	panic("should not be used")
}

// Close is called when clients close on the requested "session writer".
// We ignore their requests because this writer (file) should be closed only
// when the session logger is closed
func (sl *DiskSessionLogger) Close() error {
	return nil
}

// Finalize is called by the session when it's closing. This is where we're
// releasing audit resources associated with the session
func (sl *DiskSessionLogger) Finalize() error {
	sl.Lock()
	defer sl.Unlock()

	return sl.finalize()
}

// flush is used to flush gzip frames to file, otherwise
// some attempts to read the file could fail
func (sl *DiskSessionLogger) flush() error {
	var err, err2 error

	if sl.RecordSessions && sl.chunksFile != nil {
		err = sl.chunksFile.Flush()
	}
	if sl.eventsFile != nil {
		err2 = sl.eventsFile.Flush()
	}
	return trace.NewAggregate(err, err2)
}

func (sl *DiskSessionLogger) finalize() error {

	auditOpenFiles.Dec()

	if sl.indexFile != nil {
		sl.indexFile.Close()
	}

	if sl.chunksFile != nil {
		if err := sl.chunksFile.Close(); err != nil {
			log.Warningf("Failed closing chunks file: %v.", err)
		}
	}

	if sl.eventsFile != nil {
		if err := sl.eventsFile.Close(); err != nil {
			log.Warningf("Failed closing events file: %v.", err)
		}
	}

	return nil
}

// eventsFileName consists of session id and the first global event index recorded there
func eventsFileName(dataDir string, sessionID session.ID, eventIndex int64) string {
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.events.gz", sessionID.String(), eventIndex))
}

// chunksFileName consists of session id and the first global offset recorded
func chunksFileName(dataDir string, sessionID session.ID, offset int64) string {
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.chunks.gz", sessionID.String(), offset))
}

func (sl *DiskSessionLogger) openEventsFile(eventIndex int64) error {
	if sl.eventsFile != nil {
		err := sl.eventsFile.Close()
		if err != nil {
			sl.Warningf("Failed to close file: %v", trace.DebugReport(err))
		}
	}
	eventsFileName := eventsFileName(sl.DataDir, sl.SessionID, eventIndex)

	// update the index file to write down that new events file has been created
	data, err := json.Marshal(indexEntry{
		FileName: filepath.Base(eventsFileName),
		Type:     fileTypeEvents,
		Index:    eventIndex,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintf(sl.indexFile, "%v\n", string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	// open new events file for writing
	file, err := os.OpenFile(eventsFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	sl.eventsFile = newGzipWriter(file)
	return nil
}

func (sl *DiskSessionLogger) openChunksFile(offset int64) error {
	if sl.chunksFile != nil {
		err := sl.chunksFile.Close()
		if err != nil {
			sl.Warningf("Failed to close file: %v", trace.DebugReport(err))
		}
	}
	chunksFileName := chunksFileName(sl.DataDir, sl.SessionID, offset)

	// udpate the index file to write down that new chunks file has been created
	data, err := json.Marshal(indexEntry{
		FileName: filepath.Base(chunksFileName),
		Type:     fileTypeChunks,
		Offset:   offset,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintf(sl.indexFile, "%v\n", string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	// open new chunks file for writing
	file, err := os.OpenFile(chunksFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	sl.chunksFile = newGzipWriter(file)
	return nil
}

// PostSessionSlice takes series of events associated with the session
// and writes them to events files and data file for future replays
func (sl *DiskSessionLogger) PostSessionSlice(slice SessionSlice) error {
	sl.Lock()
	defer sl.Unlock()

	for i := range slice.Chunks {
		_, err := sl.writeChunk(slice.SessionID, slice.Chunks[i])
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return sl.flush()
}

func eventFromChunk(sessionID string, chunk *SessionChunk) (EventFields, error) {
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
	return fields, nil
}

func (sl *DiskSessionLogger) writeChunk(sessionID string, chunk *SessionChunk) (written int, err error) {

	// this section enforces the following invariant:
	// a single events file only contains successive events
	if sl.lastEventIndex == -1 || chunk.EventIndex-1 != sl.lastEventIndex {
		if err := sl.openEventsFile(chunk.EventIndex); err != nil {
			return -1, trace.Wrap(err)
		}
	}
	sl.lastEventIndex = chunk.EventIndex
	eventStart := time.Unix(0, chunk.Time).In(time.UTC).Round(time.Millisecond)
	if chunk.EventType != SessionPrintEvent {
		fields, err := eventFromChunk(sessionID, chunk)
		if err != nil {
			return -1, trace.Wrap(err)
		}
		data, err := json.Marshal(fields)
		if err != nil {
			return -1, trace.Wrap(err)
		}
		return sl.eventsFile.Write(append(data, '\n'))
	}
	if !sl.RecordSessions {
		return len(chunk.Data), nil
	}
	// this section enforces the following invariant:
	// a single chunks file only contains successive chunks
	if sl.lastChunkIndex == -1 || chunk.ChunkIndex-1 != sl.lastChunkIndex {
		if err := sl.openChunksFile(chunk.Offset); err != nil {
			return -1, trace.Wrap(err)
		}
	}
	sl.lastChunkIndex = chunk.ChunkIndex
	event := printEvent{
		Start:             eventStart,
		Type:              SessionPrintEvent,
		Bytes:             int64(len(chunk.Data)),
		DelayMilliseconds: chunk.Delay,
		Offset:            chunk.Offset,
		EventIndex:        chunk.EventIndex,
		ChunkIndex:        chunk.ChunkIndex,
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return -1, trace.Wrap(err)
	}
	_, err = sl.eventsFile.Write(append(bytes, '\n'))
	if err != nil {
		return -1, trace.Wrap(err)
	}
	return sl.chunksFile.Write(chunk.Data)
}

func diff(before, after time.Time) int64 {
	d := int64(after.Sub(before) / time.Millisecond)
	if d < 0 {
		return 0
	}
	return d
}

const (
	fileTypeChunks = "chunks"
	fileTypeEvents = "events"
)

type indexEntry struct {
	FileName   string `json:"file_name"`
	Type       string `json:"type"`
	Index      int64  `json:"index"`
	Offset     int64  `json:"offset,"`
	authServer string
}

type printEvent struct {
	// Start is event start
	Start time.Time `json:"time"`
	// Type is event type
	Type string `json:"event"`
	// Bytes is event bytes
	Bytes int64 `json:"bytes"`
	// DelayMilliseconds is the delay in milliseconds from the start of the session
	DelayMilliseconds int64 `json:"ms"`
	// Offset int64 is the offset in bytes in the session file
	Offset int64 `json:"offset"`
	// EventIndex is the global event index
	EventIndex int64 `json:"ei"`
	// ChunkIndex is the global chunk index
	ChunkIndex int64 `json:"ci"`
}

// gzipWriter wraps file, on close close both gzip writer and file
type gzipWriter struct {
	*gzip.Writer
	file *os.File
}

// Close closes gzip writer and file
func (f *gzipWriter) Close() error {
	var errors []error
	errors = append(errors, f.Writer.Close())
	f.Writer.Reset(ioutil.Discard)
	writerPool.Put(f.Writer)
	f.Writer = nil
	errors = append(errors, f.file.Close())
	return trace.NewAggregate(errors...)
}

// writerPool is a sync.Pool for shared gzip writers.
// each gzip writer allocates a lot of memory
// so it makes sense to reset the writer and reuse the
// internal buffers to avoid too many objects on the heap
var writerPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(ioutil.Discard, gzip.BestSpeed)
		return w
	},
}

func newGzipWriter(file *os.File) *gzipWriter {
	g := writerPool.Get().(*gzip.Writer)
	g.Reset(file)
	return &gzipWriter{
		Writer: g,
		file:   file,
	}
}

// gzipReader wraps file, on close close both gzip writer and file
type gzipReader struct {
	io.ReadCloser
	file io.Closer
}

// Close closes file and gzip writer
func (f *gzipReader) Close() error {
	var errors []error
	errors = append(errors, f.ReadCloser.Close())
	errors = append(errors, f.file.Close())
	return trace.NewAggregate(errors...)
}

func newGzipReader(file *os.File) (*gzipReader, error) {
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gzipReader{
		ReadCloser: reader,
		file:       file,
	}, nil
}
