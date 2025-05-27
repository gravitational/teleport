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

package services

import (
	"context"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
)

// ClusterNameGetter is a service that gets the cluster name from the backend.
type ClusterNameGetter interface {
	// GetClusterName gets types.ClusterName from the backend.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// ClusterConfiguration stores the cluster configuration in the backend. All
// the resources modified by this interface can only have a single instance
// in the backend.
type ClusterConfiguration interface {
	ClusterNameGetter

	// GetStaticTokens gets services.StaticTokens from the backend.
	GetStaticTokens() (types.StaticTokens, error)
	// SetStaticTokens sets services.StaticTokens on the backend.
	SetStaticTokens(types.StaticTokens) error
	// DeleteStaticTokens deletes static tokens resource
	DeleteStaticTokens() error

	// GetUIConfig gets the proxy service UI config from the backend
	GetUIConfig(context.Context) (types.UIConfig, error)
	// SetUIConfig sets the proxy service UI config from the backend
	SetUIConfig(context.Context, types.UIConfig) error
	// DeleteUIConfig deletes the proxy service UI config from the backend
	DeleteUIConfig(ctx context.Context) error

	// GetAuthPreference gets types.AuthPreference from the backend.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	// CreateAuthPreference creates an auth preference if once does not already exist.
	CreateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	// UpdateAuthPreference updates an existing auth preference.
	UpdateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	// UpsertAuthPreference creates a new auth preference or overwrites an existing auth preference.
	UpsertAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	// DeleteAuthPreference deletes types.AuthPreference from the backend.
	DeleteAuthPreference(ctx context.Context) error

	// GetSessionRecordingConfig gets SessionRecordingConfig from the backend.
	GetSessionRecordingConfig(context.Context) (types.SessionRecordingConfig, error)
	// CreateSessionRecordingConfig creates a session recording config if once does not already exist.
	CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	// UpdateSessionRecordingConfig updates an existing session recording config.
	UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	// UpsertSessionRecordingConfig creates a new session recording config or overwrites the existing session recording.
	UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	// DeleteSessionRecordingConfig deletes SessionRecordingConfig from the backend.
	DeleteSessionRecordingConfig(ctx context.Context) error

	// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
	GetClusterAuditConfig(context.Context) (types.ClusterAuditConfig, error)
	// CreateClusterAuditConfig creates a cluster audit config if once does not already exist.
	CreateClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error)
	// UpdateClusterAuditConfig updates an existing cluster audit config.
	UpdateClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error)
	// UpsertClusterAuditConfig creates a new cluster audit config or overwrites the existing cluster audit config.
	UpsertClusterAuditConfig(ctx context.Context, cfg types.ClusterAuditConfig) (types.ClusterAuditConfig, error)
	// SetClusterAuditConfig sets ClusterAuditConfig from the backend.
	SetClusterAuditConfig(context.Context, types.ClusterAuditConfig) error
	// DeleteClusterAuditConfig deletes ClusterAuditConfig from the backend.
	DeleteClusterAuditConfig(ctx context.Context) error

	// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
	GetClusterNetworkingConfig(context.Context) (types.ClusterNetworkingConfig, error)
	// CreateClusterNetworkingConfig creates a cluster networking config if once does not already exist.
	CreateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	// UpdateClusterNetworkingConfig updates an existing cluster networking config.
	UpdateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	// UpsertClusterNetworkingConfig creates a new cluster networking config or overwrites the existing cluster networking config.
	UpsertClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	// DeleteClusterNetworkingConfig deletes ClusterNetworkingConfig from the backend.
	DeleteClusterNetworkingConfig(ctx context.Context) error

	// GetInstallers gets all installer scripts from the backend
	GetInstallers(context.Context) ([]types.Installer, error)
	// GetInstaller gets the installer script from the backend
	GetInstaller(ctx context.Context, name string) (types.Installer, error)
	// SetInstaller sets the installer script in the backend
	SetInstaller(context.Context, types.Installer) error
	// DeleteInstaller removes the installer script from the backend
	DeleteInstaller(ctx context.Context, name string) error
	// DeleteAllInstallers removes all installer script resources from the backend
	DeleteAllInstallers(context.Context) error

	// GetClusterMaintenanceConfig loads the current maintenance config singleton.
	GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error)
	// UpdateClusterMaintenanceConfig updates the maintenance config singleton.
	UpdateClusterMaintenanceConfig(ctx context.Context, cfg types.ClusterMaintenanceConfig) error
	// DeleteClusterMaintenanceConfig deletes the maintenance config singleton.
	DeleteClusterMaintenanceConfig(ctx context.Context) error

	// GetAccessGraphSettings gets the access graph settings from the backend.
	GetAccessGraphSettings(context.Context) (*clusterconfigpb.AccessGraphSettings, error)
	// CreateAccessGraphSettings creates the access graph settings in the backend.
	CreateAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
	// UpdateAccessGraphSettings updates the access graph settings in the backend.
	UpdateAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
	// UpsertAccessGraphSettings creates or updates the access graph settings in the backend.
	UpsertAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
	// DeleteAccessGraphSettings deletes the access graph settings from the backend.
	DeleteAccessGraphSettings(context.Context) error
}

// ClusterConfigurationInternal extends [ClusterConfiguration] with
// auth-specific methods.
type ClusterConfigurationInternal interface {
	ClusterConfiguration

	// SetClusterName sets services.ClusterName on the backend.
	SetClusterName(types.ClusterName) error
	// UpsertClusterName upserts cluster name
	UpsertClusterName(types.ClusterName) error
	// DeleteClusterName deletes cluster name resource
	DeleteClusterName() error

	// AppendCheckAuthPreferenceActions appends some atomic write actions to the
	// given slice that will check that the currently stored cluster auth
	// preference has the given revision when applied as part of a
	// [backend.Backend.AtomicWrite]. The backend to which the actions are
	// applied should be the same backend used by the
	// ClusterConfigurationInternal.
	AppendCheckAuthPreferenceActions(actions []backend.ConditionalAction, revision string) ([]backend.ConditionalAction, error)
}

// ValidateAuthPreference performs checks that should happen before persisting a
// new version of the preference resource, typically only as part of Auth
// service operations.
func ValidateAuthPreference(ap types.AuthPreference) error {
	// TODO(espadolini): the checks that are duplicated in
	// {Set,Create,Update,Upsert}AuthPreference should be moved here
	if err := modules.ValidateResource(ap); err != nil {
		return trace.Wrap(err)
	}

	if err := ValidateStableUNIXUserConfig(ap.GetStableUNIXUserConfig()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ValidateStableUNIXUserConfig checks if the configuration is suitable for
// storage and use.
func ValidateStableUNIXUserConfig(c *types.StableUNIXUserConfig) error {
	if c == nil || !c.Enabled {
		return nil
	}

	if c.FirstUid > c.LastUid {
		return trace.BadParameter("stable UNIX user is enabled but UID range is empty")
	}

	// see https://github.com/systemd/systemd/blob/cc7300fc5868f6d47f3f47076100b574bf54e58d/docs/UIDS-GIDS.md
	const firstUserUID = 1000
	if c.FirstUid < firstUserUID {
		return trace.BadParameter("stable UNIX user UID range includes negative or system UIDs; the configured range should be contained between 1000 and 2147483647")
	}

	return nil
}
