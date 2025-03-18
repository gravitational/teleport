/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
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

var _ services.ClusterConfigurationInternal = (*ClusterConfigurationService)(nil)

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
func (s *ClusterConfigurationService) GetClusterName(ctx context.Context) (types.ClusterName, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, namePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			clusterNameNotFound.Inc()
			return nil, trace.NotFound("cluster name not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterName(item.Value, services.WithRevision(item.Revision))
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
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// CreateAuthPreference creates an auth preference if once does not already exist.
func (s *ClusterConfigurationService) CreateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(preference); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalAuthPreference(preference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Value: value,
	}

	lease, err := s.Backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preference.SetRevision(lease.Revision)
	return preference, nil
}

// UpdateAuthPreference updates an existing auth preference.
func (s *ClusterConfigurationService) UpdateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	// Perform the modules-provided checks.
	rev := preference.GetRevision()
	if err := modules.ValidateResource(preference); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalAuthPreference(preference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preference.SetRevision(lease.Revision)
	return preference, nil
}

// UpsertAuthPreference creates a new auth preference or overwrites an existing auth preference.
func (s *ClusterConfigurationService) UpsertAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(preference); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := preference.GetRevision()
	value, err := services.MarshalAuthPreference(preference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preference.SetRevision(lease.Revision)
	return preference, nil
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

// AppendCheckAuthPreferenceActions implements [services.ClusterConfigurationInternal].
func (s *ClusterConfigurationService) AppendCheckAuthPreferenceActions(actions []backend.ConditionalAction, revision string) ([]backend.ConditionalAction, error) {
	return append(actions, backend.ConditionalAction{
		Key:       backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Condition: backend.Revision(revision),
		Action:    backend.Nop(),
	}), nil
}

// GetClusterAuditConfig gets cluster audit config from the backend.
func (s *ClusterConfigurationService) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, auditPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster audit config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterAuditConfig(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// SetClusterAuditConfig sets the cluster audit config on the backend.
func (s *ClusterConfigurationService) SetClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) error {
	_, err := s.UpsertClusterAuditConfig(ctx, cfg)
	return trace.Wrap(err)
}

// CreateClusterAuditConfig creates a cluster audit config if once does not already exist.
func (s *ClusterConfigurationService) CreateClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	value, err := services.MarshalClusterAuditConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(clusterConfigPrefix, auditPrefix),
		Value: value,
	}

	lease, err := s.Backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpdateClusterAuditConfig updates an existing cluster audit config.
func (s *ClusterConfigurationService) UpdateClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	rev := cfg.GetRevision()
	value, err := services.MarshalClusterAuditConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, auditPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpsertClusterAuditConfig creates a new cluster audit config or overwrites the existing cluster audit config.
func (s *ClusterConfigurationService) UpsertClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	value, err := services.MarshalClusterAuditConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(clusterConfigPrefix, auditPrefix),
		Value: value,
	}

	lease, err := s.Backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
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
func (s *ClusterConfigurationService) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, networkingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster networking config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterNetworkingConfig(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// CreateClusterNetworkingConfig creates a cluster networking config if once does not already exist.
func (s *ClusterConfigurationService) CreateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalClusterNetworkingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(clusterConfigPrefix, networkingPrefix),
		Value: value,
	}

	lease, err := s.Backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpdateClusterNetworkingConfig updates an existing cluster networking config.
func (s *ClusterConfigurationService) UpdateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := cfg.GetRevision()
	value, err := services.MarshalClusterNetworkingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, networkingPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpsertClusterNetworkingConfig creates a new cluster networking config or overwrites the existing cluster networking config.
func (s *ClusterConfigurationService) UpsertClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := cfg.GetRevision()
	value, err := services.MarshalClusterNetworkingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, networkingPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
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
func (s *ClusterConfigurationService) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session recording config not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalSessionRecordingConfig(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// CreateSessionRecordingConfig creates a session recording config if once does not already exist.
func (s *ClusterConfigurationService) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalSessionRecordingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix),
		Value: value,
	}

	lease, err := s.Backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpdateSessionRecordingConfig updates an existing session recording config.
func (s *ClusterConfigurationService) UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := cfg.GetRevision()
	value, err := services.MarshalSessionRecordingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
}

// UpsertSessionRecordingConfig creates a new session recording or overwrites the existing session recording.
func (s *ClusterConfigurationService) UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	// Perform the modules-provided checks.
	if err := modules.ValidateResource(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := cfg.GetRevision()
	value, err := services.MarshalSessionRecordingConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.SetRevision(lease.Revision)
	return cfg, nil
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

// GetAccessGraphSettings fetches the cluster *clusterconfigpb.AccessGraphSettings from the backend and return them.
func (s *ClusterConfigurationService) GetAccessGraphSettings(ctx context.Context) (*clusterconfigpb.AccessGraphSettings, error) {
	item, err := s.Get(ctx, backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("AccessGraphSettings preference not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalAccessGraphSettings(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// CreateAccessGraphSettings creates an *clusterconfigpb.AccessGraphSettings if it does not already exist.
func (s *ClusterConfigurationService) CreateAccessGraphSettings(ctx context.Context, set *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error) {
	value, err := services.MarshalAccessGraphSettings(set)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix),
		Value: value,
	}

	lease, err := s.Backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	set.Metadata.Revision = lease.Revision
	return set, nil
}

// UpdateAccessGraphSettings updates an existing *clusterconfigpb.AccessGraphSettings.
func (s *ClusterConfigurationService) UpdateAccessGraphSettings(ctx context.Context, set *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error) {
	rev := set.GetMetadata().GetRevision()

	value, err := services.MarshalAccessGraphSettings(set)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	set.Metadata.Revision = lease.Revision
	return set, nil
}

// UpsertAccessGraphSettings creates or overwrites an *clusterconfigpb.AccessGraphSettings.
func (s *ClusterConfigurationService) UpsertAccessGraphSettings(ctx context.Context, set *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error) {
	rev := set.GetMetadata().GetRevision()
	value, err := services.MarshalAccessGraphSettings(set)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix),
		Value:    value,
		Revision: rev,
	}

	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	set.Metadata.Revision = lease.Revision
	return set, nil
}

// DeleteAccessGraphSettings deletes *clusterconfigpb.AccessGraphSettings from the backend.
func (s *ClusterConfigurationService) DeleteAccessGraphSettings(ctx context.Context) error {
	err := s.Delete(ctx, backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("access graph settings not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

const (
	clusterConfigPrefix       = "cluster_configuration"
	namePrefix                = "name"
	staticTokensPrefix        = "static_tokens"
	authPrefix                = "authentication"
	preferencePrefix          = "preference"
	generalPrefix             = "general"
	auditPrefix               = "audit"
	networkingPrefix          = "networking"
	sessionRecordingPrefix    = "session_recording"
	scriptsPrefix             = "scripts"
	uiPrefix                  = "ui"
	installerPrefix           = "installer"
	maintenancePrefix         = "maintenance"
	accessGraphSettingsPrefix = "access_graph_settings"
)
