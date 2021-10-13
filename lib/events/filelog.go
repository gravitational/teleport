/*
Copyright 2018-2019 Gravitational, Inc.

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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// FileLogConfig is a configuration for file log
type FileLogConfig struct {
	// RotationPeriod defines how frequently to rotate the log file
	RotationPeriod time.Duration
	// Dir is a directory where logger puts the files
	Dir string
	// SymlinkDir is a directory for symlink pointer to the current log
	SymlinkDir string
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is used to generate unique IDs for events
	UIDGenerator utils.UID
	// SearchDirs is a function that returns
	// search directories, if not set, only Dir is used
	SearchDirs func() ([]string, error)
}

// CheckAndSetDefaults checks and sets config defaults
func (cfg *FileLogConfig) CheckAndSetDefaults() error {
	if cfg.Dir == "" {
		return trace.BadParameter("missing parameter Dir")
	}
	if !utils.IsDir(cfg.Dir) {
		return trace.BadParameter("path %q does not exist or is not a directory", cfg.Dir)
	}
	if cfg.SymlinkDir == "" {
		cfg.SymlinkDir = cfg.Dir
	}
	if !utils.IsDir(cfg.SymlinkDir) {
		return trace.BadParameter("path %q does not exist or is not a directory", cfg.SymlinkDir)
	}
	if cfg.RotationPeriod == 0 {
		cfg.RotationPeriod = defaults.LogRotationPeriod
	}
	if cfg.RotationPeriod%(24*time.Hour) != 0 {
		return trace.BadParameter("rotation period %v is not a multiple of 24 hours, e.g. '24h' or '48h'", cfg.RotationPeriod)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UIDGenerator == nil {
		cfg.UIDGenerator = utils.NewRealUID()
	}
	return nil
}

// NewFileLog returns a new instance of a file log
func NewFileLog(cfg FileLogConfig) (*FileLog, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	f := &FileLog{
		FileLogConfig: cfg,
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAuditLog,
		}),
	}
	return f, nil
}

// FileLog is a file local audit events log,
// logs all events to the local file in json encoded form
type FileLog struct {
	*log.Entry
	FileLogConfig
	// rw protects the file from rotation during concurrent
	// event emission.
	rw sync.RWMutex
	// file is the current global event log file. As the time goes
	// on, it will be replaced by a new file every day.
	file *os.File
	// fileTime is a rounded (to a day, by default) timestamp of the
	// currently opened file
	fileTime time.Time
}

// EmitAuditEvent adds a new event to the log.
func (l *FileLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	l.rw.RLock()
	defer l.rw.RUnlock()

	// see if the log needs to be rotated
	if l.mightNeedRotation() {
		// log might need rotation; switch to write-lock
		// to avoid rotating during concurrent event emission.
		l.rw.RUnlock()
		l.rw.Lock()

		// perform rotation if still necessary (rotateLog rechecks the
		// requirements internally, since rotation may have been performed
		// during our switch from read to write locks)
		err := l.rotateLog()

		// switch back to read lock
		l.rw.Unlock()
		l.rw.RLock()
		if err != nil {
			log.Error(err)
		}
	}

	// line is the text to be logged
	line, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	if l.file == nil {
		return trace.NotFound(
			"file log is not found due to permission or disk issue")
	}
	// log it to the main log file:
	_, err = fmt.Fprintln(l.file, string(line))
	return trace.ConvertSystemError(err)
}

// EmitAuditEventLegacy adds a new event to the log. Part of auth.IFileLog interface.
func (l *FileLog) EmitAuditEventLegacy(event Event, fields EventFields) error {
	l.rw.RLock()
	defer l.rw.RUnlock()

	// see if the log needs to be rotated
	if l.mightNeedRotation() {
		// log might need rotation; switch to write-lock
		// to avoid rotating during concurrent event emission.
		l.rw.RUnlock()
		l.rw.Lock()

		// perform rotation if still necessary (rotateLog rechecks the
		// requirements internally, since rotation may have been performed
		// during our switch from read to write locks)
		err := l.rotateLog()

		// switch back to read lock
		l.rw.Unlock()
		l.rw.RLock()
		if err != nil {
			log.Error(err)
		}
	}

	err := UpdateEventFields(event, fields, l.Clock, l.UIDGenerator)
	if err != nil {
		log.Error(err)
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

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC).
//
// This function may never return more than 1 MiB of event data.
func (l *FileLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startAfter string) ([]apievents.AuditEvent, string, error) {
	l.Debugf("SearchEvents(%v, %v, namespace=%v, eventType=%v, limit=%v)", fromUTC, toUTC, namespace, eventTypes, limit)
	return l.searchEventsWithFilter(fromUTC, toUTC, namespace, limit, order, startAfter, searchEventsFilter{eventTypes: eventTypes})
}

func (l *FileLog) searchEventsWithFilter(fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startAfter string, filter searchEventsFilter) ([]apievents.AuditEvent, string, error) {
	if limit <= 0 {
		limit = defaults.EventsIterationLimit
	}
	if limit > defaults.EventsMaxIterationLimit {
		return nil, "", trace.BadParameter("limit %v exceeds max iteration limit %v", limit, defaults.MaxIterationLimit)
	}

	// how many days of logs to search?
	days := int(toUTC.Sub(fromUTC).Hours() / 24)
	if days < 0 {
		return nil, "", trace.BadParameter("invalid days")
	}
	filesToSearch, err := l.matchingFiles(fromUTC, toUTC, order)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	dynamicEvents := make([]EventFields, 0)

	// Fetch events from each file for further filtering.
	for _, file := range filesToSearch {
		eventsFromFile, err := l.findInFile(file.path, filter)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		dynamicEvents = append(dynamicEvents, eventsFromFile...)
	}

	// sort all accepted files by timestamp or by event index
	// in case if events are associated with the same session, to make
	// sure that events are not displayed out of order in case of multiple
	// auth servers.
	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = ByTimeAndIndex(dynamicEvents)
	case types.EventOrderDescending:
		toSort = sort.Reverse(ByTimeAndIndex(dynamicEvents))
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}
	sort.Sort(toSort)

	events := make([]apievents.AuditEvent, 0, len(dynamicEvents))

	// This is used as a flag to check if we have found the startAfter checkpoint or not.
	foundStart := startAfter == ""

	totalSize := 0

outer:
	for _, dynamicEvent := range dynamicEvents {
		// Convert the event from a dynamic representation to a typed representation.
		event, err := FromEventFields(dynamicEvent)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		size, err := estimateEventSize(dynamicEvent)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		// Skip until we've found the start checkpoint and once more
		// since it was the last key of the previous set.
		if !foundStart {
			checkpoint, err := getCheckpointFromEvent(event)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			if startAfter == checkpoint {
				foundStart = true
			}

			continue
		}

		// Skip until we've found the first event within the desired timeframe.
		switch order {
		case types.EventOrderAscending:
			if event.GetTime().Before(fromUTC) {
				continue outer
			}
		case types.EventOrderDescending:
			if event.GetTime().After(toUTC) {
				continue outer
			}
		}

		// If we've found an event after the desired timeframe, all events from here
		// on out will also be after the desired timeframe due
		// to the sort so we just break out here and consider the query as finished.
		switch order {
		case types.EventOrderAscending:
			if event.GetTime().After(toUTC) {
				break outer
			}
		case types.EventOrderDescending:
			if event.GetTime().Before(fromUTC) {
				break outer
			}
		}

		if totalSize+size >= MaxEventBytesInResponse {
			checkpoint, err := getCheckpointFromEvent(events[len(events)-1])
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return events, checkpoint, nil
		}

		events = append(events, event)
		totalSize += size

		// Check if there is a limit and if so, check if we've hit it.
		// In the event that we've hit the limit, we consider the query partially complete
		// and return a checkpoint to continue it.
		if len(events) >= limit && limit > 0 {
			checkpoint, err := getCheckpointFromEvent(events[len(events)-1])
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return events, checkpoint, nil
		}
	}

	// This return point is only hit if the query is finished and there are no further pages.
	return events, "", nil
}

func getCheckpointFromEvent(event apievents.AuditEvent) (string, error) {
	if event.GetID() == "" {
		data, err := utils.FastMarshal(event)
		if err != nil {
			return "", trace.Wrap(err)
		}

		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:]), nil
	}

	return event.GetID(), nil
}

func (l *FileLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) ([]apievents.AuditEvent, string, error) {
	l.Debugf("SearchSessionEvents(%v, %v, order=%v, limit=%v, cond=%q)", fromUTC, toUTC, order, limit, cond)
	filter := searchEventsFilter{eventTypes: []string{SessionEndEvent}}
	if cond != nil {
		condFn, err := ToEventFieldsCondition(cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}
	events, lastKey, err := l.searchEventsWithFilter(fromUTC, toUTC, apidefaults.Namespace, limit, order, startKey, filter)
	return events, lastKey, trace.Wrap(err)
}

type searchEventsFilter struct {
	eventTypes []string
	condition  EventFieldsCondition
}

// Close closes the audit log, which inluces closing all file handles and releasing
// all session loggers
func (l *FileLog) Close() error {
	l.rw.Lock()
	defer l.rw.Unlock()

	var err error
	if l.file != nil {
		err = l.file.Close()
		l.file = nil
	}
	return err
}

func (l *FileLog) WaitForDelivery(context.Context) error {
	return nil
}

func (l *FileLog) UploadSessionRecording(SessionRecording) error {
	return trace.NotImplemented("not implemented")
}

func (l *FileLog) PostSessionSlice(slice SessionSlice) error {
	if slice.Namespace == "" {
		return trace.BadParameter("missing parameter Namespace")
	}
	if len(slice.Chunks) == 0 {
		return trace.BadParameter("missing session chunks")
	}
	if slice.Version < V3 {
		return trace.BadParameter("audit log rejected V%v log entry, upgrade your components.", slice.Version)
	}
	// V3 API does not write session log to local session directory,
	// instead it writes locally, this internal method captures
	// non-print events to the global audit log
	return l.processSlice(nil, &slice)
}

func (l *FileLog) processSlice(sl SessionLogger, slice *SessionSlice) error {
	for _, chunk := range slice.Chunks {
		if chunk.EventType == SessionPrintEvent || chunk.EventType == "" {
			continue
		}
		fields, err := EventFromChunk(slice.SessionID, chunk)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := l.EmitAuditEventLegacy(Event{Name: chunk.EventType}, fields); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (l *FileLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (l *FileLog) GetSessionEvents(namespace string, sid session.ID, after int, fetchPrintEvents bool) ([]EventFields, error) {
	return nil, trace.NotImplemented("not implemented")
}

// mightNeedRotation checks if the current log file looks older than a given duration,
// used by rotateLog to decide if it should acquire a write lock.  Must be called under
// read lock.
func (l *FileLog) mightNeedRotation() bool {

	if l.file == nil {
		return true
	}

	// determine the timestamp for the current log file rounded to the day.
	fileTime := l.Clock.Now().UTC().Truncate(24 * time.Hour)

	return l.fileTime.Before(fileTime)
}

// rotateLog checks if the current log file is older than a given duration,
// and if it is, closes it and opens a new one.  Must be called under write lock.
func (l *FileLog) rotateLog() (err error) {

	// determine the timestamp for the current log file rounded to the day.
	fileTime := l.Clock.Now().UTC().Truncate(24 * time.Hour)

	logFilename := filepath.Join(l.Dir,
		fileTime.Format(defaults.AuditLogTimeFormat)+LogfileExt)

	openLogFile := func() error {
		l.file, err = os.OpenFile(logFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Error(err)
		}
		l.fileTime = fileTime
		return trace.Wrap(err)
	}

	linkFilename := filepath.Join(l.SymlinkDir, SymlinkFilename)
	createSymlink := func() error {
		err = trace.ConvertSystemError(os.Remove(linkFilename))
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		return trace.ConvertSystemError(os.Symlink(logFilename, linkFilename))
	}

	// need to create a log file?
	if l.file == nil {
		if err := openLogFile(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(createSymlink())
	}

	// time to advance the logfile?
	if l.fileTime.Before(fileTime) {
		l.file.Close()
		if err := openLogFile(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(createSymlink())
	}
	return nil
}

// matchingFiles returns files matching the time restrictions of the query
// across multiple auth servers, returns a list of file names
func (l *FileLog) matchingFiles(fromUTC, toUTC time.Time, order types.EventOrder) ([]eventFile, error) {
	var dirs []string
	var err error
	if l.SearchDirs != nil {
		dirs, err = l.SearchDirs()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		dirs = []string{l.Dir}
	}

	var filtered []eventFile
	for _, dir := range dirs {
		// scan the log directory:
		df, err := os.Open(dir)
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
			fd, err := parseFileTime(fi.Name())
			if err != nil {
				l.Warningf("Failed to parse audit log file %q format: %v", fi.Name(), err)
				continue
			}
			// File rounding in current logs is non-deterministic,
			// as Round function used in rotateLog can round up to the lowest
			// or the highest period. That's why this has to check both
			// periods.
			// Previous logic used modification time what was flaky
			// as it could be changed by migrations or simply moving files
			if fd.After(fromUTC.Add(-1*l.RotationPeriod)) && fd.Before(toUTC.Add(l.RotationPeriod)) {
				eventFile := eventFile{
					FileInfo: fi,
					path:     filepath.Join(dir, fi.Name()),
				}
				filtered = append(filtered, eventFile)
			}
		}
	}
	// sort all accepted files by date
	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = byDate(filtered)
	case types.EventOrderDescending:
		toSort = sort.Reverse(byDate(filtered))
	default:
		return nil, trace.BadParameter("invalid event order: %v", order)
	}
	sort.Sort(toSort)
	return filtered, nil
}

// parseFileTime parses file's timestamp encoded into filename
func parseFileTime(filename string) (time.Time, error) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	return time.Parse(defaults.AuditLogTimeFormat, base)
}

// findInFile scans a given log file and returns events that fit the criteria.
func (l *FileLog) findInFile(path string, filter searchEventsFilter) ([]EventFields, error) {
	l.Debugf("Called findInFile(%s, %+v).", path, filter)

	// open the log file:
	lf, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer lf.Close()

	// for each line...
	retval := []EventFields{}
	scanner := bufio.NewScanner(lf)
	for lineNo := 0; scanner.Scan(); lineNo++ {
		// Optimization: to avoid parsing JSON unnecessarily, we filter out lines
		// that don't contain the event type.
		match := len(filter.eventTypes) == 0
		for _, eventType := range filter.eventTypes {
			if strings.Contains(scanner.Text(), eventType) {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		// parse JSON on the line and compare event type field to what's
		// in the query:
		var ef EventFields
		if err = json.Unmarshal(scanner.Bytes(), &ef); err != nil {
			l.Warnf("invalid JSON in %s line %d", path, lineNo)
			continue
		}
		accepted := len(filter.eventTypes) == 0
		for _, eventType := range filter.eventTypes {
			if ef.GetString(EventType) == eventType {
				accepted = true
				break
			}
		}
		if !accepted {
			continue
		}
		// Check that the filter condition is satisfied.
		if filter.condition != nil {
			accepted = accepted && filter.condition(ef)
		}

		if accepted {
			retval = append(retval, ef)
		}
	}

	return retval, nil
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise it is simply closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (l *FileLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	e <- trace.NotImplemented("not implemented")
	return c, e
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

// ByTimeAndIndex sorts events by time extracting timestamp from JSON field
// and if there are several session events with the same session
// by event index, regardless of the time
type ByTimeAndIndex []EventFields

func (f ByTimeAndIndex) Len() int {
	return len(f)
}

func (f ByTimeAndIndex) Less(i, j int) bool {
	itime := getTime(f[i][EventTime])
	jtime := getTime(f[j][EventTime])
	if itime.Equal(jtime) && f[i][SessionEventID] == f[j][SessionEventID] {
		return getEventIndex(f[i][EventIndex]) < getEventIndex(f[j][EventIndex])
	}
	return itime.Before(jtime)
}

func (f ByTimeAndIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
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
