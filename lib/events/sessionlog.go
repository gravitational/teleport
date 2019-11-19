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
	"archive/tar"
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
	"github.com/pborman/uuid"
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
	// Namespace is logger namespace
	Namespace string
	// ServerID is a server ID
	ServerID string
}

func (cfg *DiskSessionLoggerConfig) CheckAndSetDefaults() error {
	return nil
}

// NewDiskSessionLogger creates new disk based session logger
func NewDiskSessionLogger(cfg DiskSessionLoggerConfig) (*DiskSessionLogger, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	var err error

	sessionDir := filepath.Join(cfg.DataDir, cfg.ServerID, SessionLogsDir, cfg.Namespace)
	indexFile, err := os.OpenFile(
		filepath.Join(sessionDir, fmt.Sprintf("%v.index", cfg.SessionID.String())), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
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
		sessionDir:     sessionDir,
		indexFile:      indexFile,
		lastEventIndex: -1,
		lastChunkIndex: -1,

		enhancedIndexes: map[string]int64{
			SessionCommandEvent: -1,
			SessionDiskEvent:    -1,
			SessionNetworkEvent: -1,
		},
		enhancedFiles: map[string]*gzipWriter{
			SessionCommandEvent: nil,
			SessionDiskEvent:    nil,
			SessionNetworkEvent: nil,
		},
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

	sid        session.ID
	sessionDir string

	indexFile  *os.File
	eventsFile *gzipWriter
	chunksFile *gzipWriter

	lastEventIndex int64
	lastChunkIndex int64

	enhancedIndexes map[string]int64
	enhancedFiles   map[string]*gzipWriter
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

func openFileForTar(filename string) (*tar.Header, io.ReadCloser, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}

	header, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}

	return header, f, nil
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
	var err error
	var errs []error

	if sl.RecordSessions && sl.chunksFile != nil {
		err = sl.chunksFile.Flush()
		errs = append(errs, err)
	}
	if sl.eventsFile != nil {
		err = sl.eventsFile.Flush()
		errs = append(errs, err)
	}
	for _, eventsFile := range sl.enhancedFiles {
		if eventsFile != nil {
			err = eventsFile.Flush()
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
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

	for _, eventsFile := range sl.enhancedFiles {
		if eventsFile != nil {
			err := eventsFile.Close()
			if err != nil {
				log.Warningf("Failed closing enhanced events file: %v.", err)
			}
		}
	}

	// create a sentinel to signal completion
	signalFile := filepath.Join(sl.sessionDir, fmt.Sprintf("%v.completed", sl.SessionID.String()))
	err := ioutil.WriteFile(signalFile, []byte("completed"), 0640)
	if err != nil {
		log.Warningf("Failed creating signal file: %v.", err)
	}

	return nil
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

func (sl *DiskSessionLogger) openEventsFile(eventIndex int64) error {
	if sl.eventsFile != nil {
		err := sl.eventsFile.Close()
		if err != nil {
			sl.Warningf("Failed to close file: %v", trace.DebugReport(err))
		}
	}
	eventsFileName := eventsFileName(sl.sessionDir, sl.SessionID, "", eventIndex)

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
	chunksFileName := chunksFileName(sl.sessionDir, sl.SessionID, offset)

	// Update the index file to write down that new chunks file has been created.
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

func (sl *DiskSessionLogger) openEnhancedFile(eventType string, eventIndex int64) error {
	eventsFile, ok := sl.enhancedFiles[eventType]
	if !ok {
		return trace.BadParameter("unknown event type: %v", eventType)
	}

	// If an events file is currently open, close it.
	if eventsFile != nil {
		err := eventsFile.Close()
		if err != nil {
			sl.Warningf("Failed to close file: %v.", trace.DebugReport(err))
		}
	}

	// Create a new events file.
	eventsFileName := eventsFileName(sl.sessionDir, sl.SessionID, eventType, eventIndex)

	// If the event is an enhanced event overwrite with the type of enhanced event.
	var indexType string
	switch eventType {
	case SessionCommandEvent, SessionDiskEvent, SessionNetworkEvent:
		indexType = eventType
	default:
		indexType = fileTypeEvents
	}

	// Update the index file to write down that new events file has been created.
	data, err := json.Marshal(indexEntry{
		FileName: filepath.Base(eventsFileName),
		Type:     indexType,
		Index:    eventIndex,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintf(sl.indexFile, "%v\n", string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	// Open and store new file for writing events.
	file, err := os.OpenFile(eventsFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	sl.enhancedFiles[eventType] = newGzipWriter(file)

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

// PrintEventFromChunk returns a print event converted from session chunk.
func PrintEventFromChunk(chunk *SessionChunk) printEvent {
	return printEvent{
		Start:             time.Unix(0, chunk.Time).In(time.UTC).Round(time.Millisecond),
		Type:              SessionPrintEvent,
		Bytes:             int64(len(chunk.Data)),
		DelayMilliseconds: chunk.Delay,
		Offset:            chunk.Offset,
		EventIndex:        chunk.EventIndex,
		ChunkIndex:        chunk.ChunkIndex,
	}
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
		fields[EventID] = uuid.New()
	}
	return fields, nil
}

func (sl *DiskSessionLogger) writeChunk(sessionID string, chunk *SessionChunk) (int, error) {
	switch chunk.EventType {
	// Timing events for TTY playback go to both a chunks file (the raw bytes) as
	// well as well as the events file (structured events).
	case SessionPrintEvent:
		// If the session are not being recorded, don't capture any print events.
		if !sl.RecordSessions {
			return len(chunk.Data), nil
		}

		n, err := sl.writeEventChunk(sessionID, chunk)
		if err != nil {
			return n, trace.Wrap(err)
		}
		n, err = sl.writePrintChunk(sessionID, chunk)
		if err != nil {
			return n, trace.Wrap(err)
		}
		return n, nil
	// Enhanced session recording events all go to their own events files.
	case SessionCommandEvent, SessionDiskEvent, SessionNetworkEvent:
		return sl.writeEnhancedChunk(sessionID, chunk)
	// All other events get put into the general events file. These are events like
	// session.join, session.end, etc.
	default:
		return sl.writeEventChunk(sessionID, chunk)
	}
}

func (sl *DiskSessionLogger) writeEventChunk(sessionID string, chunk *SessionChunk) (int, error) {
	var bytes []byte
	var err error

	// This section enforces the following invariant: a single events file only
	// contains successive events. If means if an event arrives that is older or
	// newer than the next expected event, a new file for that chunk is created.
	if sl.lastEventIndex == -1 || chunk.EventIndex-1 != sl.lastEventIndex {
		if err := sl.openEventsFile(chunk.EventIndex); err != nil {
			return -1, trace.Wrap(err)
		}
	}

	// Update index for the last event that was processed.
	sl.lastEventIndex = chunk.EventIndex

	// Marshal event. Note that print events are marshalled somewhat differently
	// than all other events.
	switch chunk.EventType {
	case SessionPrintEvent:
		bytes, err = json.Marshal(PrintEventFromChunk(chunk))
		if err != nil {
			return -1, trace.Wrap(err)
		}
	default:
		// Convert to a marshallable event.
		fields, err := EventFromChunk(sessionID, chunk)
		if err != nil {
			return -1, trace.Wrap(err)
		}
		bytes, err = json.Marshal(fields)
		if err != nil {
			return -1, trace.Wrap(err)
		}
	}

	n, err := sl.eventsFile.Write(append(bytes, '\n'))
	if err != nil {
		return -1, trace.Wrap(err)
	}
	return n, nil
}

func (sl *DiskSessionLogger) writePrintChunk(sessionID string, chunk *SessionChunk) (int, error) {
	// This section enforces the following invariant: a single events file only
	// contains successive events. If means if an event arrives that is older or
	// newer than the next expected event, a new file for that chunk is created.
	if sl.lastChunkIndex == -1 || chunk.ChunkIndex-1 != sl.lastChunkIndex {
		if err := sl.openChunksFile(chunk.Offset); err != nil {
			return -1, trace.Wrap(err)
		}
	}

	// Update index for the last chunk that was processed.
	sl.lastChunkIndex = chunk.ChunkIndex

	return sl.chunksFile.Write(chunk.Data)
}

func (sl *DiskSessionLogger) writeEnhancedChunk(sessionID string, chunk *SessionChunk) (int, error) {
	var bytes []byte
	var err error

	// Extract last index of particular event (command, disk, network).
	lastIndex, ok := sl.enhancedIndexes[chunk.EventType]
	if !ok {
		return -1, trace.BadParameter("unknown event type: %v", chunk.EventType)
	}

	// This section enforces the following invariant: a single events file only
	// contains successive events. If means if an event arrives that is older or
	// newer than the next expected event, a new file for that chunk is created.
	if lastIndex == -1 || chunk.EventIndex-1 != lastIndex {
		if err := sl.openEnhancedFile(chunk.EventType, chunk.EventIndex); err != nil {
			return -1, trace.Wrap(err)
		}
	}

	// Update index for the last event that was processed.
	sl.enhancedIndexes[chunk.EventType] = chunk.EventIndex

	// Convert to a marshallable event.
	fields, err := EventFromChunk(sessionID, chunk)
	if err != nil {
		return -1, trace.Wrap(err)
	}
	bytes, err = json.Marshal(fields)
	if err != nil {
		return -1, trace.Wrap(err)
	}

	// Write event to appropriate file.
	eventsFile, ok := sl.enhancedFiles[chunk.EventType]
	if !ok {
		return -1, trace.BadParameter("unknown event type: %v", chunk.EventType)
	}
	n, err := eventsFile.Write(append(bytes, '\n'))
	if err != nil {
		return -1, trace.Wrap(err)
	}
	return n, nil
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
	if f.Writer != nil {
		errors = append(errors, f.Writer.Close())
		f.Writer.Reset(ioutil.Discard)
		writerPool.Put(f.Writer)
		f.Writer = nil
	}
	if f.file != nil {
		errors = append(errors, f.file.Close())
		f.file = nil
	}
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
	if f.ReadCloser != nil {
		errors = append(errors, f.ReadCloser.Close())
		f.ReadCloser = nil
	}
	if f.file != nil {
		errors = append(errors, f.file.Close())
		f.file = nil
	}
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

const (
	// eventsSuffix is the suffix of the archive that contians session events.
	eventsSuffix = "events.gz"

	// chunksSuffix is the suffix of the archive that contains session chunks.
	chunksSuffix = "chunks.gz"
)
