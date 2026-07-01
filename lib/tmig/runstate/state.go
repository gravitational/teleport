// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const stateFile = "runstate.json"

// ClusterIdentity records a pinned cluster identity.
type ClusterIdentity struct {
	Name          string `json:"name"`
	ClusterID     string `json:"cluster_id"`
	Proxy         string `json:"proxy"`
	CAFingerprint string `json:"ca_fingerprint"`
	User          string `json:"user"`
	Version       string `json:"version"`
	ScopePinned   bool   `json:"scope_pinned"`
}

// HostState records per-host progress.
type HostState struct {
	Hostname string `json:"hostname"`
	Verdict  string `json:"verdict"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

type stateData struct {
	Identities map[string]ClusterIdentity `json:"identities"`
	Hosts      map[string]HostState       `json:"hosts"`
}

// State manages per-run persistent state.
type State struct {
	mu   sync.Mutex
	dir  string
	data stateData
}

// New creates or loads state from the given directory.
func New(dir string) (*State, error) {
	s := &State{
		dir: dir,
		data: stateData{
			Identities: make(map[string]ClusterIdentity),
			Hosts:      make(map[string]HostState),
		},
	}
	path := s.Path()
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.data); err != nil {
			return nil, fmt.Errorf("parsing state: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	return s, nil
}

// Path returns the filesystem path to the state file.
func (s *State) Path() string {
	return filepath.Join(s.dir, stateFile)
}

// Save persists the state to disk.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path(), data, 0644)
}

// SetHost records state for a host by UUID.
func (s *State) SetHost(uuid string, entry HostState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Hosts[uuid] = entry
}

// GetHost retrieves a host's state by UUID.
func (s *State) GetHost(uuid string) (HostState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.data.Hosts[uuid]
	return h, ok
}

// SetIdentity records a pinned cluster identity.
func (s *State) SetIdentity(cluster string, id ClusterIdentity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Identities[cluster] = id
}

// PendingHosts returns UUIDs of hosts that are not yet satisfied.
func (s *State) PendingHosts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []string
	for uuid, h := range s.data.Hosts {
		if h.Status == "PENDING" || h.Verdict == "" {
			pending = append(pending, uuid)
		}
	}
	return pending
}
