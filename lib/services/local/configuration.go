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
	"errors"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
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
	err := metrics.RegisterPrometheusCollectors(clusterNameNotFound)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClusterConfigurationService{
		Backend: backend,
	}, nil
}

// GetClusterName gets the name of the cluster from the backend.
func (s *ClusterConfigurationService) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	item, err := s.Get(context.TODO(), backend.NewKey(clusterConfigPrefix, namePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			clusterNameNotFound.Inc()
			return nil, trace.NotFound("cluster name not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterName(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithRevision(item.Revision))...)
}

// DeleteClusterName deletes types.ClusterName from the backend.
func (s *ClusterConfigurationService) DeleteClusterName() error {
	err := s.Delete(context.TODO(), backend.NewKey(clusterConfigPrefix, namePrefix))
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
func (s *ClusterConfigurationService) SetClusterName(c types.ClusterName) error {
	rev := c.GetRevision()
	value, err := services.MarshalClusterName(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Create(context.TODO(), backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, namePrefix),
		Value:    value,
		Expires:  c.Expiry(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpsertClusterName sets the name of the cluster in the backend.
func (s *ClusterConfigurationService) UpsertClusterName(c types.ClusterName) error {
	rev := c.GetRevision()
	value, err := services.MarshalClusterName(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(context.TODO(), backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, namePrefix),
		Value:    value,
		Expires:  c.Expiry(),
		ID:       c.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) GetStaticTokens() (types.StaticTokens, error) {
	item, err := s.Get(context.TODO(), backend.NewKey(clusterConfigPrefix, staticTokensPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("static tokens not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalStaticTokens(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (s *ClusterConfigurationService) SetStaticTokens(c types.StaticTokens) error {
	rev := c.GetRevision()
	value, err := services.MarshalStaticTokens(c)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, staticTokensPrefix),
		Value:    value,
		Expires:  c.Expiry(),
		ID:       c.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteStaticTokens deletes static tokens
func (s *ClusterConfigurationService) DeleteStaticTokens() error {
	err := s.Delete(context.TODO(), backend.NewKey(clusterConfigPrefix, staticTokensPrefix))
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
func (s *ClusterConfigurationService) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	item, err := s.Get(ctx, backend.NewKey(authPrefix, preferencePrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authentication preference not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalAuthPreference(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// SetAuthPreference sets the cluster authentication preferences
// on the backend.
func (s *ClusterConfigurationService) SetAuthPreference(ctx context.Context, preferences types.AuthPreference) error {
	// Perform the modules-provided checks.
	rev := preferences.GetRevision()
	if err := modules.ValidateResource(preferences); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalAuthPreference(preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Value:    value,
		ID:       preferences.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteAuthPreference deletes types.AuthPreference from the backend.
func (s *ClusterConfigurationService) DeleteAuthPreference(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(authPrefix, preferencePrefix, generalPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("auth preference not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterAuditConfig gets cluster audit config from the backend.
func (s *ClusterConfigurationService) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, auditPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster audit config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterAuditConfig(item.Value, append(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
}

// SetClusterAuditConfig sets the cluster audit config on the backend.
func (s *ClusterConfigurationService) SetClusterAuditConfig(ctx context.Context, auditConfig types.ClusterAuditConfig) error {
	rev := auditConfig.GetRevision()
	value, err := services.MarshalClusterAuditConfig(auditConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, auditPrefix),
		Value:    value,
		ID:       auditConfig.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteClusterAuditConfig deletes ClusterAuditConfig from the backend.
func (s *ClusterConfigurationService) DeleteClusterAuditConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(clusterConfigPrefix, auditPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cluster audit config not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterNetworkingConfig gets cluster networking config from the backend.
func (s *ClusterConfigurationService) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, networkingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster networking config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterNetworkingConfig(item.Value, append(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
}

// SetClusterNetworkingConfig sets the cluster networking config
// on the backend.
func (s *ClusterConfigurationService) SetClusterNetworkingConfig(ctx context.Context, netConfig types.ClusterNetworkingConfig) error {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(netConfig); err != nil {
		return trace.Wrap(err)
	}

	rev := netConfig.GetRevision()
	value, err := services.MarshalClusterNetworkingConfig(netConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, networkingPrefix),
		Value:    value,
		ID:       netConfig.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteClusterNetworkingConfig deletes ClusterNetworkingConfig from the backend.
func (s *ClusterConfigurationService) DeleteClusterNetworkingConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(clusterConfigPrefix, networkingPrefix))
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
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session recording config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalSessionRecordingConfig(item.Value, append(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
}

// SetSessionRecordingConfig sets session recording config on the backend.
func (s *ClusterConfigurationService) SetSessionRecordingConfig(ctx context.Context, recConfig types.SessionRecordingConfig) error {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(recConfig); err != nil {
		return trace.Wrap(err)
	}

	rev := recConfig.GetRevision()
	value, err := services.MarshalSessionRecordingConfig(recConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix),
		Value:    value,
		ID:       recConfig.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSessionRecordingConfig deletes SessionRecordingConfig from the backend.
func (s *ClusterConfigurationService) DeleteSessionRecordingConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("session recording config not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetInstallers retrieves all the install scripts.
func (s *ClusterConfigurationService) GetInstallers(ctx context.Context) ([]types.Installer, error) {
	startKey := backend.ExactKey(clusterConfigPrefix, scriptsPrefix, installerPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var installers []types.Installer
	for _, item := range result.Items {
		installer, err := services.UnmarshalInstaller(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		installers = append(installers, installer)
	}
	return installers, nil
}

func (s *ClusterConfigurationService) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, uiPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalUIConfig(item.Value)
}

func (s *ClusterConfigurationService) SetUIConfig(ctx context.Context, uic types.UIConfig) error {
	rev := uic.GetRevision()
	value, err := services.MarshalUIConfig(uic)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, uiPrefix),
		Value:    value,
		Revision: rev,
	})
	return trace.Wrap(err)
}

func (s *ClusterConfigurationService) DeleteUIConfig(ctx context.Context) error {
	return trace.Wrap(s.Delete(ctx, backend.NewKey(clusterConfigPrefix, uiPrefix)))
}

// GetInstaller gets the script of the cluster from the backend.
func (s *ClusterConfigurationService) GetInstaller(ctx context.Context, name string) (types.Installer, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, scriptsPrefix, installerPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalInstaller(item.Value, services.WithRevision(item.Revision))
}

// SetInstaller sets the script of the cluster in the backend
func (s *ClusterConfigurationService) SetInstaller(ctx context.Context, ins types.Installer) error {
	rev := ins.GetRevision()
	value, err := services.MarshalInstaller(ins)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, scriptsPrefix, installerPrefix, ins.GetName()),
		Value:    value,
		Expires:  ins.Expiry(),
		Revision: rev,
	})
	return trace.Wrap(err)
}

// DeleteInstaller sets the installer script to default script in the backend.
func (s *ClusterConfigurationService) DeleteInstaller(ctx context.Context, name string) error {
	return trace.Wrap(
		s.Delete(ctx, backend.NewKey(clusterConfigPrefix, scriptsPrefix, installerPrefix, name)))
}

// DeleteAllInstallers removes all installer resources.
func (s *ClusterConfigurationService) DeleteAllInstallers(ctx context.Context) error {
	startKey := backend.ExactKey(clusterConfigPrefix, scriptsPrefix, installerPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterMaintenanceConfig loads the maintenance config singleton resource.
func (s *ClusterConfigurationService) GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, maintenancePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no maintenance config has been created")
		}
		return nil, trace.Wrap(err)
	}

	var cmc types.ClusterMaintenanceConfigV1
	if err := utils.FastUnmarshal(item.Value, &cmc); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cmc, nil
}

// UpdateClusterMaintenanceConfig performs a nonce-protected update of the maintenance config singleton resource.
func (s *ClusterConfigurationService) UpdateClusterMaintenanceConfig(ctx context.Context, cmc types.ClusterMaintenanceConfig) error {
	if err := cmc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	err := generic.FastUpdateNonceProtectedResource(
		ctx,
		s.Backend,
		backend.NewKey(clusterConfigPrefix, maintenancePrefix),
		cmc,
	)

	if errors.Is(err, generic.ErrNonceViolation) {
		return trace.CompareFailed("maintenance config was concurrently modified, please re-pull and work from latest state")
	}

	return trace.Wrap(err)
}

// DeleteClusterMaintenanceConfig deletes the maintenance config singleton resource.
func (s *ClusterConfigurationService) DeleteClusterMaintenanceConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(clusterConfigPrefix, maintenancePrefix))
	if err != nil {
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
	auditPrefix            = "audit"
	networkingPrefix       = "networking"
	sessionRecordingPrefix = "session_recording"
	scriptsPrefix          = "scripts"
	uiPrefix               = "ui"
	installerPrefix        = "installer"
	maintenancePrefix      = "maintenance"
)
