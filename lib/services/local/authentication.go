/*
Copyright 2017 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// ClusterAuthPreferenceService is responsible for managing cluster authentication preferences.
type ClusterAuthPreferenceService struct {
	backend.Backend
}

// NewClusterAuthPreferenceService returns a new ClusterAuthPreferenceService.
func NewClusterAuthPreferenceService(backend backend.Backend) *ClusterAuthPreferenceService {
	return &ClusterAuthPreferenceService{
		Backend: backend,
	}
}

// GetClusterAuthPreference fetches the cluster authentication preferences
// from the backend and return them.
func (s *ClusterAuthPreferenceService) GetClusterAuthPreference() (services.AuthPreference, error) {
	data, err := s.GetVal([]string{"authentication", "preference"}, "general")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authentication preference not found")
		}
		return nil, trace.Wrap(err)
	}

	return services.GetAuthPreferenceMarshaler().Unmarshal(data)
}

// SetClusterAuthPreference sets the cluster authentication preferences
// on the backend.
func (s *ClusterAuthPreferenceService) SetClusterAuthPreference(preferences services.AuthPreference) error {
	data, err := services.GetAuthPreferenceMarshaler().Marshal(preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"authentication", "preference"}, "general", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
