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

package events

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	// SessionLogsDir is a subdirectory inside the eventlog data dir
	// where all session-specific logs and streams are stored, like
	// in /var/lib/teleport/logs/sessions
	SessionLogsDir = "sessions"

	// LogfileExt defines the ending of the daily event log file
	LogfileExt = ".log"

	// SessionLogPrefix defines the endof of session log files
	SessionLogPrefix = ".session.log"

	// SessionStreamPrefix defines the ending of session stream files,
	// that's where interactive PTY I/O is saved.
	SessionStreamPrefix = ".session.bytes"
)

var (
	auditOpenFiles = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "audit_server_open_files",
			Help: "Number of open audit files",
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(auditOpenFiles)
}

// AuditLog is a new combined facility to record Teleport events and
// sessions. It implements IAuditLog
type AuditLog struct {
	sync.Mutex
	*log.Entry
	AuditLogConfig

	loggers *ttlmap.TTLMap

	// file is the current global event log file. As the time goes
	// on, it will be replaced by a new file every day
	file *os.File

	// fileTime is a rounded (to a day, by default) timestamp of the
	// currently opened file
	fileTime time.Time
}

// AuditLogConfig specifies configuration for AuditLog server
type AuditLogConfig struct {
	// DataDir is the directory where audit log stores the data
	DataDir string

	// RecordSessions controls if sessions are recorded along with audit events.
	RecordSessions bool

	// RotationPeriod defines how frequently to rotate the log file
	RotationPeriod time.Duration

	// SessionIdlePeriod defines the period after which sessions will be considered
	// idle (and audit log will free up some resources)
	SessionIdlePeriod time.Duration

	// Clock is a clock either real one or used in tests
	Clock clockwork.Clock

	// GID if provided will be used to set group ownership of the directory
	// to GID
	GID *int

	// UID if provided will be used to set userownership of the directory
	// to UID
	UID *int

	// DirMask if provided will be used to set directory mask access
	// otherwise set to default value
	DirMask *os.FileMode
}

// CheckAndSetDefaults checks and sets defaults
func (a *AuditLogConfig) CheckAndSetDefaults() error {
	if a.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}
	if a.RotationPeriod == 0 {
		a.RotationPeriod = defaults.LogRotationPeriod
	}
	if a.SessionIdlePeriod == 0 {
		a.SessionIdlePeriod = defaults.SessionIdlePeriod
	}
	if a.DirMask == nil {
		mask := os.FileMode(teleport.DirMaskSharedGroup)
		a.DirMask = &mask
	}
	if (a.GID != nil && a.UID == nil) || (a.UID != nil && a.GID == nil) {
		return trace.BadParameter("if UID or GID is set, both should be specified")
	}
	return nil
}

