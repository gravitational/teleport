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
	"encoding/binary"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/peterbourgon/diskv/v3"

	"github.com/gravitational/teleport/integrations/event-handler/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
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

	// idName is the id variable name
	idName = "id"

	// sessionPrefix is the session key prefix
	sessionPrefix = "session"

	// storageDirPerms is storage directory permissions when created
	storageDirPerms = 0755
)

// State manages the plugin persistent state. It is stored on disk as a simple key-value database.
type State struct {
	// dv is a diskv instance
	dv *diskv.Diskv
}

// NewCursor creates new cursor instance
func NewState(c *StartCmdConfig) (*State, error) {
	// Simplest transform function: put all the data files into the base dir.
	flatTransform := func(s string) []string { return []string{} }

	dir, err := createStorageDir(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dv := diskv.New(diskv.Options{
		BasePath:     dir,
		Transform:    flatTransform,
		CacheSizeMax: cacheSizeMaxBytes,
	})

	s := State{dv}

	return &s, nil
}

// createStorageDir is used to calculate storage dir path and create dir if it does not exits
func createStorageDir(c *StartCmdConfig) (string, error) {
	log := logger.Standard()

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

		dir = path.Join(dir, "dry_run", rs)
	}

	dir = path.Join(c.StorageDir, dir)

	_, err = os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, storageDirPerms)
		if err != nil {
			return "", trace.Errorf("Can not create storage directory %v : %w", dir, err)
		}

		log.WithField("dir", dir).Info("Created storage directory")
	} else {
		log.WithField("dir", dir).Info("Using existing storage directory")
	}

	return dir, nil
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

// RemoveSession removes session from the state
func (s *State) RemoveSession(id string) error {
	return s.dv.Erase(sessionPrefix + id)
}
