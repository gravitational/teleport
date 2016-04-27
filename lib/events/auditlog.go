/*
Copyright 2015 Gravitational, Inc.

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

/*
This audit log implementation uses filesystem backend.
"Implements" means it implements events.AuditLogI interface (see events/api.go)

The main log files are saved as:
	/var/lib/teleport/log/<date>.log

Each session has its own session log stored as two files
	/var/lib/teleport/log/<session-id>.session.log
	/var/lib/teleport/log/<session-id>.session.bytes

Where:
	- .session.log   (same events as in the main log, but related to the session)
	- .session.bytes (recorded session bytes: PTY IO)

The log file is rotated every 24 hours. The old files must be cleaned
up or archived by an external tool.

Log file format:
utc_date,action,json_fields

Common JSON fields
- user       : teleport user
- login      : server OS login, the user logged in as
- addr.local : server address:port
- addr.remote: connected client's address:port
- sid        : session ID (GUID format)

Examples:
2016-04-25 22:37:29 +0000 UTC,session.start,{"addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:35732","login":"root","sid":"4a9d97de-0b36-11e6-a0b3-d8cb8ae5080e","user":"vincent"}
2016-04-25 22:54:31 +0000 UTC,exec,{"addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:35949","command":"-bash -c ls /","login":"root","user":"vincent"}
*/
package events

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

const (
	// SessionLogsDir is a subdirectory inside the eventlog data dir
	// where all session-specific logs and streams are stored, like
	// in /var/lib/teleport/logs/sessions
	SessionLogsDir = "sessions"

	// LogfilePrefix defines the ending of the daily event log file
	LogfilePrefix = ".log"

	// SessionLogPrefix defines the endof of session log files
	SessionLogPrefix = ".session.log"

	// SessionStreamPrefix defines the ending of session stream files,
	// that's where interactive PTY I/O is saved.
	SessionStreamPrefix = ".session.bytes"

	// DefaultRotationPeriod defines how frequently to rotate the log file
	DefaultRotationPeriod = (time.Hour * 24)
)

type TimeSourceFunc func() time.Time

// AuditLog is a new combined facility to record Teleport events and
// sessions. It implements these interfaces:
//	- events.Log
//	- recorder.Recorder
type AuditLog struct {
	sync.Mutex
	loggers map[session.ID]*SessionLogger
	dataDir string

	// file is the current global event log file. As the time goes
	// on, it will be replaced by a new file every day
	file *os.File

	// fileTime is a rounded (to a day, by default) timestamp of the
	// currently opened file
	fileTime time.Time

	// lastEvents keeps a map of most recent event
	// of each types. Useful for testing and debugging
	RecentEvents map[string]EventFields

	// RotationPeriod defines how frequently to rotate the log file
	RotationPeriod time.Duration

	// same as time.Now(), but helps with testing
	TimeSource TimeSourceFunc
}

// BaseSessionLogger implements the common features of a session logger. The imporant
// property of the base logger is that it never fails and can be used as a fallback
// implementation behind more sophisticated loggers
type SessionLogger struct {
	sync.Mutex

	sid  session.ID
	once sync.Once

	// eventsFile stores logged events, just like the main logger, except
	// these are all associated with this session
	eventsFile *os.File
	// streamFile stores bytes from the session terminal I/O for replaying
	streamFile *os.File

	// counter of how many bytes have been written during this session
	writtenBytes int

	// same as time.Now(), but helps with testing
	timeSource TimeSourceFunc

	createdTime time.Time
}

// LogEvent logs an event associated with this session
func (sl *SessionLogger) LogEvent(fields EventFields) {
	// space optimization: no need to log sessionID into the session log (that log
	// itself knows which session it belogs to)
	delete(fields, SessionEventID)

	// add "bytes written" counter:
	fields[SessionEventBytes] = sl.writtenBytes
	// add "seconds since" timestamp:
	now := sl.timeSource().In(time.UTC).Round(time.Second)
	fields[SessionEventTimestamp] = int(now.Sub(sl.createdTime).Seconds())

	line := eventToLine(fields)

	sl.Lock()
	sl.Unlock()
	_, err := fmt.Fprintln(sl.eventsFile, line)
	if err != nil {
		logrus.Error(err)
	}
}

// Close() is called when a session is closed. This releases resources
// owned by the session logger
func (sl *SessionLogger) Close() error {
	sl.once.Do(func() {
		sl.streamFile.Close()
		sl.eventsFile.Close()
		sl.streamFile = nil
		sl.eventsFile = nil
	})
	return nil
}

// Write takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *SessionLogger) Write(bytes []byte) (written int, err error) {
	if sl.streamFile == nil {
		err := trace.Errorf("session %v error: attempt to write to a closed file", sl.sid)
		logrus.Error(err)
		return 0, err
	}
	if written, err = sl.streamFile.Write(bytes); err != nil {
		logrus.Error(err)
		return written, trace.Wrap(err)
	}
	logrus.Infof("sessionLogger %d bytes -> %v", written, sl.streamFile.Name())

	// increase the total lengh of the stream
	lockedMath := func() {
		sl.Lock()
		defer sl.Unlock()
		sl.writtenBytes += len(bytes)
	}
	lockedMath()

	// log this as a session event (but not more often than once a sec)
	sl.LogEvent(EventFields{
		EventType:         SessionEventTermio,
		SessionEventBytes: sl.writtenBytes})
	return written, nil
}

