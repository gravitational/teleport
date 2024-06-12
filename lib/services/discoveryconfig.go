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

	discoveryconfigclient "github.com/gravitational/teleport/api/client/discoveryconfig"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/utils"
)

var _ DiscoveryConfigs = (*discoveryconfigclient.Client)(nil)

// DiscoveryConfigs defines an interface for managing DiscoveryConfigs.
type DiscoveryConfigs interface {
	DiscoveryConfigsGetter
	// CreateDiscoveryConfig creates a new DiscoveryConfig resource.
	CreateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// UpdateDiscoveryConfig updates an existing DiscoveryConfig resource.
	UpdateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// UpsertDiscoveryConfig upserts a DiscoveryConfig resource.
	UpsertDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
	DeleteDiscoveryConfig(ctx context.Context, name string) error
	// DeleteAllDiscoveryConfigs removes all DiscoveryConfigs.
	DeleteAllDiscoveryConfigs(context.Context) error
}

// DiscoveryConfigWithStatusUpdater defines an interface for managing DiscoveryConfig resources including updating their status.
type DiscoveryConfigWithStatusUpdater interface {
	DiscoveryConfigs
	// UpdateDiscoveryConfigStatus updates the status of the specified DiscoveryConfig resource.
	UpdateDiscoveryConfigStatus(context.Context, string, discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error)
}

// DiscoveryConfigsGetter defines methods for List/Read operations on DiscoveryConfig Resources.
type DiscoveryConfigsGetter interface {
	// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
	// An optional DiscoveryGroup can be provided to filter.
	ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
	// GetDiscoveryConfig returns the specified DiscoveryConfig resources.
	GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error)
}

// MarshalDiscoveryConfig marshals the DiscoveryCOnfig resource to JSON.
func MarshalDiscoveryConfig(discoveryConfig *discoveryconfig.DiscoveryConfig, opts ...MarshalOption) ([]byte, error) {
	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *discoveryConfig
		copy.SetResourceID(0)
		copy.SetRevision("")
		discoveryConfig = &copy
	}
	return utils.FastMarshal(discoveryConfig)
}

// UnmarshalDiscoveryConfig unmarshals the DiscoveryConfig resource from JSON.
func UnmarshalDiscoveryConfig(data []byte, opts ...MarshalOption) (*discoveryconfig.DiscoveryConfig, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing discovery config data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var discoveryConfig *discoveryconfig.DiscoveryConfig
	if err := utils.FastUnmarshal(data, &discoveryConfig); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		discoveryConfig.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		discoveryConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		discoveryConfig.SetExpiry(cfg.Expires)
	}
	return discoveryConfig, nil
}
