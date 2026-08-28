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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/terraformcloud"
)

// GetTerraformIDTokenValidator returns the server's currently configured
// Terraform Cloud ID token validator
func (a *Server) GetTerraformIDTokenValidator() terraformcloud.Validator {
	return a.terraformIDTokenValidator
}

// SetTerraformIDTokenValidator sets the current Terraform Cloud OIDC token
// validator, used in tests.
func (a *Server) SetTerraformIDTokenValidator(validator terraformcloud.Validator) {
	a.terraformIDTokenValidator = validator
}

func (a *Server) checkTerraformCloudJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*terraformcloud.IDTokenClaims, error) {
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims, err := terraformcloud.CheckIDToken(ctx, &terraformcloud.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        []byte(req.IDToken),
		Validator:      a.terraformIDTokenValidator,
		ClusterName:    clusterName.GetClusterName(),
	})

	// As usual, attempt to return claims regardless of whether or not an error
	// was encountered.
	return claims, trace.Wrap(err)
}
