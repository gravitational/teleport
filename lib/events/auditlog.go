package events

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

const (
	SyslogPriority = syslog.LOG_AUTH | syslog.LOG_NOTICE
	SyslogPrefix   = "teleport"
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
	// TODO ev: this is for compatibility. remove it later
	delete(e.Data, "sessionid")
	bytes, _ := json.Marshal(e.Data)
	return fmt.Sprintf("%v,%s,%s", e.Created, e.Kind, string(bytes))
}

type ISessionLogger interface {
	recorder.ChunkWriteCloser
	recorder.ChunkReader

	SID() session.ID
	UserLogin() string
	AddEvent(*AuditEvent) error
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

func (sl *BaseSessionLogger) AddEvent(e *AuditEvent) error {
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

func (sl *BaseSessionLogger) WriteChunks(chunks []recorder.Chunk) error {
	lc := len(chunks)
	tl := 0
	for i := range chunks {
		tl += len(chunks[i].Data)
	}
	infof("BaseLoggerWriteChunks(%v chunks, total len: %v)", lc, tl)
	return nil
}

func (sl *BaseSessionLogger) ReadChunks(start int, end int) ([]recorder.Chunk, error) {
	infof("ReadChunks(%v, %v)", start, end)
	return make([]recorder.Chunk, 0), nil
}

func (sl *BaseSessionLogger) GetChunksCount() (uint64, error) {
	infof("GetChunkCount()")
	return 0, nil
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

func (sl *SessionLogger) AddEvent(e *AuditEvent) (err error) {
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
	infof("SessionLogger.AddEvent(sid=%v, event=%s, data=%s)", sl.sid, e.Kind, e.Data)
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

// TODO (ev): kill it
func (sl *SessionLogger) WriteChunks(chunks []recorder.Chunk) error {
	lc := len(chunks)
	tl := 0
	for i := range chunks {
		tl += len(chunks[i].Data)
		sl.WriteStream(chunks[i].Data)
	}
	infof("FSLogger.WriteChunks(%v chunks, total len: %v)", lc, tl)
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

// TODO (ev) kill it
func (l *AuditLog) Log(id lunk.EventID, e lunk.Event) {
	l.LogEntry(lunk.NewEntry(id, e))
}

// TODO (ev) kill it
func (l *AuditLog) LogEntry(entry lunk.Entry) error {
	sid := session.ID(entry.Properties["sessionid"])
	infof("GOT SID: %v", sid)
	if sid == "" {
		errorf("received log entry without event ID: %v", entry.Properties)
		return nil
	}
	e := &AuditEvent{
		SessionID: sid,
		Created:   time.Now().In(time.UTC).Round(time.Second),
		Kind:      entry.Schema,
		Data:      make(map[string]interface{}),
	}
	if e.Kind == SessionEndEvent {
		exitCode, _ := strconv.Atoi(entry.Properties["exitcode"])
		e.Data["exitcode"] = exitCode
		e.Data["output"] = entry.Properties["output"]
	} else {
		for k, v := range entry.Properties {
			e.Data[k] = v
		}
	}
	return l.GetSessionLogger(sid).AddEvent(e)
}

// TODO (ev) kill it
func (l *AuditLog) LogSession(s session.Session) error {
	l.NewSessionLogger(&s)
	infof("LogSession() -> %v", s.ID)
	return nil
}

// TODO (ev) kill it
func (l *AuditLog) GetEvents(filter Filter) ([]lunk.Entry, error) {
	infof("GetEvents(session=%s)", filter.SessionID)
	return nil, nil
}

// TODO (ev) kill it
func (l *AuditLog) GetSessionEvents(filter Filter) ([]session.Session, error) {
	infof("GetSessionEvents(session=%s)", filter.SessionID)
	return nil, nil
}

func (l *AuditLog) Close() error {
	infof("Close()")
	return nil
}

// TODO (ev) kill it
func (l *AuditLog) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	infof("GetChunkWriter(%s)", id)
	return l.GetSessionLogger(session.ID(id)), nil
}

// TODO (ev) kill it
func (l *AuditLog) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	infof("GetChunkWriter(%s)", id)
	return l.GetSessionLogger(session.ID(id)), nil
}
