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
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

	// playbackDir is a directory used for unpacked session recordings
	playbackDir string

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

	// ServerID is the id of the audit log server
	ServerID string

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

	// PlaybackRecycleTTL is a time after uncompressed playback files will be
	// deleted
	PlaybackRecycleTTL time.Duration
}

// CheckAndSetDefaults checks and sets defaults
func (a *AuditLogConfig) CheckAndSetDefaults() error {
	if a.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if a.ServerID == "" {
		return trace.BadParameter("missing parameter ServerID")
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
	if a.PlaybackRecycleTTL == 0 {
		a.PlaybackRecycleTTL = defaults.PlaybackRecycleTTL
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
	al := &AuditLog{
		playbackDir:    filepath.Join(cfg.DataDir, "playbacks"),
		AuditLogConfig: cfg,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
		}),
	}
	loggers, err := ttlmap.New(defaults.AuditLogSessions,
		ttlmap.CallOnExpire(al.closeSessionLogger), ttlmap.Clock(cfg.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	al.loggers = loggers
	// create a directory for audit logs, audit log does not create
	// session logs before migrations are run in case if the directory
	// has to be moved
	auditDir := filepath.Join(cfg.DataDir, cfg.ServerID)
	if err := os.MkdirAll(auditDir, *cfg.DirMask); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := al.runMigrations(); err != nil {
		return nil, trace.Wrap(err)
	}
	// create a directory for session logs:
	sessionDir := filepath.Join(cfg.DataDir, cfg.ServerID, SessionLogsDir)
	if err := os.MkdirAll(sessionDir, *cfg.DirMask); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	// create a directory for uncompressed playbacks
	if err := os.MkdirAll(al.playbackDir, *cfg.DirMask); err != nil {
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
		err = os.Chown(al.playbackDir, *cfg.UID, *cfg.GID)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	go al.periodicCloseInactiveLoggers()
	go al.periodicCleanupPlaybacks()
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
	sl, err := l.LoggerFor(slice.Namespace, session.ID(slice.SessionID), slice.Version == V1)
	if err != nil {
		l.Errorf("failed to get logger: %v", trace.DebugReport(err))
		return trace.BadParameter("audit.log: no session writer for %s", slice.SessionID)
	}
	var errors []error
	errors = append(errors, sl.PostSessionSlice(slice))
	errors = append(errors, l.processSlice(sl, &slice))
	return trace.NewAggregate(errors...)
}

func (l *AuditLog) processSlice(sl SessionLogger, slice *SessionSlice) error {
	for _, chunk := range slice.Chunks {
		if chunk.EventType == SessionPrintEvent || chunk.EventType == "" {
			continue
		}
		if chunk.EventType == SessionEndEvent {
			defer func() {
				l.removeLogger(slice.SessionID)
				if err := sl.Finalize(); err != nil {
					log.Warningf("Failed to finalize logger: %v", trace.DebugReport(err))
				}
			}()
		}
		fields, err := eventFromChunk(slice.SessionID, chunk)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := l.emitAuditEvent(chunk.EventType, fields); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DELETE IN: 2.6.0
// This method is no longer used in 2.5.0
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

func (l *AuditLog) getAuthServers() ([]string, error) {
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
	var authServers []string
	for i := range entries {
		fi := entries[i]
		if fi.IsDir() {
			authServers = append(authServers, filepath.Base(fi.Name()))
		}
	}
	return authServers, nil
}

type sessionIndex struct {
	dataDir   string
	namespace string
	sid       session.ID
	events    []indexEntry
	chunks    []indexEntry
}

func (idx *sessionIndex) sort() {
	sort.Slice(idx.events, func(i, j int) bool {
		return idx.events[i].Index < idx.events[j].Index
	})
	sort.Slice(idx.chunks, func(i, j int) bool {
		return idx.chunks[i].Offset < idx.chunks[j].Offset
	})
}

func (idx *sessionIndex) eventsFileName(index int) string {
	entry := idx.events[index]
	return filepath.Join(idx.dataDir, entry.authServer, SessionLogsDir, idx.namespace, entry.FileName)
}

func (idx *sessionIndex) eventsFile(afterN int) (int, error) {
	for i := len(idx.events) - 1; i >= 0; i-- {
		entry := idx.events[i]
		if int64(afterN) >= entry.Index {
			return i, nil
		}
	}
	return -1, trace.NotFound("%v not found", afterN)
}

func (idx *sessionIndex) chunksFile(offset int64) (string, int64, error) {
	for i := len(idx.chunks) - 1; i >= 0; i-- {
		entry := idx.chunks[i]
		if offset >= entry.Offset {
			return filepath.Join(idx.dataDir, entry.authServer, SessionLogsDir, idx.namespace, entry.FileName), entry.Offset, nil
		}
	}
	return "", 0, trace.NotFound("%v not found", offset)
}

func (l *AuditLog) readSessionIndex(namespace string, sid session.ID) (*sessionIndex, error) {
	authServers, err := l.getAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	index := sessionIndex{
		sid:       sid,
		dataDir:   l.DataDir,
		namespace: namespace,
	}
	for _, authServer := range authServers {
		indexFileName := filepath.Join(l.DataDir, authServer, SessionLogsDir, namespace, fmt.Sprintf("%v.index", sid))
		indexFile, err := os.OpenFile(indexFileName, os.O_RDONLY, 0640)
		err = trace.ConvertSystemError(err)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		events, chunks, err := readIndexEntries(indexFile, authServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		index.events = append(index.events, events...)
		index.chunks = append(index.chunks, chunks...)
		err = indexFile.Close()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	index.sort()
	return &index, nil
}

func readIndexEntries(file *os.File, authServer string) (events []indexEntry, chunks []indexEntry, err error) {
	scanner := bufio.NewScanner(file)
	for lineNo := 0; scanner.Scan(); lineNo++ {
		var entry indexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		entry.authServer = authServer
		switch entry.Type {
		case fileTypeEvents:
			events = append(events, entry)
		case fileTypeChunks:
			chunks = append(chunks, entry)
		default:
			return nil, nil, trace.BadParameter("unsupported type: %q", entry.Type)
		}
	}
	return
}

// GetSessionChunk returns a reader which console and web clients request
// to receive a live stream of a given session. The reader allows access to a
// session stream range from offsetBytes to offsetBytes+maxBytes
func (l *AuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	var data []byte
	for {
		out, err := l.getSessionChunk(namespace, sid, offsetBytes, maxBytes)
		if err != nil {
			if err == io.EOF {
				return data, nil
			}
			return nil, trace.Wrap(err)
		}
		data = append(data, out...)
		if len(data) == maxBytes || len(out) == 0 {
			return data, nil
		}
		maxBytes = maxBytes - len(out)
		offsetBytes = offsetBytes + len(out)
	}
}

func (l *AuditLog) cleanupOldPlaybacks() error {
	// scan the log directory and clean files last
	// accessed after an hour
	df, err := os.Open(l.playbackDir)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer df.Close()
	entries, err := df.Readdir(-1)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	for i := range entries {
		fi := entries[i]
		if fi.IsDir() {
			continue
		}
		fd := fi.ModTime().UTC()
		diff := l.Clock.Now().UTC().Sub(fd)
		if diff <= l.PlaybackRecycleTTL {
			continue
		}
		fileToRemove := filepath.Join(l.playbackDir, fi.Name())
		err := os.Remove(fileToRemove)
		if err != nil {
			l.Warningf("Failed to remove file %v: %v.", fileToRemove, err)
		}
		l.Debugf("Removed unpacked session playback file %v after %v.", fileToRemove, diff)
	}
	return nil
}

type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

func (l *AuditLog) unpackFile(fileName string) (readSeekCloser, error) {
	unpackedFile := filepath.Join(l.playbackDir, filepath.Base(fileName))

	// If client has called GetSessionChunk before session is over
	// this could lead to cases when not all data will be returned,
	// because unpackFile will be called concurrently with the unfinished write
	unpackedInfo, err := os.Stat(unpackedFile)
	err = trace.ConvertSystemError(err)
	switch {
	case err != nil && !trace.IsNotFound(err):
		return nil, trace.Wrap(err)
	case err == nil:
		packedInfo, err := os.Stat(fileName)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		// no new data has been added
		if unpackedInfo.ModTime().After(packedInfo.ModTime()) {
			return os.OpenFile(unpackedFile, os.O_RDONLY, 0640)
		}
	}

	start := l.Clock.Now()
	dest, err := os.OpenFile(unpackedFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	source, err := os.OpenFile(fileName, os.O_RDONLY, 0640)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer source.Close()
	reader, err := gzip.NewReader(source)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	if _, err := io.Copy(dest, reader); err != nil {
		// Unexpected EOF is returned by gzip reader
		// when the file has not been closed yet,
		// ignore this error
		if err != io.ErrUnexpectedEOF {
			dest.Close()
			return nil, trace.Wrap(err)
		}
	}
	if _, err := dest.Seek(0, 0); err != nil {
		dest.Close()
		return nil, trace.Wrap(err)
	}
	l.Debugf("Uncompressed %v into %v in %v", fileName, unpackedFile, l.Clock.Now().Sub(start))
	return dest, nil
}

func (l *AuditLog) getSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}
	idx, err := l.readSessionIndex(namespace, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fileName, fileOffset, err := idx.chunksFile(int64(offsetBytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := l.unpackFile(fileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	// seek to 'offset' from the beginning
	reader.Seek(int64(offsetBytes)-fileOffset, 0)

	// copy up to maxBytes from the offset position:
	var buff bytes.Buffer
	_, err = io.Copy(&buff, io.LimitReader(reader, int64(maxBytes)))
	return buff.Bytes(), err
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
	idx, err := l.readSessionIndex(namespace, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fileIndex, err := idx.eventsFile(afterN)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	events := make([]EventFields, 0, 256)
	for i := fileIndex; i < len(idx.events); i++ {
		skip := 0
		if i == fileIndex {
			skip = afterN
		}
		out, err := l.fetchSessionEvents(idx.eventsFileName(i), skip)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, out...)
	}
	return events, nil
}

func (l *AuditLog) fetchSessionEvents(fileName string, afterN int) ([]EventFields, error) {
	logFile, err := os.OpenFile(fileName, os.O_RDONLY, 0640)
	if err != nil {
		// no file found? this means no events have been logged yet
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	defer logFile.Close()
	reader, err := gzip.NewReader(logFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	retval := make([]EventFields, 0, 256)
	// read line by line:
	scanner := bufio.NewScanner(reader)
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

	if err := l.emitAuditEvent(eventType, fields); err != nil {
		return trace.Wrap(err)
	}

	// DELETE IN: 2.6.0
	// if this event is associated with a session -> forward it to the session log as well
	// this means that this is a legacy client, so audit logger is going to use compatibility mode logger
	sessionID := fields.GetString(SessionEventID)

	if sessionID != "" {
		sl, err := l.LoggerFor(fields.GetString(EventNamespace), session.ID(sessionID), true)
		if err == nil {
			if err := sl.LogEvent(fields); err != nil {
				l.Warningf("Failed to log event: %v.", err)
			}
			// Session ended? Get rid of the session logger then:
			if eventType == SessionEndEvent {
				l.removeLogger(sessionID)
				if err := sl.Finalize(); err != nil {
					log.Error(err)
				}
			}
		} else {
			l.Errorf("failed to get logger: %v", trace.DebugReport(err))
		}
	}
	return nil
}

func (l *AuditLog) removeLogger(sessionID string) {
	l.Debugf("Removing session logger for SID=%v.", sessionID)
	l.Lock()
	defer l.Unlock()
	l.loggers.Remove(sessionID)
}

// emitAuditEvent adds a new event to the log. Part of auth.IAuditLog interface.
func (l *AuditLog) emitAuditEvent(eventType string, fields EventFields) error {
	// see if the log needs to be rotated
	if err := l.rotateLog(); err != nil {
		log.Error(err)
	}

	// set event type and time:
	fields[EventType] = eventType
	if _, ok := fields[EventTime]; !ok {
		fields[EventTime] = l.Clock.Now().In(time.UTC).Round(time.Second)
	}
	// line is the text to be logged
	line, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err)
	}
	// log it to the main log file:
	if l.file != nil {
		fmt.Fprintln(l.file, string(line))
	}
	return nil
}

// matchingFiles returns files matching the time restrictions of the query
// across multiple auth servers, returns a list of file names
func (l *AuditLog) matchingFiles(fromUTC, toUTC time.Time) ([]eventFile, error) {
	authServers, err := l.getAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var filtered []eventFile
	for _, serverID := range authServers {
		// scan the log directory:
		df, err := os.Open(filepath.Join(l.DataDir, serverID))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer df.Close()
		entries, err := df.Readdir(-1)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for i := range entries {
			fi := entries[i]
			if fi.IsDir() || filepath.Ext(fi.Name()) != LogfileExt {
				continue
			}
			fd := fi.ModTime().UTC()
			if fd.After(fromUTC) && fd.Before(toUTC) {
				eventFile := eventFile{
					FileInfo: fi,
					path:     filepath.Join(l.DataDir, serverID, fi.Name()),
				}
				filtered = append(filtered, eventFile)
			}
		}
	}
	// sort all accepted files by date
	sort.Sort(byDate(filtered))
	return filtered, nil
}

func (l *AuditLog) moveAuditLogFile(fileName string) error {
	sourceFile := filepath.Join(l.DataDir, fileName)
	targetFile := filepath.Join(l.DataDir, l.ServerID, fileName)
	l.Infof("Migrating log file from %v to %v", sourceFile, targetFile)
	if err := os.Rename(sourceFile, targetFile); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func (l *AuditLog) migrateSessionsDir() error {
	sessionDir := filepath.Join(l.DataDir, SessionLogsDir)
	targetDir := filepath.Join(l.DataDir, l.ServerID, SessionLogsDir)
	_, err := utils.StatDir(targetDir)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// transform the recorded files to the new index format
	recordingsDir := filepath.Join(l.DataDir, SessionLogsDir, defaults.Namespace)
	fileInfos, err := listDir(recordingsDir)
	if err != nil {
		// source directory does not exist, means nothing to migrate
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil
	}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			l.Debugf("Migrating, skipping directory %v", fi.Name())
			continue
		}

		// only trigger migrations on .log
		// to avoid double migrations attempts or migrating recordings
		// that were already migrated
		if !strings.HasSuffix(fi.Name(), "session.log") {
			continue
		}

		parts := strings.Split(fi.Name(), ".")
		if len(parts) < 2 {
			l.Debugf("Migrating, skipping unknown file: %v", fi.Name())
		}
		sessionID := parts[0]
		sourceEventsFile := filepath.Join(recordingsDir, fmt.Sprintf("%v.session.log", sessionID))
		targetEventsFile := filepath.Join(recordingsDir, fmt.Sprintf("%v-0.events.gz", sessionID))
		l.Debugf("Migrating, session ID %v. Compressed %v to %v", sessionID, sourceEventsFile, targetEventsFile)
		err := moveAndGzipFile(sourceEventsFile, targetEventsFile)
		if err != nil {
			return trace.Wrap(err)
		}
		sourceChunksFile := filepath.Join(recordingsDir, fmt.Sprintf("%v.session.bytes", sessionID))
		targetChunksFile := filepath.Join(recordingsDir, fmt.Sprintf("%v-0.chunks.gz", sessionID))
		l.Debugf("Migrating session ID %v. Compressed %v to %v", sessionID, sourceChunksFile, targetChunksFile)
		err = moveAndGzipFile(sourceChunksFile, targetChunksFile)
		if err != nil {
			return trace.Wrap(err)
		}
		indexFileName := filepath.Join(recordingsDir, fmt.Sprintf("%v.index", sessionID))

		eventsData, err := json.Marshal(indexEntry{
			FileName: filepath.Base(targetEventsFile),
			Type:     fileTypeEvents,
			Index:    0,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		chunksData, err := json.Marshal(indexEntry{
			FileName: filepath.Base(targetChunksFile),
			Type:     fileTypeChunks,
			Offset:   0,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		err = ioutil.WriteFile(indexFileName,
			[]byte(
				fmt.Sprintf("%v\n%v\n",
					string(eventsData),
					string(chunksData),
				)),
			0640,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		l.Debugf("Migrating session ID %v. Wrote index file %v.", sessionID, indexFileName)
	}
	l.Infof("Moving sessions folder from %v to %v", sessionDir, targetDir)
	if err := os.Rename(sessionDir, targetDir); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func moveAndGzipFile(source string, target string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer sourceFile.Close()
	destFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	destWriter := newGzipWriter(destFile)
	defer destWriter.Close()
	_, err = io.Copy(destWriter, sourceFile)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.Remove(source); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func listDir(dir string) ([]os.FileInfo, error) {
	df, err := os.Open(dir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer df.Close()
	entries, err := df.Readdir(-1)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return entries, nil
}

// DELETE IN: 2.6.0
// runMigrations runs migrations for audit logs format
func (l *AuditLog) runMigrations() error {
	// migrate sessions directory
	if err := l.migrateSessionsDir(); err != nil {
		return trace.Wrap(err)
	}
	// move audit log files to the auth server id
	fileInfos, err := listDir(l.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			if err := l.moveAuditLogFile(fi.Name()); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// SearchEvents finds events. Results show up sorted by date (newest first),
// limit is used when set to value > 0
func (l *AuditLog) SearchEvents(fromUTC, toUTC time.Time, query string, limit int) ([]EventFields, error) {
	l.Debugf("SearchEvents(%v, %v, query=%v, limit=%v)", fromUTC, toUTC, query, limit)
	if limit <= 0 {
		limit = defaults.EventsIterationLimit
	}
	if limit > defaults.EventsMaxIterationLimit {
		return nil, trace.BadParameter("limit %v exceeds max iteration limit %v", limit, defaults.MaxIterationLimit)
	}
	// how many days of logs to search?
	days := int(toUTC.Sub(fromUTC).Hours() / 24)
	if days < 0 {
		return nil, trace.BadParameter("query", query)
	}
	queryVals, err := url.ParseQuery(query)
	if err != nil {
		return nil, trace.BadParameter("missing parameter query", query)
	}
	filtered, err := l.matchingFiles(fromUTC, toUTC)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var total int
	// search within each file:
	events := make([]EventFields, 0)
	for i := range filtered {
		found, err := l.findInFile(filtered[i].path, queryVals, &total, limit)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, found...)
		if limit > 0 && total >= limit {
			break
		}
	}
	// sort all accepted files by timestamp or by event index
	// in case if events are associated with the same session, to make
	// sure that events are not displayed out of order in case of multiple
	// auth servers.
	sort.Sort(byTimeAndIndex(events))
	return events, nil
}

// SearchSessionEvents searches for session related events. Used to find completed sessions.
func (l *AuditLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int) ([]EventFields, error) {
	l.Debugf("SearchSessionEvents(%v, %v, %v)", fromUTC, toUTC, limit)

	// only search for specific event types
	query := url.Values{}
	query[EventType] = []string{
		SessionStartEvent,
		SessionEndEvent,
	}

	// because of the limit and distributed nature of auth server event
	// logs, some events can be fetched with session end event and without
	// session start event. to fix this, the code below filters out the events without
	// start event to guarantee that all events in the range will get fetched
	events, err := l.SearchEvents(fromUTC, toUTC, query.Encode(), limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// filter out 'session end' events that do not
	// have a corresponding 'session start' event
	started := make(map[string]struct{}, len(events)/2)
	filtered := make([]EventFields, 0, len(events))
	for i := range events {
		event := events[i]
		eventType := event[EventType]
		sessionID := event.GetString(SessionEventID)
		if sessionID == "" {
			continue
		}
		if eventType == SessionStartEvent {
			started[sessionID] = struct{}{}
			filtered = append(filtered, event)
		}
		if eventType == SessionEndEvent {
			if _, ok := started[sessionID]; ok {
				filtered = append(filtered, event)
			}
		}
	}

	return filtered, nil
}

type eventFile struct {
	os.FileInfo
	path string
}

// byDate implements sort.Interface.
type byDate []eventFile

func (f byDate) Len() int           { return len(f) }
func (f byDate) Less(i, j int) bool { return f[i].ModTime().Before(f[j].ModTime()) }
func (f byDate) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// byTimeAndIndex sorts events by time extracting timestamp from JSON field
// and if there are several session events with the same session
// by event index, regardless of the time
type byTimeAndIndex []EventFields

func (f byTimeAndIndex) Len() int {
	return len(f)
}

// getTime converts json time to string
func getTime(v interface{}) time.Time {
	sval, ok := v.(string)
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, sval)
	if err != nil {
		return time.Time{}
	}
	return t
}

func getEventIndex(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	}
	return 0
}

func (f byTimeAndIndex) Less(i, j int) bool {
	itime := getTime(f[i][EventTime])
	jtime := getTime(f[j][EventTime])
	if itime.Equal(jtime) && f[i][SessionEventID] == f[j][SessionEventID] {
		return getEventIndex(f[i][EventIndex]) < getEventIndex(f[j][EventIndex])
	}
	return itime.Before(jtime)
}

func (f byTimeAndIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// findInFile scans a given log file and returns events that fit the criteria
// This simplistic implementation ONLY SEARCHES FOR EVENT TYPE(s)
//
// You can pass multiple types like "event=session.start&event=session.end"
func (l *AuditLog) findInFile(fn string, query url.Values, total *int, limit int) ([]EventFields, error) {
	l.Debugf("Called findInFile(%s, %v).", fn, query)
	retval := make([]EventFields, 0)

	eventFilter, ok := query[EventType]
	if !ok && len(query) > 0 {
		return nil, nil
	}
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
			*total += 1
			if limit > 0 && *total >= limit {
				break
			}
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
		logfname := filepath.Join(l.DataDir, l.ServerID,
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

// LoggerFor creates a logger for a specified session. Session loggers allow
// to group all events into special "session log files" for easier audit
func (l *AuditLog) LoggerFor(namespace string, sid session.ID, compatibilityMode bool) (SessionLogger, error) {
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
	sdir := filepath.Join(l.DataDir, l.ServerID, SessionLogsDir, namespace)
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
	// DELETE IN: 2.6.0
	// Compatibility mode is required for migration from 2.5.0 to 2.6.0
	if compatibilityMode {
		l.Debugf("Using compatibility mode logger for session id %v.", sid)
		sessionLogger, err := NewCompatSessionLogger(CompatSessionLoggerConfig{
			SessionID:      sid,
			DataDir:        sdir,
			Clock:          l.Clock,
			RecordSessions: l.RecordSessions,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		l.loggers.Set(string(sid), sessionLogger, l.SessionIdlePeriod)
		auditOpenFiles.Inc()
		return sessionLogger, nil
	} else {
		sessionLogger, err := NewDiskSessionLogger(DiskSessionLoggerConfig{
			SessionID:      sid,
			DataDir:        sdir,
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

}

func (l *AuditLog) closeSessionLogger(key string, val interface{}) {
	l.Debugf("Closing session logger %v.", key)
	logger, ok := val.(SessionLogger)
	if !ok {
		l.Warningf("Warning, not valid value type %T for %v.", val, key)
		return
	}
	if err := logger.Finalize(); err != nil {
		log.Warningf("Failed to finalize: %v.", trace.DebugReport(err))
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

func (l *AuditLog) periodicCleanupPlaybacks() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.cleanupOldPlaybacks(); err != nil {
				l.Warningf("Error while cleaning up playback files: %v.", err)
			}
		}
	}
}

func (l *AuditLog) closeInactiveLoggers() {
	l.Lock()
	defer l.Unlock()

	expired := l.loggers.RemoveExpired(10)
	if expired != 0 {
		l.Debugf("Closed %v inactive session loggers.", expired)
	}
}
