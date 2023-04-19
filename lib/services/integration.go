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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Integrations defines an interface for managing Integrations.
type Integrations interface {
	IntegrationsGetter
	// CreateIntegration creates a new integration resource.
	CreateIntegration(context.Context, types.Integration) (types.Integration, error)
	// UpdateIntegration updates an existing integration resource.
	UpdateIntegration(context.Context, types.Integration) (types.Integration, error)
	// DeleteIntegration removes the specified integration resource.
	DeleteIntegration(ctx context.Context, name string) error
	// DeleteAllIntegrations removes all integrations.
	DeleteAllIntegrations(context.Context) error
}

// IntegrationsGetter defines methods for List/Read operations on Integration Resources.
type IntegrationsGetter interface {
	// ListIntegrations returns a paginated list of all integration resources.
	ListIntegrations(ctx context.Context, pageSize int, nextToken string) ([]types.Integration, string, error)
	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// MarshalIntegration marshals the Integration resource to JSON.
func MarshalIntegration(ig types.Integration, opts ...MarshalOption) ([]byte, error) {
	if err := ig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch g := ig.(type) {
	case *types.IntegrationV1:
		if !cfg.PreserveResourceID {
			copy := *g
			copy.SetResourceID(0)
			g = &copy
		}
		return utils.FastMarshal(g)
	default:
		return nil, trace.BadParameter("unsupported integration resource %T", g)
	}
}

// UnmarshalIntegration unmarshals Integration resource from JSON.
func UnmarshalIntegration(data []byte, opts ...MarshalOption) (types.Integration, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var ig types.IntegrationV1

	err := utils.FastUnmarshal(data, &ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		ig.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		ig.SetExpiry(cfg.Expires)
	}
	return &ig, nil
}
