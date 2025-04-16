/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package integrationv1

import (
	"context"

	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
)

// GenerateAzureOIDCToken generates a token to be used to execute an Azure OIDC Integration action.
func (s *Service) GenerateAzureOIDCToken(ctx context.Context, req *integrationpb.GenerateAzureOIDCTokenRequest) (*integrationpb.GenerateAzureOIDCTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.cache.GetIntegration(ctx, req.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, allowedRole := range []types.SystemRole{types.RoleDiscovery, types.RoleAuth, types.RoleProxy} {
		if authz.HasBuiltinRole(*authCtx, string(allowedRole)) {
			token, err := azureoidc.GenerateEntraOIDCToken(ctx, s.cache, s.keyStoreManager, s.clock)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &integrationpb.GenerateAzureOIDCTokenResponse{Token: token}, nil
		}
	}
	return nil, trace.AccessDenied("token generation is only available to auth, proxy or discovery services")
}
