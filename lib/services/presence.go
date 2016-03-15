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

package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

// PresenceService records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type PresenceService struct {
	backend backend.Backend
}

// NewPresenceService returns new presence service instance
func NewPresenceService(backend backend.Backend) *PresenceService {
	return &PresenceService{backend}
}

func (s *PresenceService) getServers(prefix string) ([]Server, error) {
	IDs, err := s.backend.GetKeys([]string{prefix})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]Server, len(IDs))
	for i, id := range IDs {
		data, err := s.backend.GetVal([]string{prefix}, id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := json.Unmarshal(data, &servers[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// sorting helps with tests and makes it all deterministic
	sort.Sort(sortedServers(servers))
	return servers, nil
}

type sortedServers []Server

func (s sortedServers) Len() int {
	return len(s)
}

func (s sortedServers) Less(i, j int) bool {
	return s[i].ID < s[j].ID
}

func (s sortedServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s *PresenceService) upsertServer(prefix string, server Server, ttl time.Duration) error {
	data, err := json.Marshal(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{prefix}, server.ID, data, ttl)
	return trace.Wrap(err)
}

const (
	nodesPrefix       = "nodes"
	authServersPrefix = "authservers"
	proxiesPrefix     = "proxies"
)

// GetNodes returns a list of registered servers
func (s *PresenceService) GetNodes() ([]Server, error) {
	return s.getServers(nodesPrefix)
}

// UpsertNode registers node presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertNode(server Server, ttl time.Duration) error {
	return s.upsertServer(nodesPrefix, server, ttl)
}

// GetAuthServers returns a list of registered servers
func (s *PresenceService) GetAuthServers() ([]Server, error) {
	return s.getServers(authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(server Server, ttl time.Duration) error {
	return s.upsertServer(authServersPrefix, server, ttl)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertProxy(server Server, ttl time.Duration) error {
	return s.upsertServer(proxiesPrefix, server, ttl)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]Server, error) {
	return s.getServers(proxiesPrefix)
}

// Site represents a cluster of teleport nodes who collectively trust the same
// certificate authority (CA) and have a common name.
//
// The CA is represented by an auth server (or multiple auth servers, if running
// in HA mode)
type Site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"lastconnected"`
	Status        string    `json:"status"`
}

// Server represents a node in a Teleport cluster
type Server struct {
	ID        string                  `json:"id"`
	Addr      string                  `json:"addr"`
	Hostname  string                  `json:"hostname"`
	Labels    map[string]string       `json:"labels"`
	CmdLabels map[string]CommandLabel `json:"cmd_labels"`
}

type CommandLabel struct {
	Period  time.Duration `json:"period"`
	Command []string      `json:"command"` //["cmd", "arg1", "arg2"]
	Result  string        `json:"result"`
}

type CommandLabels map[string]CommandLabel

func (c *CommandLabels) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), c); err != nil {
		return trace.Wrap(err, "Can't parse Command Labels")
	}
	return nil
}

// LabelsMap returns the full key:value map of both static labels and
// "command labels"
func (s *Server) LabelsMap() map[string]string {
	lmap := make(map[string]string)
	for key, value := range s.Labels {
		lmap[key] = value
	}
	for key, cmd := range s.CmdLabels {
		lmap[key] = cmd.Result
	}
	return lmap
}

// MatchAgainst takes a map of labels and returns True if this server
// has ALL of them
//
// Any server matches against an empty label set
func (s *Server) MatchAgainst(labels map[string]string) bool {
	if labels != nil {
		myLabels := s.LabelsMap()
		for key, value := range labels {
			if myLabels[key] != value {
				return false
			}
		}
	}
	return true
}

// LabelsString returns a comma separated string with all node's labels
func (s *Server) LabelsString() string {
	labels := []string{}
	for key, val := range s.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range s.CmdLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val.Result))
	}
	return strings.Join(labels, ",")
}
