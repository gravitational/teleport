/*
Copyright 2023 Gravitational, Inc.

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
	// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
	DeleteDiscoveryConfig(ctx context.Context, name string) error
	// DeleteAllDiscoveryConfigs removes all DiscoveryConfigs.
	DeleteAllDiscoveryConfigs(context.Context) error
}

// DiscoveryConfigsGetter defines methods for List/Read operations on DiscoveryConfig Resources.
type DiscoveryConfigsGetter interface {
	// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
	// An optional DiscoveryGroup can be provided to filter.
	ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
	// GetDiscoveryConfig returns the specified DiscoveryConfig resources.
	GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error)
}

// MarshalDiscoveryConfig marshals the access list resource to JSON.
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
		discoveryConfig = &copy
	}
	return utils.FastMarshal(discoveryConfig)
}

// UnmarshalDiscoveryConfig unmarshals the access list resource from JSON.
func UnmarshalDiscoveryConfig(data []byte, opts ...MarshalOption) (*discoveryconfig.DiscoveryConfig, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
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
	if !cfg.Expires.IsZero() {
		discoveryConfig.SetExpiry(cfg.Expires)
	}
	return discoveryConfig, nil
}
