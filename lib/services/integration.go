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
	"iter"

	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

// Integrations defines an interface for managing Integrations.
type Integrations interface {
	IntegrationsGetter
	// CreateIntegration creates a new integration resource.
	CreateIntegration(context.Context, types.Integration) (types.Integration, error)
	// UpdateIntegration updates an existing integration resource.
	UpdateIntegration(context.Context, types.Integration) (types.Integration, error)
	// DeleteIntegration removes the specified scoped or unscoped integration resource.
	DeleteIntegration(ctx context.Context, req *integrationpb.DeleteIntegrationRequest) error
	// DeleteAllIntegrations removes all integrations.
	DeleteAllIntegrations(context.Context) error
}

// IntegrationsGetter defines methods for List/Read operations on Integration Resources.
type IntegrationsGetter interface {
	// GetIntegration returns the specified scoped or unscoped integration resource.
	GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (types.Integration, error)
	// ListIntegrations returns a page of integration resources respecting scope filters.
	ListIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) ([]types.Integration, string, error)
	// RangeIntegrations returns integrations within the range [start, end) respecting scope filters.
	RangeIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) iter.Seq2[types.Integration, error]
}

// IntegrationsTokenGenerator defines methods to generate tokens for Integrations.
type IntegrationsTokenGenerator interface {
	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action
	// with the scoped or unscoped integration named in the request.
	GenerateAWSOIDCToken(ctx context.Context, req *integrationpb.GenerateAWSOIDCTokenRequest) (string, error)
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

// GetCursorForIntegration returns the resource cursor identifying an integration in the logical
// resource stream: "<name>" for unscoped integrations and "~scoped/<encoded-scope>/<name>"
// for scoped integrations.
func GetCursorForIntegration(integration types.Integration) string {
	return scopes.MakeResourceCursor(integration.GetScope(), integration.GetName())
}
