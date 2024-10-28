/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/peterbourgon/diskv/v3"

	"github.com/gravitational/teleport/lib/events/export"

	"github.com/gravitational/teleport/integrations/event-handler/lib"
)

const (
	// cacheSizeMaxBytes max memory cache
	cacheSizeMaxBytes = 1024

	// startTimeName is the start time variable name
	startTimeName = "start_time"

	// windowTimeName is the start time of the last window.
	windowTimeName = "window_time"

	// cursorName is the cursor variable name
	cursorName = "cursor"

	// cursorV2Dir is the cursor v2 directory
	cursorV2Dir = "cursor_v2"

	// idName is the id variable name
	idName = "id"

	// sessionPrefix is the session key prefix
	sessionPrefix = "session"

	// missingRecordingPrefix is the missing recording key prefix
	missingRecordingPrefix = "missing_recording"

	// storageDirPerms is storage directory permissions when created
	storageDirPerms = 0755
)

// State manages the plugin persistent state. It is stored on disk as a simple key-value database.
type State struct {
	// dv is a diskv instance
	dv *diskv.Diskv

	// cursorV2 is an export cursor. if the event handler was started before
	// introduction of the v2 cursor or is talking to an auth that does not
	// implement the newer bulk export apis, the v1 cursor stored in the above
	// dv may be the source of truth still.
	cursorV2 *export.Cursor

	log *slog.Logger
}

