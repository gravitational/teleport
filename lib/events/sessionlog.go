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
	"sync/atomic"
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
	writtenBytes, err := ReadWrittenBytes(cfg.EventsFileName)
	if err != nil {
		return nil, trace.Wrap(err)
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
		writtenBytes: writtenBytes,
		sid:          cfg.SessionID,
		streamFile:   fstream,
		eventsFile:   fevents,
		clock:        cfg.Clock,
		createdTime:  cfg.Clock.Now().In(time.UTC).Round(time.Second),
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

	// counter of how many bytes have been written during this session
	writtenBytes int64

	// clock provides real of fake clock (for tests)
	clock clockwork.Clock

	createdTime time.Time
}

// LogEvent logs an event associated with this session
func (sl *DiskSessionLogger) LogEvent(fields EventFields) {
	sl.logEvent(fields, time.Time{})
}

// ReadWritternBytes reads last written bytes offset from the events file
func ReadWrittenBytes(fileName string) (int64, error) {
	lastEvent, err := ReadLastEvent(fileName)
	if err != nil {
		// empty file, no last event found or the file was
		// not created yet
		if trace.IsNotFound(err) {
			return 0, nil
		}
		return -1, trace.Wrap(err)
	}
	bytes, ok := lastEvent[SessionByteOffset].(float64)
	if !ok {
		return -1, trace.BadParameter("expected float64, got %T", lastEvent[SessionByteOffset])
	}
	return int64(bytes), nil
}

// ReadLastEvent reads last event from the file, it opens
// the file in read only mode and closes it after
func ReadLastEvent(fileName string) (EventFields, error) {
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
	var lastLine []byte
	for i := len(lines) - 1; i > 0; i-- {
		lastLine = bytes.TrimSpace(lines[i])
		if len(lastLine) != 0 {
			break
		}
	}
	if len(lastLine) == 0 {
		return nil, trace.BadParameter("file is filled with empty newlines")
	}
	var fields EventFields
	if err = json.Unmarshal(lastLine, &fields); err != nil {
		return nil, trace.Wrap(err)
	}
	return fields, nil
}

// LogEvent logs an event associated with this session
func (sl *DiskSessionLogger) logEvent(fields EventFields, start time.Time) {
	sl.Lock()
	defer sl.Unlock()

	// add "bytes written" counter:
	fields[SessionByteOffset] = atomic.LoadInt64(&sl.writtenBytes)

	// add "milliseconds since" timestamp:
	var now time.Time
	if start.IsZero() {
		now = sl.clock.Now().In(time.UTC).Round(time.Millisecond)
	} else {
		now = start.In(time.UTC).Round(time.Millisecond)
	}

	fields[SessionEventTimestamp] = int(now.Sub(sl.createdTime).Nanoseconds() / 1000000)
	fields[EventTime] = now

	line := eventToLine(fields)

	if sl.eventsFile != nil {
		_, err := fmt.Fprintln(sl.eventsFile, line)
		if err != nil {
			log.Error(err)
		}
	}
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
	if sl.streamFile == nil {
		err := trace.Errorf("session %v error: attempt to write to a closed file", sl.sid)
		return 0, trace.Wrap(err)
	}
	if written, err = sl.streamFile.Write(chunk.Data); err != nil {
		log.Error(err)
		return written, trace.Wrap(err)
	}

	// log this as a session event (but not more often than once a sec)
	sl.logEvent(EventFields{
		EventType:              SessionPrintEvent,
		SessionPrintEventBytes: len(chunk.Data),
	}, time.Unix(0, chunk.Time))

	// increase the total lengh of the stream
	atomic.AddInt64(&sl.writtenBytes, int64(len(chunk.Data)))
	return written, nil
}
