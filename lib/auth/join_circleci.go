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

package auth

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/circleci"
)

func (a *Server) checkCircleCIJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*circleci.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for %q join request", types.JoinMethodCircleCI)
	}
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("%q join method only support ProvisionTokenV2, '%T' was provided", types.JoinMethodCircleCI, pt)
	}

	claims, err := a.circleCITokenValidate(
		ctx,
		token.Spec.CircleCI.OrganizationID,
		req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, trace.Wrap(checkCircleCIAllowRules(token, claims))
}

func checkCircleCIAllowRules(token *types.ProvisionTokenV2, claims *circleci.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.CircleCI.Allow {
		if rule.ProjectID != "" && claims.ProjectID != rule.ProjectID {
			continue
		}

		// If ContextID is specified in rule, it must be contained in the slice
		// of ContextIDs within the claims.
		if rule.ContextID != "" && !slices.Contains(claims.ContextIDs, rule.ContextID) {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
