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
	LogEvent(fields EventFields)

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
	// EventsFileName is the events file name
	EventsFileName string
	// StreamFileName is the byte stream file name
	StreamFileName string
	// Clock is the clock replacement
	Clock clockwork.Clock
}

// NewDiskSessionLogger creates new disk based session logger
func NewDiskSessionLogger(cfg DiskSessionLoggerConfig) (*DiskSessionLogger, error) {
	lastPrintEvent, err := readLastPrintEvent(cfg.EventsFileName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// no last event is ok
		lastPrintEvent = nil
	}
	// create a new session stream file:
	fstream, err := os.OpenFile(cfg.StreamFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// create a new session file:
	fevents, err := os.OpenFile(cfg.EventsFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionLogger := &DiskSessionLogger{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
			trace.ComponentFields: log.Fields{
				"sid": cfg.SessionID,
			},
		}),
		sid:            cfg.SessionID,
		streamFile:     fstream,
		eventsFile:     fevents,
		clock:          cfg.Clock,
		lastPrintEvent: lastPrintEvent,
	}
	return sessionLogger, nil
}

// DiskSessionLogger implements a disk based session logger. The imporant
// property of the disk based logger is that it never fails and can be used as
// a fallback implementation behind more sophisticated loggers.
type DiskSessionLogger struct {
	*log.Entry

	sync.Mutex

	sid session.ID

	// eventsFile stores logged events, just like the main logger, except
	// these are all associated with this session
	eventsFile *os.File

	// streamFile stores bytes from the session terminal I/O for replaying
	streamFile *os.File

	// clock provides real of fake clock (for tests)
	clock clockwork.Clock

	// lastPrintEvent is the last written session event
	lastPrintEvent *printEvent
}

// LogEvent logs an event associated with this session
func (sl *DiskSessionLogger) LogEvent(fields EventFields) {
	if _, ok := fields[EventTime]; !ok {
		fields[EventTime] = sl.clock.Now().In(time.UTC).Round(time.Millisecond)
	}

	if sl.eventsFile != nil {
		_, err := fmt.Fprintln(sl.eventsFile, eventToLine(fields))
		if err != nil {
			log.Error(trace.DebugReport(err))
		}
	}
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
func (sl *DiskSessionLogger) Close() error {
	sl.Debugf("Close")
	return nil
}

// Finalize is called by the session when it's closing. This is where we're
// releasing audit resources associated with the session
func (sl *DiskSessionLogger) Finalize() error {
	sl.Lock()
	defer sl.Unlock()
	if sl.streamFile != nil {
		auditOpenFiles.Dec()
		sl.Debug("Finalize")
		sl.streamFile.Close()
		sl.eventsFile.Close()
		sl.streamFile = nil
		sl.eventsFile = nil
	}
	return nil
}

// WriteChunk takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *DiskSessionLogger) WriteChunk(chunk *SessionChunk) (written int, err error) {
	sl.Lock()
	defer sl.Unlock()

	if sl.streamFile == nil || sl.eventsFile == nil {
		return 0, trace.BadParameter("session %v: attempt to write to a closed file", sl.sid)
	}

	if written, err = sl.streamFile.Write(chunk.Data); err != nil {
		return written, trace.Wrap(err)
	}

	err = sl.writePrintEvent(time.Unix(0, chunk.Time), len(chunk.Data))
	return written, trace.Wrap(err)
}

// writePrintEvent logs print event indicating write to the session
func (sl *DiskSessionLogger) writePrintEvent(start time.Time, bytesWritten int) error {
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

func diff(before, after time.Time) int64 {
	d := int64(after.Sub(before) / time.Millisecond)
	if d < 0 {
		return 0
	}
	return d
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
}
