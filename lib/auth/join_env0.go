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
	"github.com/gravitational/teleport/lib/env0"
)

type env0CloudIDTokenValidator interface {
	Validate(
		ctx context.Context, audience, hostname, token string,
	) (*env0.IDTokenClaims, error)
}

func (a *Server) checkTerraformCloudJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*env0.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("id_token not provided for env0 join request")
	}
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("env0 join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	aud := token.Spec.Env0.Audience
	if aud == "" {
		clusterName, err := a.GetClusterName(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		aud = clusterName.GetClusterName()
	}

	claims, err := a.Env0IDTokenValidator.Validate(
		ctx, aud, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.logger.InfoContext(ctx, "Env0 run trying to join cluster",
		"claims", claims,
		"token", pt.GetName(),
	)

	return claims, trace.Wrap(checkEnv0AllowRules(token, claims))
}

func checkEnv0AllowRules(token *types.ProvisionTokenV2, claims *env0.IDTokenClaims) error {
	for _, rule := range token.Spec.Env0.Allow {
		if rule.OrganizationID != "" && claims.OrganizationID != rule.OrganizationID {
			continue
		}
		if rule.OrganizationName != "" && claims.OrganizationName != rule.OrganizationName {
			continue
		}
		if rule.ProjectID != "" && claims.ProjectID != rule.ProjectID {
			continue
		}
		if rule.ProjectName != "" && claims.ProjectName != rule.ProjectName {
			continue
		}
		if rule.EnvironmentID != "" && claims.EnvironmentID != rule.EnvironmentID {
			continue
		}
		if rule.EnvironmentName != "" && claims.EnvironmentName != rule.EnvironmentName {
			continue
		}
		if rule.RunPhase != "" && claims.RunPhase != rule.RunPhase {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