// Creates and returns a new Audit Log object whish will store its logfiles in
// a given directory. Session recording can be disabled by setting
// recordSessions to false.
func NewAuditLog(cfg AuditLogConfig) (*AuditLog, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// create a directory for session logs:
	sessionDir := filepath.Join(cfg.DataDir, SessionLogsDir)
	if err := os.MkdirAll(sessionDir, *cfg.DirMask); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if cfg.UID != nil && cfg.GID != nil {
		err := os.Chown(cfg.DataDir, *cfg.UID, *cfg.GID)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		err = os.Chown(sessionDir, *cfg.UID, *cfg.GID)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	al := &AuditLog{
		AuditLogConfig: cfg,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
		}),
	}
	loggers, err := ttlmap.New(defaults.AuditLogSessions,
		ttlmap.CallOnExpire(al.asyncCloseSessionLogger), ttlmap.Clock(cfg.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	al.loggers = loggers
	go al.periodicCloseInactiveLoggers()
	return al, nil
}

func (l *AuditLog) WaitForDelivery(context.Context) error {
	return nil
}

// PostSessionSlice submits slice of session chunks to the audit log server.
func (l *AuditLog) PostSessionSlice(slice SessionSlice) error {
	if slice.Namespace == "" {
		return trace.BadParameter("missing parameter Namespace")
	}
	if len(slice.Chunks) == 0 {
		return trace.BadParameter("missing session chunks")
	}
	sl, err := l.LoggerFor(slice.Namespace, session.ID(slice.SessionID))
	if err != nil {
		l.Errorf("failed to get logger: %v", trace.DebugReport(err))
		return trace.BadParameter("audit.log: no session writer for %s", slice.SessionID)
	}
	for i := range slice.Chunks {
		_, err := sl.WriteChunk(slice.Chunks[i])
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// PostSessionChunk writes a new chunk of session stream into the audit log
func (l *AuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	tmp, err := utils.ReadAll(reader, 16*1024)
	if err != nil {
		return trace.Wrap(err)
	}
	chunk := &SessionChunk{
		Time: l.Clock.Now().In(time.UTC).UnixNano(),
		Data: tmp,
	}
	return l.PostSessionSlice(SessionSlice{
		Namespace: namespace,
		SessionID: string(sid),
		Chunks:    []*SessionChunk{chunk},
	})
}

// GetSessionChunk returns a reader which console and web clients request
// to receive a live stream of a given session. The reader allows access to a
// session stream range from offsetBytes to offsetBytes+maxBytes
//
func (l *AuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	l.Debugf("getSessionReader(%v, %v)", namespace, sid)
	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}
	fstream, err := os.OpenFile(l.sessionStreamFn(namespace, sid), os.O_RDONLY, 0640)
	if err != nil {
		log.Warning(err)
		return nil, trace.Wrap(err)
	}
	defer fstream.Close()

	// seek to 'offset' from the beginning
	fstream.Seek(int64(offsetBytes), 0)

	// copy up to maxBytes from the offset position:
	var buff bytes.Buffer
	io.Copy(&buff, io.LimitReader(fstream, int64(maxBytes)))

	return buff.Bytes(), nil
}

// Returns all events that happen during a session sorted by time
// (oldest first).
//
// Can be filtered by 'after' (cursor value to return events newer than)
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (l *AuditLog) GetSessionEvents(namespace string, sid session.ID, afterN int) ([]EventFields, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}
	logFile, err := os.OpenFile(l.sessionLogFn(namespace, sid), os.O_RDONLY, 0640)
	if err != nil {
		// no file found? this means no events have been logged yet
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	defer logFile.Close()

	retval := make([]EventFields, 0, 256)

	// read line by line:
	scanner := bufio.NewScanner(logFile)
	for lineNo := 0; scanner.Scan(); lineNo++ {
		if lineNo < afterN {
			continue
		}
		var fields EventFields
		if err = json.Unmarshal(scanner.Bytes(), &fields); err != nil {
			log.Error(err)
			return nil, trace.Wrap(err)
		}
		fields[EventCursor] = lineNo
		retval = append(retval, fields)
	}
	return retval, nil
}

// EmitAuditEvent adds a new event to the log. Part of auth.IAuditLog interface.
func (l *AuditLog) EmitAuditEvent(eventType string, fields EventFields) error {
	l.Debugf("EmitAuditEvent(%v: %v)", eventType, fields)

	// see if the log needs to be rotated
	if err := l.rotateLog(); err != nil {
		log.Error(err)
	}

	// set event type and time:
	fields[EventType] = eventType
	fields[EventTime] = l.Clock.Now().In(time.UTC).Round(time.Second)

	// line is the text to be logged
	line := eventToLine(fields)

	// if this event is associated with a session -> forward it to the session log as well
	sessionID := fields.GetString(SessionEventID)
	if sessionID != "" {
		sl, err := l.LoggerFor(fields.GetString(EventNamespace), session.ID(sessionID))
		if err == nil {
			sl.LogEvent(fields)

			// Session ended? Get rid of the session logger then:
			if eventType == SessionEndEvent {
				l.Debugf("removing session logger for SID=%v", sessionID)
				l.Lock()
				l.loggers.Remove(sessionID)
				l.Unlock()
				if err := sl.Finalize(); err != nil {
					log.Error(err)
				}
			}
		} else {
			l.Errorf("failed to get logger: %v", trace.DebugReport(err))
		}
	}
	// log it to the main log file:
	if l.file != nil {
		fmt.Fprintln(l.file, line)
	}
	return nil
}

// SearchEvents finds events. Results show up sorted by date (newest first)
func (l *AuditLog) SearchEvents(fromUTC, toUTC time.Time, query string) ([]EventFields, error) {
	l.Debugf("SearchEvents(%v, %v, query=%v)", fromUTC, toUTC, query)
	queryVals, err := url.ParseQuery(query)
	if err != nil {
		return nil, trace.BadParameter("missing parameter query", query)
	}
	// how many days of logs to search?
	days := int(toUTC.Sub(fromUTC).Hours() / 24)
	if days < 0 {
		return nil, trace.BadParameter("query", query)
	}

	// scan the log directory:
	df, err := os.Open(l.DataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer df.Close()
	entries, err := df.Readdir(-1)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered := make([]os.FileInfo, 0, days)
	for i := range entries {
		fi := entries[i]
		if fi.IsDir() || filepath.Ext(fi.Name()) != LogfileExt {
			continue
		}
		fd := fi.ModTime().UTC()
		if fd.After(fromUTC) && fd.Before(toUTC) {
			filtered = append(filtered, fi)
		}
	}
	// sort all accepted  files by date
	sort.Sort(byDate(filtered))

	// search within each file:
	events := make([]EventFields, 0)
	for i := range filtered {
		found, err := l.findInFile(filepath.Join(l.DataDir, filtered[i].Name()), queryVals)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, found...)
	}
	return events, nil
}

// SearchSessionEvents searches for session related events. Used to find completed sessions.
func (l *AuditLog) SearchSessionEvents(fromUTC, toUTC time.Time) ([]EventFields, error) {
	l.Infof("SearchSessionEvents(%v, %v)", fromUTC, toUTC)

	// only search for specific event types
	query := url.Values{}
	query[EventType] = []string{
		SessionStartEvent,
		SessionEndEvent,
	}

	return l.SearchEvents(fromUTC, toUTC, query.Encode())
}

// byDate implements sort.Interface.
type byDate []os.FileInfo

func (f byDate) Len() int           { return len(f) }
func (f byDate) Less(i, j int) bool { return f[i].ModTime().Before(f[j].ModTime()) }
func (f byDate) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// findInFile scans a given log file and returns events that fit the criteria
// This simplistic implementation ONLY SEARCHES FOR EVENT TYPE(s)
//
// You can pass multiple types like "event=session.start&event=session.end"
func (l *AuditLog) findInFile(fn string, query url.Values) ([]EventFields, error) {
	l.Infof("findInFile(%s, %v)", fn, query)
	retval := make([]EventFields, 0)

	eventFilter := query[EventType]
	doFilter := len(eventFilter) > 0

	// open the log file:
	lf, err := os.OpenFile(fn, os.O_RDONLY, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer lf.Close()

	// for each line...
	scanner := bufio.NewScanner(lf)
	for lineNo := 0; scanner.Scan(); lineNo++ {
		accepted := false
		// optimization: to avoid parsing JSON unnecessarily, lets see if we
		// can filter out lines that don't even have the requested event type on the line
		for i := range eventFilter {
			if strings.Contains(scanner.Text(), eventFilter[i]) {
				accepted = true
				break
			}
		}
		if doFilter && !accepted {
			continue
		}
		// parse JSON on the line and compare event type field to what's
		// in the query:
		var ef EventFields
		if err = json.Unmarshal(scanner.Bytes(), &ef); err != nil {
			l.Warnf("invalid JSON in %s line %d", fn, lineNo)
		}
		for i := range eventFilter {
			if ef.GetString(EventType) == eventFilter[i] {
				accepted = true
				break
			}
		}
		if accepted || !doFilter {
			retval = append(retval, ef)
		}
	}
	return retval, nil
}

// rotateLog checks if the current log file is older than a given duration,
// and if it is, closes it and opens a new one.
func (l *AuditLog) rotateLog() (err error) {
	l.Lock()
	defer l.Unlock()

	// determine the timestamp for the current log file
	fileTime := l.Clock.Now().In(time.UTC).Round(l.RotationPeriod)

	openLogFile := func() error {
		logfname := filepath.Join(l.DataDir,
			fileTime.Format(defaults.AuditLogTimeFormat)+LogfileExt)
		l.file, err = os.OpenFile(logfname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Error(err)
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

	// close any open sessions that haven't expired yet and are open
	for {
		key, value, found := l.loggers.Pop()
		if !found {
			break
		}
		l.closeSessionLogger(key, value)
	}
	return nil
}

// sessionStreamFn helper determines the name of the stream file for a given
// session by its ID
func (l *AuditLog) sessionStreamFn(namespace string, sid session.ID) string {
	return filepath.Join(
		l.DataDir,
		SessionLogsDir,
		namespace,
		fmt.Sprintf("%s%s", sid, SessionStreamPrefix))
}

// sessionLogFn helper determines the name of the stream file for a given
// session by its ID
func (l *AuditLog) sessionLogFn(namespace string, sid session.ID) string {
	return filepath.Join(
		l.DataDir,
		SessionLogsDir,
		namespace,
		fmt.Sprintf("%s%s", sid, SessionLogPrefix))
}

// LoggerFor creates a logger for a specified session. Session loggers allow
// to group all events into special "session log files" for easier audit
func (l *AuditLog) LoggerFor(namespace string, sid session.ID) (SessionLogger, error) {
	l.Lock()
	defer l.Unlock()

	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}

	logger, ok := l.loggers.Get(string(sid))
	if ok {
		sessionLogger, converted := logger.(SessionLogger)
		if !converted {
			return nil, trace.BadParameter("unsupported type: %T", logger)
		}
		// refresh the last active time of the logger
		l.loggers.Set(string(sid), logger, l.SessionIdlePeriod)
		return sessionLogger, nil
	}
	// make sure session logs dir is present
	sdir := filepath.Join(l.DataDir, SessionLogsDir, namespace)
	if err := os.Mkdir(sdir, *l.DirMask); err != nil {
		if !os.IsExist(err) {
			return nil, trace.Wrap(err)
		}
	} else if l.UID != nil && l.GID != nil {
		err := os.Chown(sdir, *l.UID, *l.GID)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	sessionLogger, err := NewDiskSessionLogger(DiskSessionLoggerConfig{
		SessionID:      sid,
		EventsFileName: l.sessionLogFn(namespace, sid),
		StreamFileName: l.sessionStreamFn(namespace, sid),
		Clock:          l.Clock,
		RecordSessions: l.RecordSessions,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	l.loggers.Set(string(sid), sessionLogger, l.SessionIdlePeriod)
	auditOpenFiles.Inc()
	return sessionLogger, nil
}

func (l *AuditLog) asyncCloseSessionLogger(key string, val interface{}) {
	go l.closeSessionLogger(key, val)
}

func (l *AuditLog) closeSessionLogger(key string, val interface{}) {
	l.Debugf("closing session logger %v", key)
	logger, ok := val.(SessionLogger)
	if !ok {
		l.Warningf("warning, not valid value type %T for %v", val, key)
		return
	}
	if err := logger.Finalize(); err != nil {
		log.Warningf("failed to finalize: %v", trace.DebugReport(err))
	}
}

func (l *AuditLog) periodicCloseInactiveLoggers() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.closeInactiveLoggers()
		}
	}
}

func (l *AuditLog) closeInactiveLoggers() {
	l.Lock()
	defer l.Unlock()

	expired := l.loggers.RemoveExpired(10)
	if expired != 0 {
		l.Debugf("closed %v inactive session loggers", expired)
	}
}

// eventToLine helper creates a loggable line/string for a given event
func eventToLine(fields EventFields) string {
	jbytes, err := json.Marshal(fields)
	jsonString := string(jbytes)
	if err != nil {
		log.Error(err)
		jsonString = ""
	}
	return jsonString
}
