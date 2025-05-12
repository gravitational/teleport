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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/azuredevops"
)

func (a *Server) checkAzureDevopsJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*azuredevops.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for %q join request", types.JoinMethodAzureDevops)
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("%q join method only support ProvisionTokenV2, '%T' was provided", types.JoinMethodAzureDevops, pt)
	}

	return nil, trace.NotImplemented("azuredevops join request not implemented yet")
}
