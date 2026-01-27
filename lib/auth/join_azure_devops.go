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
	"github.com/gravitational/teleport/lib/join/azuredevops"
)

func (a *Server) checkAzureDevopsJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*azuredevops.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for %q join request", types.JoinMethodAzureDevops)
	}
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("%q join method only support ProvisionTokenV2, '%T' was provided", types.JoinMethodAzureDevops, pt)
	}

	claims, err := azuredevops.CheckIDToken(ctx, &azuredevops.CheckIDTokenParams{
		ProvisionToken: token,
		IDToken:        req.IDToken,
		Validator:      a.azureDevopsIDTokenValidator,
	})
	return claims, trace.Wrap(err)
}

// GetAzureDevopsIDTokenValidator returns the currently configured token validator
// for Azure Devops.
func (a *Server) GetAzureDevopsIDTokenValidator() azuredevops.Validator {
	return a.azureDevopsIDTokenValidator
}

// SetAzureDevopsIDTokenValidator sets the current token validator for Azure
// Devops.
func (a *Server) SetAzureDevopsIDTokenValidator(validator azuredevops.Validator) {
	a.azureDevopsIDTokenValidator = validator
}
