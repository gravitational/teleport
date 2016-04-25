package events

import (
	"encoding/json"
	"fmt"
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
	LogfilePrefix = ".tele.log"

	// DefaultRotationPeriod defines how frequently to rotate the log file
	DefaultRotationPeriod = (time.Hour * 24)
)

const (
	// SessionEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionStartEvent = "session.start"

	// SessionEndEvent indicates taht a session has ended
	SessionEndEvent   = "session.end"
	SessionEventID    = "sid"
	SessionEventLogin = "login"
	SessionLocalAddr  = "addr.local"
	SessionRemoveAddr = "addr.remote"

	// Join & Leave events indicate when someone joins/leaves a session
	SessionJoinEvent  = "session.join"
	SessionLeaveEvent = "session.leave"

	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent       = "exec"
	ExecEventOutput = "output"
	ExecEventCode   = "exitCode"
	ExecEventError  = "exitError"

	// Port forwarding event
	PortForwardEvent      = "port"
	PortForwardLogin      = "login"
	PortForwardAddr       = "addr"
	PortForwardLocalAddr  = "addr.local"
	PortForwardRemoteAddr = "addr.remote"

	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent   = "auth"
	AuthAttemptUser    = "user"
	AuthAttemptSuccess = "success"
	AuthAttemptErr     = "error"

	// SCPEvent means data transfer that occured on the server
	SCPEvent      = "scp"
	SCPPath       = "path"
	SCPLengh      = "len"
	SCPLocalAddr  = "addr.local"
	SCPRemoteAddr = "addr.remote"
	SCPLogin      = "login"
	SCPAction     = "action"

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent = "resize"
	ResizeSize  = "size" // expressed as 'W:H'
)

// EventFields instance is attached to every logged event
type EventFields map[string]interface{}

func (f EventFields) GetString(key string) (string, bool) {
	val, found := f[key]
	if !found {
		return "", false
	}
	return val.(string), true
}

func (f EventFields) GetInt(key string) (int, bool) {
	val, found := f[key]
	if !found {
		return -1, false
	}
	return val.(int), true
}

// AuditLogI is the primary (and the only external-facing) interface for AUditLogger.
// If you wish to implement a different kind of logger (not filesystem-based), you
// have to implement this interface
type AuditLogI interface {
	EmitAuditEvent(eventType string, fields EventFields) error
}

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
}

// LogEvent logs an event associated with this session
func (sl *SessionLogger) LogEvent(line string) {
	sl.Lock()
	sl.Unlock()
	_, err := fmt.Fprintln(sl.eventsFile, line)
	if err != nil {
		logrus.Error(err)
	}
}

// Close() is called when a session is closed. This releases resources
// owned by the session logger
func (sl *SessionLogger) Close() {
	sl.once.Do(func() {
		sl.streamFile.Close()
		sl.eventsFile.Close()
		sl.streamFile = nil
		sl.eventsFile = nil
	})
}

// WriteStream takes a stream of bytes (usually the output from a session terminal)
// and writes it into a "stream file", for future replay of interactive sessions.
func (sl *SessionLogger) WriteStream(bytes []byte) error {
	if sl.streamFile == nil {
		err := trace.Errorf("session %v error: attempt to write to a closed file", sl.sid)
		logrus.Error(err)
		return err
	}
	if _, err := sl.streamFile.Write(bytes); err != nil {
		logrus.Error(err)
		return trace.Wrap(err)
	}
	return nil
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

	// line is the text to be logged
	line := l.eventToLine(eventType, fields)

	// if this event is associated with a session -> forward it to the session log as well
	sessionId, found := fields.GetString(SessionEventID)
	if found {
		sl := l.LoggerFor(session.ID(sessionId))
		if sl != nil {
			delete(fields, SessionEventID)
			sl.LogEvent(l.eventToLine(eventType, fields))
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
	streamFname := filepath.Join(sdir, fmt.Sprintf("%s.session.bytes", sid))
	fstream, err := os.OpenFile(streamFname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	// create a new session file:
	eventsFname := filepath.Join(sdir, fmt.Sprintf("%s.session", sid))
	fevents, err := os.OpenFile(eventsFname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		logrus.Error(err)
		return nil
	}

	l.Lock()
	defer l.Unlock()
	sl = &SessionLogger{
		sid:        sid,
		streamFile: fstream,
		eventsFile: fevents,
	}
	l.loggers[sid] = sl
	return sl
}

// eventToLine helper creates a loggable line/string for a given event
func (l *AuditLog) eventToLine(eventType string, fields EventFields) string {
	jbytes, err := json.Marshal(fields)
	jsonString := string(jbytes)
	if err != nil {
		logrus.Error(err)
		jsonString = `"error"`
	}
	now := l.TimeSource().In(time.UTC).Round(time.Second).String()
	return fmt.Sprintf("%s,%s,%s", now, eventType, jsonString)
}
