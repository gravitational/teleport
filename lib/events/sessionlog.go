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
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"

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

// diskSessionLogger implements a disk based session logger. The imporant
// property of the disk based logger is that it never fails and can be used as
// a fallback implementation behind more sophisticated loggers.
type diskSessionLogger struct {
	sync.Mutex

	sid session.ID

	// eventsFile stores logged events, just like the main logger, except
	// these are all associated with this session
	eventsFile *os.File

	// streamFile stores bytes from the session terminal I/O for replaying
	streamFile *os.File

	// counter of how many bytes have been written during this session
	writtenBytes int64

	// same as time.Now(), but helps with testing
	timeSource TimeSourceFunc

	createdTime time.Time
}

// LogEvent logs an event associated with this session
func (sl *diskSessionLogger) LogEvent(fields EventFields) {
	sl.logEvent(fields, time.Time{})
}

// LogEvent logs an event associated with this session
func (sl *diskSessionLogger) logEvent(fields EventFields, start time.Time) {
	sl.Lock()
	defer sl.Unlock()

	// add "bytes written" counter:
	fields[SessionByteOffset] = atomic.LoadInt64(&sl.writtenBytes)

	// add "milliseconds since" timestamp:
	var now time.Time
	if start.IsZero() {
		now = sl.timeSource().In(time.UTC).Round(time.Millisecond)
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
func (sl *diskSessionLogger) Close() error {
	log.Infof("sessionLogger.Close(sid=%s)", sl.sid)
	return nil
}

// Finalize is called by the session when it's closing. This is where we're
// releasing audit resources associated with the session
func (sl *diskSessionLogger) Finalize() error {
	sl.Lock()
	defer sl.Unlock()
	if sl.streamFile != nil {
		auditOpenFiles.Dec()
		log.Infof("sessionLogger.Finalize(sid=%s)", sl.sid)
		sl.streamFile.Close()
		sl.eventsFile.Close()
		sl.streamFile = nil
		sl.eventsFile = nil
	}
	return nil
}

// WriteChunk takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *diskSessionLogger) WriteChunk(chunk *SessionChunk) (written int, err error) {
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
