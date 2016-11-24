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

package local

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

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

func (s *PresenceService) getServers(prefix string) ([]services.Server, error) {
	keys, err := s.backend.GetKeys([]string{prefix})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]services.Server, len(keys))
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

func (s *PresenceService) upsertServer(prefix string, server services.Server, ttl time.Duration) error {
	data, err := json.Marshal(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{prefix}, server.ID, data, ttl)
	return trace.Wrap(err)
}

// GetNodes returns a list of registered servers
func (s *PresenceService) GetNodes() ([]services.Server, error) {
	return s.getServers(nodesPrefix)
}

// GetUserNodes returns a list of registered servers filtered by user
func (s *PresenceService) GetUserNodes(username string) ([]services.Server, error) {
	nodes, err := s.getServers(nodesPrefix)
	if err != nil {
		return nodes, err
	}
	is := NewIdentityService(s.backend, 10, time.Duration(time.Hour))
	user, err := is.GetUser(username)
	if err == nil {
		userLabels := user.GetNodeLabels()
		if len(userLabels) > 0 {
			newNodes := make([]services.Server, 0)
			for _, node := range nodes {
			LabelLoop:
				for lk, lv := range node.Labels {
					tempLabel := lk + "=" + lv
					for _, userLabel := range userLabels {
						if tempLabel == userLabel {
							newNodes = append(newNodes, node)
							break LabelLoop
						}
					}
				}
			}
			nodes = newNodes
		}
	}

	return nodes, err
}

// UpsertNode registers node presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertNode(server services.Server, ttl time.Duration) error {
	return s.upsertServer(nodesPrefix, server, ttl)
}

// GetAuthServers returns a list of registered servers
func (s *PresenceService) GetAuthServers() ([]services.Server, error) {
	return s.getServers(authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(server services.Server, ttl time.Duration) error {
	return s.upsertServer(authServersPrefix, server, ttl)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertProxy(server services.Server, ttl time.Duration) error {
	return s.upsertServer(proxiesPrefix, server, ttl)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]services.Server, error) {
	return s.getServers(proxiesPrefix)
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (s *PresenceService) UpsertReverseTunnel(tunnel services.ReverseTunnel, ttl time.Duration) error {
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
func (s *PresenceService) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	keys, err := s.backend.GetKeys([]string{reverseTunnelsPrefix})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]services.ReverseTunnel, len(keys))
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

const (
	reverseTunnelsPrefix = "reverseTunnels"
	nodesPrefix          = "nodes"
	authServersPrefix    = "authservers"
	proxiesPrefix        = "proxies"
)

type sortedServers []services.Server

func (s sortedServers) Len() int {
	return len(s)
}

func (s sortedServers) Less(i, j int) bool {
	return s[i].ID < s[j].ID
}

func (s sortedServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type sortedReverseTunnels []services.ReverseTunnel

func (s sortedReverseTunnels) Len() int {
	return len(s)
}

func (s sortedReverseTunnels) Less(i, j int) bool {
	return s[i].DomainName < s[j].DomainName
}

func (s sortedReverseTunnels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
