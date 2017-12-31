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
	"bytes"
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

// DELETE IN: 2.6.0
// CompatSessionLogger is used only during upgrades from 2.4.0 to 2.5.0
// Should be deleted in 2.6.0 releases
// CompatSessionLoggerConfig sets up parameters for disk session logger
// associated with the session ID
type CompatSessionLoggerConfig struct {
	// SessionID is the session id of the logger
	SessionID session.ID
	// DataDir is data directory for session events files
	DataDir string
	// Clock is the clock replacement
	Clock clockwork.Clock
	// RecordSessions controls if sessions are recorded along with audit events.
	RecordSessions bool
}

// DELETE IN: 2.6.0
// CompatSessionLogger is used only during upgrades from 2.4.0 to 2.5.0
// Should be deleted in 2.6.0 releases
// NewCompatSessionLogger creates new disk based session logger
func NewCompatSessionLogger(cfg CompatSessionLoggerConfig) (*CompatSessionLogger, error) {
	var err error

	lastPrintEvent, err := readLastPrintEvent(eventsFileName(cfg.DataDir, cfg.SessionID, 0))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// no last event is ok
		lastPrintEvent = nil
	}

	indexFile, err := os.OpenFile(filepath.Join(cfg.DataDir, fmt.Sprintf("%v.index", cfg.SessionID.String())), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionLogger := &CompatSessionLogger{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
			trace.ComponentFields: log.Fields{
				"sid": cfg.SessionID,
			},
		}),
		indexFile:                 indexFile,
		CompatSessionLoggerConfig: cfg,
		lastPrintEvent:            lastPrintEvent,
	}
	return sessionLogger, nil
}

// CompatSessionLogger implements a disk based session logger. The imporant
// property of the disk based logger is that it never fails and can be used as
// a fallback implementation behind more sophisticated loggers.
type CompatSessionLogger struct {
	CompatSessionLoggerConfig

	*log.Entry

	sync.Mutex

	indexFile  *os.File
	eventsFile *os.File
	chunksFile *os.File

	// lastPrintEvent is the last written session event
	lastPrintEvent *printEvent
}

// LogEvent logs an event associated with this session
func (sl *CompatSessionLogger) LogEvent(fields EventFields) error {
	if err := sl.openEventsFile(); err != nil {
		return trace.Wrap(err)
	}

	if _, ok := fields[EventTime]; !ok {
		fields[EventTime] = sl.Clock.Now().In(time.UTC).Round(time.Millisecond)
	}

	data, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintln(sl.eventsFile, string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// readLastEvent reads last event from the file, it opens
// the file in read only mode and closes it after
func readLastPrintEvent(fileName string) (*printEvent, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if info.Size() == 0 {
		return nil, trace.NotFound("no events found")
	}
	bufSize := int64(512)
	if info.Size() < bufSize {
		bufSize = info.Size()
	}
	buf := make([]byte, bufSize)
	_, err = f.ReadAt(buf, info.Size()-bufSize)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	lines := bytes.Split(buf, []byte("\n"))
	if len(lines) == 0 {
		return nil, trace.BadParameter("expected some lines, got %q", string(buf))
	}
	for i := len(lines) - 1; i > 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var event printEvent
		if err = json.Unmarshal(line, &event); err != nil {
			return nil, trace.Wrap(err)
		}
		if event.Type != SessionPrintEvent {
			continue
		}
		return &event, nil
	}
	return nil, trace.NotFound("no session print events found")
}

// Close is called when clients close on the requested "session writer".
// We ignore their requests because this writer (file) should be closed only
// when the session logger is closed
func (sl *CompatSessionLogger) Close() error {
	sl.Debugf("Close")
	return nil
}

// Finalize is called by the session when it's closing. This is where we're
// releasing audit resources associated with the session
func (sl *CompatSessionLogger) Finalize() error {
	sl.Lock()
	defer sl.Unlock()

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

// WriteChunk takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *CompatSessionLogger) WriteChunk(chunk *SessionChunk) (written int, err error) {
	sl.Lock()
	defer sl.Unlock()

	// when session recording is turned off, don't record the session byte stream
	if sl.RecordSessions == false {
		return len(chunk.Data), nil
	}

	if err := sl.openChunksFile(); err != nil {
		return -1, trace.Wrap(err)
	}

	if written, err = sl.chunksFile.Write(chunk.Data); err != nil {
		return written, trace.Wrap(err)
	}

	err = sl.writePrintEvent(time.Unix(0, chunk.Time), len(chunk.Data))
	return written, trace.Wrap(err)
}

// writePrintEvent logs print event indicating write to the session
func (sl *CompatSessionLogger) writePrintEvent(start time.Time, bytesWritten int) error {
	if err := sl.openEventsFile(); err != nil {
		return trace.Wrap(err)
	}

	start = start.In(time.UTC).Round(time.Millisecond)
	offset := int64(0)
	delayMilliseconds := int64(0)
	if sl.lastPrintEvent != nil {
		offset = sl.lastPrintEvent.Offset + sl.lastPrintEvent.Bytes
		delayMilliseconds = diff(sl.lastPrintEvent.Start, start) + sl.lastPrintEvent.DelayMilliseconds
	}
	event := printEvent{
		Start:             start,
		Type:              SessionPrintEvent,
		Bytes:             int64(bytesWritten),
		DelayMilliseconds: delayMilliseconds,
		Offset:            offset,
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintln(sl.eventsFile, string(bytes))
	if err != nil {
		return trace.Wrap(err)
	}
	sl.lastPrintEvent = &event
	return trace.Wrap(err)
}

func (sl *CompatSessionLogger) openEventsFile() error {
	if sl.eventsFile != nil {
		return nil
	}
	eventsFileName := eventsFileName(sl.DataDir, sl.SessionID, 0)

	// udpate the index file to write down that new events file has been created
	data, err := json.Marshal(indexEntry{
		FileName: filepath.Base(eventsFileName),
		Type:     fileTypeEvents,
		Index:    0,
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

func (sl *CompatSessionLogger) openChunksFile() error {
	if sl.chunksFile != nil {
		return nil
	}
	// chunksFileName consists of session id and the first global offset recorded
	chunksFileName := chunksFileName(sl.DataDir, sl.SessionID, 0)

	// udpate the index file to write down that new chunks file has been created
	data, err := json.Marshal(indexEntry{
		FileName: filepath.Base(chunksFileName),
		Type:     fileTypeChunks,
		Offset:   0,
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
