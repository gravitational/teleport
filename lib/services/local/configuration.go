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

// ClusterConfigurationService is responsible for managing cluster configuration.
type ClusterConfigurationService struct {
	backend.Backend
}

// NewClusterConfigurationService returns a new ClusterConfigurationService.
func NewClusterConfigurationService(backend backend.Backend) *ClusterConfigurationService {
	return &ClusterConfigurationService{
		Backend: backend,
	}
}

// GetClusterName gets the name of the cluster from the backend.
func (s *ClusterConfigurationService) GetClusterName() (services.ClusterName, error) {
	data, err := s.GetVal([]string{"cluster_configuration"}, "name")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster name not found")
		}
		return nil, trace.Wrap(err)
	}

	return services.GetClusterNameMarshaler().Unmarshal(data)
}

// SetClusterName sets the name of the cluster in the backend. SetClusterName
// can only be called once on a cluster after which it will return trace.AlreadyExists.
func (s *ClusterConfigurationService) SetClusterName(c services.ClusterName) error {
	data, err := services.GetClusterNameMarshaler().Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.CreateVal([]string{"cluster_configuration"}, "name", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) GetStaticTokens() (services.StaticTokens, error) {
	data, err := s.GetVal([]string{"cluster_configuration"}, "static_tokens")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("static tokens not found")
		}
		return nil, trace.Wrap(err)
	}

	return services.GetStaticTokensMarshaler().Unmarshal(data)
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) SetStaticTokens(c services.StaticTokens) error {
	data, err := services.GetStaticTokensMarshaler().Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"cluster_configuration"}, "static_tokens", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetAuthPreference fetches the cluster authentication preferences
// from the backend and return them.
func (s *ClusterConfigurationService) GetAuthPreference() (services.AuthPreference, error) {
	data, err := s.GetVal([]string{"authentication", "preference"}, "general")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authentication preference not found")
		}
		return nil, trace.Wrap(err)
	}

	return services.GetAuthPreferenceMarshaler().Unmarshal(data)
}

// SetAuthPreference sets the cluster authentication preferences
// on the backend.
func (s *ClusterConfigurationService) SetAuthPreference(preferences services.AuthPreference) error {
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
