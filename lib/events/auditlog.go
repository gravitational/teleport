package events

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

func infof(format string, params ...interface{}) {
	logrus.Infof("[audit] -----> "+format, params...)
}

func warningf(format string, params ...interface{}) {
	logrus.Warningf("[audit] -----> "+format, params...)
}

type ISessionLogger interface {
	recorder.ChunkWriteCloser
	recorder.ChunkReader

	SID() session.ID
	UserLogin() string
}

// BaseSessionLogger implements the common features of a session logger. The imporant
// property of the base logger is that it never fails and can be used as a fallback
// implementation behind more sophisticated loggers
type BaseSessionLogger struct {
	sid       session.ID
	userLogin string
	userAddr  string
	created   time.Time
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
	BaseSessionLogger
	file *os.File
}

func (sl *SessionLogger) OnExec(cmd []string) {
}

func (sl *SessionLogger) WriteStream(bytes []byte) error {
	if _, err := sl.file.Write(bytes); err != nil {
		logrus.Error(err)
		return trace.Wrap(err)
	}
	infof("%d bytes -> %s", len(bytes), sl.file.Name())
	return nil
}

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
	loggers map[session.ID]ISessionLogger
	dataDir string
}

func NewAuditLog(dataDir string) (*AuditLog, error) {
	// make sure the directory exists
	if err := os.MkdirAll(dataDir, 0770); err != nil {
		logrus.Error(err)
		return nil, trace.Wrap(err)
	}
	return &AuditLog{
		loggers: make(map[session.ID]ISessionLogger, 0),
		dataDir: dataDir,
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
	}
	// create a new session file:
	sessionFname := filepath.Join(l.dataDir, "sessions", fmt.Sprintf("%s.tele.session", sess.ID))
	f, err := os.OpenFile(sessionFname, os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		// failed creating an FS-based logger? report error and return the
		// basic logger instead:
		logrus.Error(err)
		sl = &base
	} else {
		sl = &SessionLogger{
			BaseSessionLogger: base,
			file:              f,
		}
	}
	// add the new logger:
	l.Lock()
	defer l.Unlock()
	l.loggers[sess.ID] = sl
	return sl
}

func (l *AuditLog) GetSessionLogger(id session.ID) ISessionLogger {
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
	infof("LogEntry(%s) -> %s: %v", entry.Schema, entry.String(), entry.Properties)
	return nil
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
