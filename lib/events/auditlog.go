package events

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

const (
	SyslogPriority = syslog.LOG_AUTH | syslog.LOG_NOTICE
	SyslogPrefix   = "teleport"
)

const (
	// SessionEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionEvent = "teleport.session"
	// SessionEndEvent indicates taht a session has ended
	SessionEndEvent = "teleport.session.end"
	// TerminalResizedEvent fires when the user who initiated the session
	// resizes his terminal
	TerminalResizedEvent = "teleport.resized"
	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent = "teleport.exec"
	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent = "teleport.auth.attempt"
	// SCPEvent means data transfer that occured on the server
	SCPEvent = "teleport.scp"
	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent = "teleport.resize.pty"
)

func infof(format string, params ...interface{}) {
	logrus.Infof("[audit] -----> "+format, params...)
}

func warningf(format string, params ...interface{}) {
	logrus.Warningf("[audit] -----> "+format, params...)
}

func errorf(format string, params ...interface{}) {
	logrus.Errorf("[audit] -----> "+format, params...)
}

type AuditEvent struct {
	Kind      string
	SessionID session.ID
	Created   time.Time
	Data      map[string]interface{}
}

func (e *AuditEvent) String() string {
	bytes, _ := json.Marshal(e.Data)
	return fmt.Sprintf("%v,%s,%s", e.Created, e.Kind, string(bytes))
}

type ISessionLogger interface {
	SID() session.ID
	UserLogin() string
	LogEvent(*AuditEvent) error
}

// BaseSessionLogger implements the common features of a session logger. The imporant
// property of the base logger is that it never fails and can be used as a fallback
// implementation behind more sophisticated loggers
type BaseSessionLogger struct {
	sid       session.ID
	userLogin string
	userAddr  string
	created   time.Time
	parent    *AuditLog
	writer    io.Writer
}

func (sl *BaseSessionLogger) UserLogin() string {
	return sl.userLogin
}

func (sl *BaseSessionLogger) SID() session.ID {
	return sl.sid
}

func (sl *BaseSessionLogger) Close() error {
	return nil
}

func (sl *BaseSessionLogger) LogEvent(e *AuditEvent) error {
	if _, err := fmt.Fprintf(sl.writer, e.String()); err != nil {
		logrus.Error(err)
	}
	return nil
}

func (sl *BaseSessionLogger) initSyslog() {
	writer, err := syslog.New(SyslogPriority, SyslogPrefix)
	if err != nil {
		logrus.Error(err)
		return
	}
	sl.writer = writer
}

// SessionLogger is a file system based session logger. Every session is a file. It is
// based on the BaseSessionLogger
type SessionLogger struct {
	sync.Mutex
	once sync.Once
	BaseSessionLogger
	eventsFile *os.File
	streamFile *os.File
}

func (sl *SessionLogger) LogEvent(e *AuditEvent) (err error) {
	defer (func() {
		sl.Lock()
		sl.Unlock()
		_, err := fmt.Fprintln(sl.eventsFile, e.String())
		if err != nil {
			logrus.Error(err)
		}
	})()
	if e.Kind == SessionEndEvent {
		sl.parent.DeleteSessionLogger(sl.sid)
		// Finalize will close all resources owned by the session logger,
		// and "seal" the storage
		exitCode, _ := e.Data["exitcode"].(int)
		output, _ := e.Data["output"].(string)
		sl.finalize(exitCode, output)
	}
	infof("SessionLogger.LogEvent(sid=%v, event=%s, data=%s)", sl.sid, e.Kind, e.Data)
	return err
}

func (sl *SessionLogger) finalize(exitCode int, output string) {
	sl.once.Do(func() {
		sl.streamFile.Close()
		sl.eventsFile.Close()
		sl.streamFile = nil
		sl.eventsFile = nil
		infof("sessionLogger.Finalize(%v) exitCode=%d", sl.SID(), exitCode)
	})
}

func (sl *SessionLogger) WriteStream(bytes []byte) error {
	if sl.streamFile == nil {
		err := trace.Errorf("session %v error: attempt to write to a closed file", sl.SID())
		logrus.Error(err)
		return err
	}
	if _, err := sl.streamFile.Write(bytes); err != nil {
		logrus.Error(err)
		return trace.Wrap(err)
	}
	infof("%d bytes -> %s", len(bytes), sl.streamFile.Name())
	return nil
}

// AuditLog is a new combined facility to record Teleport events and
// sessions. It implements these interfaces:
//	- events.Log
//	- recorder.Recorder
type AuditLog struct {
	sync.Mutex
	loggers    map[session.ID]ISessionLogger
	dataDir    string
	safeWriter io.Writer
}

func NewAuditLog(dataDir string) (*AuditLog, error) {
	dataDir = filepath.Join(dataDir, "sessions")
	if err := os.MkdirAll(dataDir, 0770); err != nil {
		logrus.Error(err)
		return nil, trace.Wrap(err)
	}
	return &AuditLog{
		loggers:    make(map[session.ID]ISessionLogger, 0),
		dataDir:    dataDir,
		safeWriter: ioutil.Discard,
	}, nil
}

func (l *AuditLog) LogEvent(sid session.ID, e *AuditEvent) {
	sl := l.GetSessionLogger(sid)
	if sl != nil {
		logrus.Errorf("failed to log event %v for session %v: unknown session", e.Kind, sid)
		return
	}
	sl.LogEvent(e)
}

func (l *AuditLog) Close() error {
	return nil
}

func (l *AuditLog) NewSessionLogger(sess *session.Session) (sl ISessionLogger) {
	// have existing with this ID?
	if sl = l.GetSessionLogger(sess.ID); sl != nil {
		if sl.UserLogin() != sess.Login {
			warningf("overwriting session %s with a new session for %s", sess.ID, sess.Login)
		}
		return sl
	}
	var userAddr string
	if len(sess.Parties) > 0 {
		userAddr = sess.Parties[0].RemoteAddr
	}
	base := BaseSessionLogger{
		created:   sess.Created,
		sid:       sess.ID,
		userLogin: sess.Login,
		userAddr:  userAddr,
		parent:    l,
		writer:    l.safeWriter,
	}
	// failed creating an FS-based logger? report error and return the
	// basic logger instead:
	safeLogger := func(err error) ISessionLogger {
		logrus.Errorf("failed creating a session logger: %v", err)
		base.initSyslog()
		return &base
	}
	// wrapper around locking:
	pickLogger := func(sl ISessionLogger) ISessionLogger {
		l.Lock()
		defer l.Unlock()
		l.loggers[sess.ID] = sl
		return sl
	}
	// create a new session stream file:
	streamFname := filepath.Join(l.dataDir, fmt.Sprintf("%s.session.bytes", sess.ID))
	fstream, err := os.OpenFile(streamFname, os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		return pickLogger(safeLogger(err))
	}
	// create a new session file:
	eventsFname := filepath.Join(l.dataDir, fmt.Sprintf("%s.session", sess.ID))
	fevents, err := os.OpenFile(eventsFname, os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		return pickLogger(safeLogger(err))
	}
	return pickLogger(&SessionLogger{
		BaseSessionLogger: base,
		streamFile:        fstream,
		eventsFile:        fevents,
	})
}

func (l *AuditLog) DeleteSessionLogger(sid session.ID) {
	l.Lock()
	defer l.Unlock()
	delete(l.loggers, sid)
}

func (l *AuditLog) GetSessionLogger(id session.ID) ISessionLogger {
	l.Lock()
	defer l.Unlock()
	sl, ok := l.loggers[id]
	if !ok {
		warningf("request for unknown session id=%v", id)
		return nil
	}
	return sl
}
