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
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/prometheus/client_golang/prometheus"
)

var clusterNameNotFound = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricClusterNameNotFound,
		Help: "Number of times a cluster name was not found",
	},
)

// ClusterConfigurationService is responsible for managing cluster configuration.
type ClusterConfigurationService struct {
	backend.Backend
}

// NewClusterConfigurationService returns a new ClusterConfigurationService.
func NewClusterConfigurationService(backend backend.Backend) (*ClusterConfigurationService, error) {
	err := utils.RegisterPrometheusCollectors(clusterNameNotFound)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClusterConfigurationService{
		Backend: backend,
	}, nil
}

// GetClusterName gets the name of the cluster from the backend.
func (s *ClusterConfigurationService) GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error) {
	item, err := s.Get(context.TODO(), backend.Key(clusterConfigPrefix, namePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			clusterNameNotFound.Inc()
			return nil, trace.NotFound("cluster name not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterName(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID))...)
}

// DeleteClusterName deletes services.ClusterName from the backend.
func (s *ClusterConfigurationService) DeleteClusterName() error {
	err := s.Delete(context.TODO(), backend.Key(clusterConfigPrefix, namePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cluster configuration not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// SetClusterName sets the name of the cluster in the backend. SetClusterName
// can only be called once on a cluster after which it will return trace.AlreadyExists.
func (s *ClusterConfigurationService) SetClusterName(c services.ClusterName) error {
	value, err := services.MarshalClusterName(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Create(context.TODO(), backend.Item{
		Key:     backend.Key(clusterConfigPrefix, namePrefix),
		Value:   value,
		Expires: c.Expiry(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpsertClusterName sets the name of the cluster in the backend.
func (s *ClusterConfigurationService) UpsertClusterName(c services.ClusterName) error {
	value, err := services.MarshalClusterName(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(context.TODO(), backend.Item{
		Key:     backend.Key(clusterConfigPrefix, namePrefix),
		Value:   value,
		Expires: c.Expiry(),
		ID:      c.GetResourceID(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) GetStaticTokens() (services.StaticTokens, error) {
	item, err := s.Get(context.TODO(), backend.Key(clusterConfigPrefix, staticTokensPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("static tokens not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalStaticTokens(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) SetStaticTokens(c services.StaticTokens) error {
	value, err := services.MarshalStaticTokens(c)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:     backend.Key(clusterConfigPrefix, staticTokensPrefix),
		Value:   value,
		Expires: c.Expiry(),
		ID:      c.GetResourceID(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteStaticTokens deletes static tokens
func (s *ClusterConfigurationService) DeleteStaticTokens() error {
	err := s.Delete(context.TODO(), backend.Key(clusterConfigPrefix, staticTokensPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("static tokens are not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetAuthPreference fetches the cluster authentication preferences
// from the backend and return them.
func (s *ClusterConfigurationService) GetAuthPreference() (services.AuthPreference, error) {
	item, err := s.Get(context.TODO(), backend.Key(authPrefix, preferencePrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authentication preference not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalAuthPreference(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// SetAuthPreference sets the cluster authentication preferences
// on the backend.
func (s *ClusterConfigurationService) SetAuthPreference(preferences services.AuthPreference) error {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(preferences); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalAuthPreference(preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(authPrefix, preferencePrefix, generalPrefix),
		Value: value,
		ID:    preferences.GetResourceID(),
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteAuthPreference deletes services.AuthPreference from the backend.
func (s *ClusterConfigurationService) DeleteAuthPreference(ctx context.Context) error {
	err := s.Delete(ctx, backend.Key(authPrefix, preferencePrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("auth preference not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterConfig gets services.ClusterConfig from the backend.
func (s *ClusterConfigurationService) GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error) {
	item, err := s.Get(context.TODO(), backend.Key(clusterConfigPrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster configuration not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterConfig(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID),
			services.WithExpires(item.Expires))...)
}

// DeleteClusterConfig deletes services.ClusterConfig from the backend.
func (s *ClusterConfigurationService) DeleteClusterConfig() error {
	err := s.Delete(context.TODO(), backend.Key(clusterConfigPrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cluster configuration not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// SetClusterConfig sets services.ClusterConfig on the backend.
func (s *ClusterConfigurationService) SetClusterConfig(c services.ClusterConfig) error {
	if err := s.syncClusterConfigWithNetworkingConfig(c); err != nil {
		return trace.Wrap(err)
	}
	if err := s.syncClusterConfigWithSessionRecordingConfig(c); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalClusterConfig(c)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(clusterConfigPrefix, generalPrefix),
		Value: value,
		ID:    c.GetResourceID(),
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// syncClusterConfigWithNetworkingConfig updates the given ClusterConfig
// with the networking values that are now stored in ClusterNetworkingConfig,
// or vice versa.  Intended to facilitate backward compatible transition from
// legacy ClusterConfig.  DELETE IN 8.0.0
func (s *ClusterConfigurationService) syncClusterConfigWithNetworkingConfig(clusterConfig services.ClusterConfig) error {
	if clusterConfig.HasNetworkingConfig() {
		netConfig, err := clusterConfig.GetNetworkingConfig()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := s.SetClusterNetworkingConfig(context.TODO(), netConfig); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	netConfig, err := s.GetClusterNetworkingConfig(context.TODO(), services.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := clusterConfig.SetNetworkingConfig(netConfig); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// syncClusterConfigWithSessionRecordingConfig updates the given ClusterConfig
// with the session recording values that are now stored in SessionRecordingConfig,
// or vice versa.  Intended to facilitate backward compatible transition from
// legacy ClusterConfig.  DELETE IN 8.0.0
func (s *ClusterConfigurationService) syncClusterConfigWithSessionRecordingConfig(clusterConfig services.ClusterConfig) error {
	if clusterConfig.HasSessionRecordingConfig() {
		recConfig, err := clusterConfig.GetSessionRecordingConfig()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := s.SetSessionRecordingConfig(context.TODO(), recConfig); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	recConfig, err := s.GetSessionRecordingConfig(context.TODO(), services.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := clusterConfig.SetSessionRecordingConfig(recConfig); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterNetworkingConfig gets cluster networking config from the backend.
func (s *ClusterConfigurationService) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	item, err := s.Get(ctx, backend.Key(clusterConfigPrefix, networkingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster networking config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterNetworkingConfig(item.Value, append(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
}

// SetClusterNetworkingConfig sets the cluster networking config
// on the backend.
func (s *ClusterConfigurationService) SetClusterNetworkingConfig(ctx context.Context, netConfig types.ClusterNetworkingConfig) error {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(netConfig); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalClusterNetworkingConfig(netConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(clusterConfigPrefix, networkingPrefix),
		Value: value,
		ID:    netConfig.GetResourceID(),
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteClusterNetworkingConfig deletes ClusterNetworkingConfig from the backend.
func (s *ClusterConfigurationService) DeleteClusterNetworkingConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.Key(clusterConfigPrefix, networkingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cluster networking config not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetSessionRecordingConfig gets session recording config from the backend.
func (s *ClusterConfigurationService) GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	item, err := s.Get(ctx, backend.Key(clusterConfigPrefix, sessionRecordingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session recording config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalSessionRecordingConfig(item.Value, append(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
}

// SetSessionRecordingConfig sets session recording config on the backend.
func (s *ClusterConfigurationService) SetSessionRecordingConfig(ctx context.Context, recConfig types.SessionRecordingConfig) error {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(recConfig); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalSessionRecordingConfig(recConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(clusterConfigPrefix, sessionRecordingPrefix),
		Value: value,
		ID:    recConfig.GetResourceID(),
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSessionRecordingConfig deletes SessionRecordingConfig from the backend.
func (s *ClusterConfigurationService) DeleteSessionRecordingConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.Key(clusterConfigPrefix, sessionRecordingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("session recording config not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

const (
	clusterConfigPrefix    = "cluster_configuration"
	namePrefix             = "name"
	staticTokensPrefix     = "static_tokens"
	authPrefix             = "authentication"
	preferencePrefix       = "preference"
	generalPrefix          = "general"
	networkingPrefix       = "networking"
	sessionRecordingPrefix = "session_recording"
)
