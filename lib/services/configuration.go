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

	"github.com/gravitational/teleport/api/types"
)

// ClusterConfiguration stores the cluster configuration in the backend. All
// the resources modified by this interface can only have a single instance
// in the backend.
type ClusterConfiguration interface {
	// SetClusterName gets services.ClusterName from the backend.
	GetClusterName(opts ...MarshalOption) (types.ClusterName, error)
	// SetClusterName sets services.ClusterName on the backend.
	SetClusterName(types.ClusterName) error
	// UpsertClusterName upserts cluster name
	UpsertClusterName(types.ClusterName) error

	// DeleteClusterName deletes cluster name resource
	DeleteClusterName() error

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
	// SetAuthPreference sets types.AuthPreference from the backend.
	SetAuthPreference(context.Context, types.AuthPreference) error
	// DeleteAuthPreference deletes types.AuthPreference from the backend.
	DeleteAuthPreference(ctx context.Context) error

	// GetSessionRecordingConfig gets SessionRecordingConfig from the backend.
	GetSessionRecordingConfig(context.Context, ...MarshalOption) (types.SessionRecordingConfig, error)
	// SetSessionRecordingConfig sets SessionRecordingConfig from the backend.
	SetSessionRecordingConfig(context.Context, types.SessionRecordingConfig) error
	// DeleteSessionRecordingConfig deletes SessionRecordingConfig from the backend.
	DeleteSessionRecordingConfig(ctx context.Context) error

	// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
	GetClusterAuditConfig(context.Context, ...MarshalOption) (types.ClusterAuditConfig, error)
	// SetClusterAuditConfig sets ClusterAuditConfig from the backend.
	SetClusterAuditConfig(context.Context, types.ClusterAuditConfig) error
	// DeleteClusterAuditConfig deletes ClusterAuditConfig from the backend.
	DeleteClusterAuditConfig(ctx context.Context) error

	// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
	GetClusterNetworkingConfig(context.Context, ...MarshalOption) (types.ClusterNetworkingConfig, error)
	// SetClusterNetworkingConfig sets ClusterNetworkingConfig from the backend.
	SetClusterNetworkingConfig(context.Context, types.ClusterNetworkingConfig) error
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
}
