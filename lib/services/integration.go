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

// IntegrationsTokenGenerator defines methods to generate tokens for Integrations.
type IntegrationsTokenGenerator interface {
	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)
	// GenerateAzureOIDCToken generates a token to be used to execute an Azure OIDC Integration action.
	GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error)
}

// MarshalIntegration marshals the Integration resource to JSON.
func MarshalIntegration(ig types.Integration, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch g := ig.(type) {
	case *types.IntegrationV1:
		if err := g.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, g))
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

	if cfg.Revision != "" {
		ig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		ig.SetExpiry(cfg.Expires)
	}
	return &ig, nil
}
