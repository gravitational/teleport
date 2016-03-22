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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure/cstrings"
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
	keys, err := s.backend.GetKeys([]string{prefix})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]Server, len(keys))
	for i, key := range keys {
		data, err := s.backend.GetVal([]string{prefix}, key)
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

func (s *PresenceService) upsertServer(prefix string, server Server, ttl time.Duration) error {
	data, err := json.Marshal(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{prefix}, server.ID, data, ttl)
	return trace.Wrap(err)
}

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

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (s *PresenceService) UpsertReverseTunnel(tunnel ReverseTunnel, ttl time.Duration) error {
	if err := tunnel.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{reverseTunnelsPrefix}, tunnel.DomainName, data, ttl)
	return trace.Wrap(err)
}

// GetReverseTunnels returns a list of registered servers
func (s *PresenceService) GetReverseTunnels() ([]ReverseTunnel, error) {
	keys, err := s.backend.GetKeys([]string{reverseTunnelsPrefix})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]ReverseTunnel, len(keys))
	for i, key := range keys {
		data, err := s.backend.GetVal([]string{reverseTunnelsPrefix}, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := json.Unmarshal(data, &tunnels[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// sorting helps with tests and makes it all deterministic
	sort.Sort(sortedReverseTunnels(tunnels))
	return tunnels, nil
}

// DeleteReverseTunnel deletes reverse tunnel by it's domain name
func (s *PresenceService) DeleteReverseTunnel(domainName string) error {
	err := s.backend.DeleteKey([]string{reverseTunnelsPrefix}, domainName)
	return trace.Wrap(err)
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

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel struct {
	// DomainName is a domain name of remote cluster we are connecting to
	DomainName string `json:"domain_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs"`
}

// Check returns nil if all parameters are good, error otherwise
func (r *ReverseTunnel) Check() error {
	if !cstrings.IsValidDomainName(r.DomainName) {
		return trace.Wrap(teleport.BadParameter("DomainName",
			fmt.Sprintf("'%v' is a bad domain name", r.DomainName)))
	}

	if len(r.DialAddrs) == 0 {
		return trace.Wrap(teleport.BadParameter("DialAddrs",
			fmt.Sprintf("'%v' is a bad domain name", r.DialAddrs)))
	}

	for _, addr := range r.DialAddrs {
		_, err := utils.ParseAddr(addr)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CommandLabel is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabel struct {
	// Period is a time between command runs
	Period time.Duration `json:"period"`
	// Command is a command to run
	Command []string `json:"command"` //["/usr/bin/hostname", "--long"]
	// Result captures standard output
	Result string `json:"result"`
}

// CommandLabels is a set of command labels
type CommandLabels map[string]CommandLabel

// SetEnv sets the value of the label from environment variable
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

const (
	reverseTunnelsPrefix = "reverseTunnels"
	nodesPrefix          = "nodes"
	authServersPrefix    = "authservers"
	proxiesPrefix        = "proxies"
)

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

type sortedReverseTunnels []ReverseTunnel

func (s sortedReverseTunnels) Len() int {
	return len(s)
}

func (s sortedReverseTunnels) Less(i, j int) bool {
	return s[i].DomainName < s[j].DomainName
}

func (s sortedReverseTunnels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
