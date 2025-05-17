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

type azureDevopsIDTokenValidator interface {
	Validate(ctx context.Context, organizationID string, idToken string) (*azuredevops.IDTokenClaims, error)
}

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

	claims, err := a.azureDevopsIDTokenValidator.Validate(
		ctx,
		token.Spec.AzureDevops.OrganizationID,
		req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, trace.Wrap(checkAzureDevopsAllowRules(token, claims))
}

func checkAzureDevopsAllowRules(token *types.ProvisionTokenV2, claims *azuredevops.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.AzureDevops.Allow {
		if rule.Sub != "" && rule.Sub != claims.Sub {
			continue
		}
		if rule.ProjectName != "" && rule.ProjectName != claims.ProjectName {
			continue
		}
		if rule.PipelineName != "" && rule.PipelineName != claims.PipelineName {
			continue
		}
		if rule.ProjectID != "" && claims.ProjectID != rule.ProjectID {
			continue
		}
		if rule.DefinitionID != "" && claims.DefinitionID != rule.DefinitionID {
			continue
		}
		if rule.RepositoryURI != "" && claims.RepositoryURI != rule.RepositoryURI {
			continue
		}
		if rule.RepositoryVersion != "" && claims.RepositoryVersion != rule.RepositoryVersion {
			continue
		}
		if rule.RepositoryRef != "" && claims.RepositoryRef != rule.RepositoryRef {
			continue
		}
		return nil
	}

	return trace.AccessDenied("id token claims failed to match any allow rules")
}
