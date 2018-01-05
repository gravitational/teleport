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
	"encoding/json"
	"fmt"
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

// SessionLogger is an interface that all session loggers must implement.
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

	// WriteChunk takes a stream of bytes (usually the output from a session
	// terminal) and writes it into a "stream file", for future replay of
	// interactive sessions.
	WriteChunk(chunk *SessionChunk) (written int, err error)
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
	// AuditLog is the audit log
	AuditLog *AuditLog
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
	eventsFile *os.File
	chunksFile *os.File

	lastEventIndex int64
	lastChunkIndex int64

	// recordSessions controls if sessions are recorded along with audit events.
	recordSessions bool
}

// LogEvent logs an event associated with this session
func (sl *DiskSessionLogger) LogEvent(fields EventFields) error {
	panic("does  not work")
}

// Close is called when clients close on the requested "session writer".
// We ignore their requests because this writer (file) should be closed only
// when the session logger is closed
func (sl *DiskSessionLogger) Close() error {
	sl.Debugf("Close")
	return nil
}

// Finalize is called by the session when it's closing. This is where we're
// releasing audit resources associated with the session
func (sl *DiskSessionLogger) Finalize() error {
	sl.Lock()
	defer sl.Unlock()

	return sl.finalize()
}

func (sl *DiskSessionLogger) finalize() error {
	auditOpenFiles.Dec()

	if sl.indexFile != nil {
		sl.indexFile.Close()
	}

	if sl.chunksFile != nil {
		sl.chunksFile.Close()
	}

	if sl.eventsFile != nil {
		sl.eventsFile.Close()
	}

	return nil
}

// eventsFileName consists of session id and the first global event index recorded there
func eventsFileName(dataDir string, sessionID session.ID, eventIndex int64) string {
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.events", sessionID.String(), eventIndex))
}

// chunksFileName consists of session id and the first global offset recorded
func chunksFileName(dataDir string, sessionID session.ID, offset int64) string {
	return filepath.Join(dataDir, fmt.Sprintf("%v-%v.chunks", sessionID.String(), offset))
}

func (sl *DiskSessionLogger) openEventsFile(eventIndex int64) error {
	if sl.eventsFile != nil {
		err := sl.eventsFile.Close()
		if err != nil {
			sl.Warningf("Failed to close file: %v", trace.DebugReport(err))
		}
	}
	eventsFileName := eventsFileName(sl.DataDir, sl.SessionID, eventIndex)

	// udpate the index file to write down that new events file has been created
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
	sl.eventsFile, err = os.OpenFile(eventsFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
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
	sl.chunksFile, err = os.OpenFile(chunksFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WriteChunk takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *DiskSessionLogger) WriteChunk(chunk *SessionChunk) (written int, err error) {
	sl.Lock()
	defer sl.Unlock()

	// this section enforces the following invariant:
	// a single events file only contains successive events
	if sl.lastEventIndex == -1 || chunk.EventIndex-1 != sl.lastEventIndex {
		if err := sl.openEventsFile(chunk.EventIndex); err != nil {
			return -1, trace.Wrap(err)
		}
	}
	sl.lastEventIndex = chunk.EventIndex

	if chunk.EventType != SessionPrintEvent {
		if chunk.EventType == SessionEndEvent {
			defer sl.closeLogger()
		}
		var fields EventFields
		err := json.Unmarshal(chunk.Data, &fields)
		if err != nil {
			return -1, trace.Wrap(err)
		}
		fields[EventIndex] = chunk.EventIndex
		fields[EventTime] = sl.Clock.Now().In(time.UTC).Round(time.Millisecond)
		fields[EventType] = chunk.EventType
		data, err := json.Marshal(fields)
		if err != nil {
			return -1, trace.Wrap(err)
		}
		if err := sl.AuditLog.emitAuditEvent(chunk.EventType, fields); err != nil {
			return -1, trace.Wrap(err)
		}
		return fmt.Fprintln(sl.eventsFile, string(data))
	}
	if !sl.RecordSessions {
		return len(chunk.Data), nil
	}
	eventStart := time.Unix(0, chunk.Time).In(time.UTC).Round(time.Millisecond)
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
	_, err = fmt.Fprintln(sl.eventsFile, string(bytes))
	if err != nil {
		return -1, trace.Wrap(err)
	}
	return sl.chunksFile.Write(chunk.Data)
}

func (sl *DiskSessionLogger) closeLogger() {
	sl.AuditLog.removeLogger(sl.SessionID.String())
	if err := sl.finalize(); err != nil {
		log.Error(err)
	}
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
