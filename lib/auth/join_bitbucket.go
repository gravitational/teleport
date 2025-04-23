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
	"github.com/gravitational/teleport/lib/bitbucket"
)

type bitbucketIDTokenValidator interface {
	Validate(
		ctx context.Context, idpURL, audience, token string,
	) (*bitbucket.IDTokenClaims, error)
}

func (a *Server) checkBitbucketJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*bitbucket.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("id_token not provided for bitbucket join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("bitbucket join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	claims, err := a.bitbucketIDTokenValidator.Validate(
		ctx, token.Spec.Bitbucket.IdentityProviderURL, token.Spec.Bitbucket.Audience, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.logger.InfoContext(ctx, "Bitbucket run trying to join cluster", "claims", claims, "token", pt.GetName())

	return claims, trace.Wrap(checkBitbucketAllowRules(token, claims))
}

func checkBitbucketAllowRules(token *types.ProvisionTokenV2, claims *bitbucket.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.Bitbucket.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2Bitbucket proto spec.

		if rule.WorkspaceUUID != "" && claims.WorkspaceUUID != rule.WorkspaceUUID {
			continue
		}

		if rule.RepositoryUUID != "" && claims.RepositoryUUID != rule.RepositoryUUID {
			continue
		}

		if rule.DeploymentEnvironmentUUID != "" && claims.DeploymentEnvironmentUUID != rule.DeploymentEnvironmentUUID {
			continue
		}

		if rule.BranchName != "" && claims.BranchName != rule.BranchName {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