// NewCursor creates new cursor instance
func NewState(c *StartCmdConfig, log *slog.Logger) (*State, error) {
	// Simplest transform function: put all the data files into the base dir.
	flatTransform := func(s string) []string { return []string{} }

	dir, err := createStorageDir(c, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dv := diskv.New(diskv.Options{
		BasePath:     dir,
		Transform:    flatTransform,
		CacheSizeMax: cacheSizeMaxBytes,
	})

	cursorV2, err := export.NewCursor(export.CursorConfig{
		Dir: filepath.Join(dir, cursorV2Dir),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := State{
		dv:       dv,
		cursorV2: cursorV2,
		log:      log,
	}

	return &s, nil
}

// createStorageDir is used to calculate storage dir path and create dir if it does not exits
func createStorageDir(c *StartCmdConfig, log *slog.Logger) (string, error) {
	host, port, err := net.SplitHostPort(c.TeleportAddr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	dir := strings.TrimSpace(host + "_" + port)
	if dir == "_" {
		return "", trace.Errorf("Can not generate cursor name from Teleport host %s", c.TeleportAddr)
	}

	if c.DryRun {
		rs, err := lib.RandomString(32)
		if err != nil {
			return "", trace.Wrap(err)
		}

		dir = filepath.Join(dir, "dry_run", rs)
	}

	dir = filepath.Join(c.StorageDir, dir)

	_, err = os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, storageDirPerms)
		if err != nil {
			return "", trace.Errorf("Can not create storage directory %v : %w", dir, err)
		}

		log.InfoContext(context.TODO(), "Created storage directory", "dir", dir)
	} else {
		log.InfoContext(context.TODO(), "Using existing storage directory", "dir", dir)
	}

	return dir, nil
}

func (s *State) GetCursorV2State() export.ExporterState {
	return s.cursorV2.GetState()
}

func (s *State) SetCursorV2State(state export.ExporterState) error {
	return s.cursorV2.Sync(state)
}

func (s *State) GetLegacyCursorValues() (*LegacyCursorValues, error) {
	latestCursor, err := s.GetCursor()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	latestID, err := s.GetID()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lastWindowTime, err := s.GetLastWindowTime()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var windowStartTime time.Time
	if lastWindowTime != nil {
		windowStartTime = *lastWindowTime
	}

	lcv := &LegacyCursorValues{
		Cursor:          latestCursor,
		ID:              latestID,
		WindowStartTime: windowStartTime,
	}

	return lcv, nil
}

func (s *State) SetLegacyCursorValues(v LegacyCursorValues) error {
	if err := s.SetCursor(v.Cursor); err != nil {
		return trace.Wrap(err)
	}

	if err := s.SetID(v.ID); err != nil {
		return trace.Wrap(err)
	}

	return s.SetLastWindowTime(&v.WindowStartTime)
}

// GetStartTime gets current start time
func (s *State) GetStartTime() (*time.Time, error) {
	return s.getTimeKey(startTimeName)
}

// SetStartTime sets current start time
func (s *State) SetStartTime(t *time.Time) error {
	return s.setTimeKey(startTimeName, t)
}

// GetLastWindowTime gets current start time
func (s *State) GetLastWindowTime() (*time.Time, error) {
	return s.getTimeKey(windowTimeName)
}

func (s *State) getTimeKey(keyName string) (*time.Time, error) {
	if !s.dv.Has(keyName) {
		return nil, nil
	}

	b, err := s.dv.Read(keyName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// No previous start time exist
	if string(b) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, string(b))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	t = t.Truncate(time.Second)
	return &t, nil
}

func (s *State) setTimeKey(keyName string, t *time.Time) error {
	if t == nil {
		return s.dv.Write(keyName, []byte(""))
	}

	v := t.Truncate(time.Second).Format(time.RFC3339)
	return s.dv.Write(keyName, []byte(v))
}

// SetLastWindowTime sets current start time of the last window used.
func (s *State) SetLastWindowTime(t *time.Time) error {
	return s.setTimeKey(windowTimeName, t)
}

// GetCursor gets current cursor value
func (s *State) GetCursor() (string, error) {
	return s.getStringValue(cursorName)
}

// SetCursor sets cursor value
func (s *State) SetCursor(v string) error {
	return s.setStringValue(cursorName, v)
}

// GetID gets current ID value
func (s *State) GetID() (string, error) {
	return s.getStringValue(idName)
}

// SetID sets cursor value
func (s *State) SetID(v string) error {
	return s.setStringValue(idName, v)
}

// getStringValue gets a string value
func (s *State) getStringValue(name string) (string, error) {
	if !s.dv.Has(name) {
		return "", nil
	}

	b, err := s.dv.Read(name)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(b), err
}

// setStringValue sets string value
func (s *State) setStringValue(name string, value string) error {
	err := s.dv.Write(name, []byte(value))
	return trace.Wrap(err)
}

// GetSessions get active sessions (map[id]index)
func (s *State) GetSessions() (map[string]int64, error) {
	r := make(map[string]int64)

	for key := range s.dv.KeysPrefix(sessionPrefix, nil) {
		b, err := s.dv.Read(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		id := key[len(sessionPrefix):]
		r[id] = int64(binary.BigEndian.Uint64(b))
	}

	return r, nil
}

// SetSessionIndex writes current session index into state
func (s *State) SetSessionIndex(id string, index int64) error {
	var b = make([]byte, 8)

	binary.BigEndian.PutUint64(b, uint64(index))

	return s.dv.Write(sessionPrefix+id, b)
}

// SetMissingRecording writes the session with missing recording into state.
func (s *State) SetMissingRecording(sess session, attempt int) error {
	b, err := json.Marshal(missingRecording{
		Index:     sess.Index,
		Attempt:   attempt,
		Timestamp: sess.UploadTime,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.dv.Write(missingRecordingPrefix+sess.ID, b); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.RemoveSession(sess.ID))
}

type missingRecording struct {
	ID        string `json:"id,omitempty"`
	Index     int64
	Attempt   int
	Timestamp time.Time
}

// IterateMissingRecordings finds any sessions with a missing recording and
// provides them to the callback for processing.
func (s *State) IterateMissingRecordings(callback func(s session, attempts int) error) error {
	closeC := make(chan struct{})
	defer close(closeC)
	for key := range s.dv.KeysPrefix(missingRecordingPrefix, closeC) {
		b, err := s.dv.Read(key)
		if err != nil {
			// Ignore any errors caused by the file being removed
			// by an external entity.
			var pathError *fs.PathError
			if !errors.Is(err, fs.ErrNotExist) ||
				errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOENT) {
				continue
			}

			return trace.Wrap(err)
		}

		var m missingRecording
		if err := json.Unmarshal(b, &m); err != nil {
			s.log.WarnContext(
				context.TODO(),
				"Failed to unmarshal missing recording from persisted state",
				"key", key,
				"error", err,
			)
			continue
		}

		s := session{
			ID:         key[len(missingRecordingPrefix):],
			Index:      m.Index,
			UploadTime: m.Timestamp,
		}

		if err := callback(s, m.Attempt); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// RemoveMissingRecording removes the session with a missing recording from state.
func (s *State) RemoveMissingRecording(id string) error {
	err := s.dv.Erase(missingRecordingPrefix + id)
	if err == nil {
		return nil
	}

	// If the session had no events, the file won't exist, so we ignore the error
	var pathError *fs.PathError
	if !errors.Is(err, fs.ErrNotExist) ||
		errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOENT) {
		return nil
	}

	return trace.Wrap(err)
}

// RemoveSession removes session from the state.
func (s *State) RemoveSession(id string) error {
	err := s.dv.Erase(sessionPrefix + id)
	if err == nil {
		return nil
	}

	// If the session had no events, the file won't exist, so we ignore the error
	var pathError *fs.PathError
	if !errors.Is(err, fs.ErrNotExist) ||
		errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOENT) {
		return nil
	}

	return trace.Wrap(err)
}
