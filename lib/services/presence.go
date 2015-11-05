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
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/backend"
)

type PresenceService struct {
	backend backend.Backend
}

func NewPresenceService(backend backend.Backend) *PresenceService {
	return &PresenceService{backend}
}

// GetServers returns a list of registered servers
func (s *PresenceService) GetServers() ([]Server, error) {
	IDs, err := s.backend.GetKeys([]string{"servers"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]Server, len(IDs))
	for i, id := range IDs {
		data, err := s.backend.GetVal([]string{"servers"}, id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := json.Unmarshal(data, &servers[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return servers, nil
}

// UpsertServer registers server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertServer(server Server, ttl time.Duration) error {
	data, err := json.Marshal(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"servers"},
		server.ID, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

type Server struct {
	ID       string `json:"id"`
	Addr     string `json:"addr"`
	Hostname string `json:"hostname"`
}
