/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// ClusterAutoUpdateService is responsible for managing autoupdate configuration and version management.
type ClusterAutoUpdateService struct {
	backend.Backend
}

// NewClusterAutoUpdateService returns a new AutoUpdateService.
func NewClusterAutoUpdateService(backend backend.Backend) *ClusterAutoUpdateService {
	return &ClusterAutoUpdateService{
		Backend: backend,
	}
}

// UpsertClusterAutoUpdateConfig sets cluster autoupdate configuration.
func (s *ClusterAutoUpdateService) UpsertClusterAutoUpdateConfig(ctx context.Context, c types.ClusterAutoUpdateConfig) error {
	rev := c.GetRevision()
	value, err := services.MarshalClusterAutoUpdateConfig(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, backend.Item{
		Key:      backend.Key(clusterAutoupdatePrefix, clusterAutoupdateConfigPrefix),
		Value:    value,
		Expires:  c.Expiry(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetClusterAutoUpdateConfig gets the autoupdate configuration from the backend.
func (s *ClusterAutoUpdateService) GetClusterAutoUpdateConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAutoUpdateConfig, error) {
	item, err := s.Get(ctx, backend.Key(clusterAutoupdatePrefix, clusterAutoupdateConfigPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster autoupdate configuration not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalClusterAutoUpdateConfig(item.Value,
		services.AddOptions(opts, services.WithRevision(item.Revision))...)
}

// DeleteClusterAutoUpdateConfig deletes types.ClusterAutoUpdateConfig from the backend.
func (s *ClusterAutoUpdateService) DeleteClusterAutoUpdateConfig(ctx context.Context) error {
	err := s.Delete(ctx, backend.Key(clusterAutoupdatePrefix, clusterAutoupdateConfigPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cluster autoupdate configuration not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpsertAutoUpdateVersion sets cluster autoupdate version resource.
func (s *ClusterAutoUpdateService) UpsertAutoUpdateVersion(ctx context.Context, c types.AutoUpdateVersion) error {
	rev := c.GetRevision()
	value, err := services.MarshalAutoUpdateVersion(c)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, backend.Item{
		Key:      backend.Key(clusterAutoupdatePrefix, clusterAutoupdateVersionPrefix),
		Value:    value,
		Expires:  c.Expiry(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetAutoUpdateVersion gets the autoupdate version from the backend.
func (s *ClusterAutoUpdateService) GetAutoUpdateVersion(ctx context.Context, opts ...services.MarshalOption) (types.AutoUpdateVersion, error) {
	item, err := s.Get(ctx, backend.Key(clusterAutoupdatePrefix, clusterAutoupdateVersionPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("autoupdate version not found")
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalAutoUpdateVersion(item.Value,
		services.AddOptions(opts, services.WithRevision(item.Revision))...)
}

// DeleteAutoUpdateVersion deletes types.AutoUpdateVersion from the backend.
func (s *ClusterAutoUpdateService) DeleteAutoUpdateVersion(ctx context.Context) error {
	err := s.Delete(ctx, backend.Key(clusterAutoupdatePrefix, clusterAutoupdateVersionPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("autoupdate version not found")
		}
		return trace.Wrap(err)
	}
	return nil
}

const (
	clusterAutoupdatePrefix        = "cluster_autoupdate"
	clusterAutoupdateConfigPrefix  = "config"
	clusterAutoupdateVersionPrefix = "version"
)