// Creates and returns a new Audit Log oboject whish will store its logfiles
// in a given directory> When 'testMode' is set to True, additional bookeeing
// to assist with unit testing will be performed
func NewAuditLog(dataDir string, testMode bool) (*AuditLog, error) {
	// create a directory for session logs:
	if err := os.MkdirAll(dataDir, 0770); err != nil {
		logrus.Error(err)
		return nil, trace.Wrap(err)
	}
	makeRecentEvents := func() map[string]EventFields {
		if testMode {
			return make(map[string]EventFields)
		}
		return nil
	}
	return &AuditLog{
		loggers:        make(map[session.ID]*SessionLogger, 0),
		dataDir:        dataDir,
		RecentEvents:   makeRecentEvents(),
		RotationPeriod: DefaultRotationPeriod,
		TimeSource:     time.Now,
	}, nil
}

// GetSessionWriter returns a writer interface for the given session. SSH Server
// uses this to stream active sessions into the audit log
func (l *AuditLog) GetSessionWriter(sid session.ID) (io.WriteCloser, error) {
	logrus.Infof("audit.log: getSessionWriter(%v)", sid)
	sl := l.LoggerFor(sid)
	if sl == nil {
		logrus.Warnf("audit.log: no session writer for %s", sid)
		return nil, nil
	}
	return sl, nil
}

// GetSessionReader returns a reader which console and web clients request
// to receive a live stream of a given session
func (l *AuditLog) GetSessionReader(sid session.ID) (io.ReadCloser, error) {
	logrus.Infof("audit.log: getSessionReader(%v)", sid)
	fstream, err := os.OpenFile(l.streamFileName(sid), os.O_RDONLY, 0640)
	if err != nil {
		logrus.Error(err)
		return nil, trace.Wrap(err)
	}
	return fstream, nil
}

// EmitAuditEvent adds a new event to the log. Part of auth.AuditLogI interface.
func (l *AuditLog) EmitAuditEvent(eventType string, fields EventFields) error {
	// keep the most recent event of every kind for testing purposes:
	if l.RecentEvents != nil {
		l.RecentEvents[eventType] = fields
	}

	// see if the log needs to be rotated
	if err := l.rotateLog(); err != nil {
		logrus.Error(err)
	}

	// set event type and time:
	fields[EventType] = eventType
	fields[EventTime] = l.TimeSource().In(time.UTC).Round(time.Second)

	// line is the text to be logged
	line := eventToLine(fields)

	// if this event is associated with a session -> forward it to the session log as well
	sessionId, found := fields.GetString(SessionEventID)
	if found {
		sl := l.LoggerFor(session.ID(sessionId))
		if sl != nil {
			sl.LogEvent(fields)

			// Session ended? Get rid of the session logger then:
			if eventType == SessionEndEvent {
				logrus.Infof("audit log: removing session logger for SID=%v", sessionId)
				delete(l.loggers, session.ID(sessionId))
				sl.Close()
			}
		}
	}
	// log it to the main log file:
	if l.file != nil {
		fmt.Fprintln(l.file, line)
	}
	return nil
}

// rotateLog() checks if the current log file is older than a given duration,
// and if it is, closes it and opens a new one
func (l *AuditLog) rotateLog() (err error) {
	// determine the timestamp for the current log file
	fileTime := l.TimeSource().In(time.UTC).Round(l.RotationPeriod)

	openLogFile := func() error {
		l.Lock()
		defer l.Unlock()
		logfname := filepath.Join(l.dataDir,
			fileTime.Format("2006-01-02.15:04:05")+LogfilePrefix)
		l.file, err = os.OpenFile(logfname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			logrus.Error(err)
		}
		l.fileTime = fileTime
		return trace.Wrap(err)
	}

	// need to create a log file?
	if l.file == nil {
		return openLogFile()
	}

	// time to advance the logfile?
	if l.fileTime.Before(fileTime) {
		l.file.Close()
		return openLogFile()
	}
	return nil
}

// Closes the audit log, which inluces closing all file handles and releasing
// all session loggers
func (l *AuditLog) Close() error {
	l.Lock()
	defer l.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	for sid, logger := range l.loggers {
		logger.Close()
		delete(l.loggers, sid)
	}
	return nil
}

// streamFileName helper determins the name of the stream file for a given
// session by its ID
func (l *AuditLog) streamFileName(sid session.ID) string {
	return filepath.Join(
		l.dataDir,
		SessionLogsDir,
		fmt.Sprintf("%s%s", sid, SessionStreamPrefix))
}

// LoggerFor creates a logger for a specified session. Session loggers allow
// to group all events into special "session log files" for easier audit
func (l *AuditLog) LoggerFor(sid session.ID) (sl *SessionLogger) {
	sl, ok := l.loggers[sid]
	if ok {
		return sl
	}
	// make sure session logs dir is present
	sdir := filepath.Join(l.dataDir, SessionLogsDir)
	if err := os.MkdirAll(sdir, 0770); err != nil {
		logrus.Error(err)
		return nil
	}
	// create a new session stream file:
	fstream, err := os.OpenFile(l.streamFileName(sid), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	// create a new session file:
	eventsFname := filepath.Join(sdir, fmt.Sprintf("%s%s", sid, SessionLogPrefix))
	fevents, err := os.OpenFile(eventsFname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	sl = &SessionLogger{
		sid:         sid,
		streamFile:  fstream,
		eventsFile:  fevents,
		timeSource:  l.TimeSource,
		createdTime: l.TimeSource().In(time.UTC).Round(time.Second),
	}
	l.Lock()
	defer l.Unlock()
	l.loggers[sid] = sl
	return sl
}

// eventToLine helper creates a loggable line/string for a given event
func eventToLine(fields EventFields) string {
	jbytes, err := json.Marshal(fields)
	jsonString := string(jbytes)
	if err != nil {
		logrus.Error(err)
		jsonString = ""
	}
	return jsonString
}
