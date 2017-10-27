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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
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

	// UID of the root user (always 0)
	RootUID = 0

	// GID of the adm group (always 4)
	AdmGID = 4
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

type TimeSourceFunc func() time.Time

// AuditLog is a new combined facility to record Teleport events and
// sessions. It implements IAuditLog
type AuditLog struct {
	sync.Mutex
	loggers map[session.ID]SessionLogger
	dataDir string

	// file is the current global event log file. As the time goes
	// on, it will be replaced by a new file every day
	file *os.File

	// fileTime is a rounded (to a day, by default) timestamp of the
	// currently opened file
	fileTime time.Time

	// RotationPeriod defines how frequently to rotate the log file
	RotationPeriod time.Duration

	// same as time.Now(), but helps with testing
	TimeSource TimeSourceFunc

	// recordSessions controls if sessions are recorded along with audit events.
	recordSessions bool
}

// Creates and returns a new Audit Log object which will store its logfiles in
// a given directory. Session recording can be disabled by setting
// recordSessions to false.
func NewAuditLog(dataDir string, recordSessions bool) (IAuditLog, error) {
	// create the log directory first, with slightly broader permissions
	if err := os.MkdirAll(dataDir, 0770); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := os.Chown(dataDir, RootUID, AdmGID); err != nil {
		return nil, trace.Wrap(err)
	}
	// create a directory for session logs:
	sessionDir := filepath.Join(dataDir, SessionLogsDir)
	if err := os.MkdirAll(sessionDir, 0770); err != nil {
		return nil, trace.Wrap(err)
	}

	al := &AuditLog{
		loggers:        make(map[session.ID]SessionLogger, 0),
		dataDir:        dataDir,
		RotationPeriod: defaults.LogRotationPeriod,
		TimeSource:     time.Now,
		recordSessions: recordSessions,
	}
	if err := al.migrateSessions(); err != nil {
		return nil, trace.Wrap(err)
	}
	return al, nil
}

func (l *AuditLog) migrateSessions() error {
	// if 'default' namespace does not exist, migrate old logs to the new location
	sessionDir := filepath.Join(l.dataDir, SessionLogsDir)
	targetDir := filepath.Join(sessionDir, defaults.Namespace)
	_, err := utils.StatDir(targetDir)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	log.Infof("[MIGRATION] migrating sessions from %v to %v", sessionDir, filepath.Join(sessionDir, defaults.Namespace))
	// can't directly rename dir to its own subdir, so using temp dir
	tempDir := filepath.Join(l.dataDir, "___migrate")
	if err := os.Rename(sessionDir, tempDir); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := os.MkdirAll(sessionDir, 0770); err != nil {
		return trace.Wrap(err)
	}
	if err := os.Rename(tempDir, targetDir); err != nil {
		return trace.ConvertSystemError(err)
	}
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
		Time: l.TimeSource().In(time.UTC).UnixNano(),
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
	log.Debugf("audit.log: getSessionReader(%v, %v)", namespace, sid)
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
	log.Debugf("auditLog.EmitAuditEvent(%s)", eventType)

	// see if the log needs to be rotated
	if err := l.rotateLog(); err != nil {
		log.Error(err)
	}

	// set event type and time:
	fields[EventType] = eventType
	fields[EventTime] = l.TimeSource().In(time.UTC).Round(time.Second)

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
				log.Debugf("audit log: removing session logger for SID=%v", sessionID)
				l.Lock()
				delete(l.loggers, session.ID(sessionID))
				l.Unlock()
				if err := sl.Finalize(); err != nil {
					log.Error(err)
				}
			}
		} else {
			log.Warning(err.Error())
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
	log.Infof("auditLog.SearchEvents(%v, %v, query=%v)", fromUTC, toUTC, query)
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
	df, err := os.Open(l.dataDir)
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
		found, err := l.findInFile(filepath.Join(l.dataDir, filtered[i].Name()), queryVals)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, found...)
	}
	return events, nil
}

// SearchSessionEvents searches for session related events. Used to find completed sessions.
func (l *AuditLog) SearchSessionEvents(fromUTC, toUTC time.Time) ([]EventFields, error) {
	log.Infof("auditLog.SearchSessionEvents(%v, %v)", fromUTC, toUTC)

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
	log.Infof("auditLog.findInFile(%s, %v)", fn, query)
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
			log.Warnf("invalid JSON in %s line %d", fn, lineNo)
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

// rotateLog() checks if the current log file is older than a given duration,
// and if it is, closes it and opens a new one
func (l *AuditLog) rotateLog() (err error) {
	// determine the timestamp for the current log file
	fileTime := l.TimeSource().In(time.UTC).Round(l.RotationPeriod)

	openLogFile := func() error {
		l.Lock()
		defer l.Unlock()
		logfname := filepath.Join(l.dataDir,
			fileTime.Format("2006-01-02.15:04:05")+LogfileExt)
		l.file, err = os.OpenFile(logfname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Error(err)
		}
		err := os.Chown(logfname, RootUID, AdmGID)
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
	for sid, logger := range l.loggers {
		logger.Close()
		delete(l.loggers, sid)
	}
	return nil
}

// sessionStreamFn helper determins the name of the stream file for a given
// session by its ID
func (l *AuditLog) sessionStreamFn(namespace string, sid session.ID) string {
	return filepath.Join(
		l.dataDir,
		SessionLogsDir,
		namespace,
		fmt.Sprintf("%s%s", sid, SessionStreamPrefix))
}

// sessionLogFn helper determins the name of the stream file for a given
// session by its ID
func (l *AuditLog) sessionLogFn(namespace string, sid session.ID) string {
	return filepath.Join(
		l.dataDir,
		SessionLogsDir,
		namespace,
		fmt.Sprintf("%s%s", sid, SessionLogPrefix))
}

// LoggerFor creates a logger for a specified session. Session loggers allow
// to group all events into special "session log files" for easier audit
func (l *AuditLog) LoggerFor(namespace string, sid session.ID) (sl SessionLogger, err error) {
	l.Lock()
	defer l.Unlock()

	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}

	// if we are not recording sessions, create a logger that discards all
	// session data sent to it.
	if l.recordSessions == false {
		return &discardSessionLogger{}, nil
	}

	sl, ok := l.loggers[sid]
	if ok {
		return sl, nil
	}
	// make sure session logs dir is present
	sdir := filepath.Join(l.dataDir, SessionLogsDir, namespace)
	if err := os.MkdirAll(sdir, 0770); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// create a new session stream file:
	fstream, err := os.OpenFile(l.sessionStreamFn(namespace, sid), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// create a new session file:
	fevents, err := os.OpenFile(l.sessionLogFn(namespace, sid), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	sl = &diskSessionLogger{
		sid:         sid,
		streamFile:  fstream,
		eventsFile:  fevents,
		timeSource:  l.TimeSource,
		createdTime: l.TimeSource().In(time.UTC).Round(time.Second),
	}
	l.loggers[sid] = sl
	auditOpenFiles.Inc()
	return sl, nil
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
