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
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/modules"
)

type ghaIDTokenValidator interface {
	Validate(
		ctx context.Context, GHESHost string, enterpriseSlug string, token string,
	) (*githubactions.IDTokenClaims, error)
}

type ghaIDTokenJWKSValidator func(
	now time.Time, jwksData []byte, token string,
) (*githubactions.IDTokenClaims, error)

func (a *Server) checkGitHubJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*githubactions.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for Github join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("github join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	// enterpriseOverride is a hostname to use instead of github.com when
	// validating tokens. This allows GHES instances to be connected.
	enterpriseOverride := token.Spec.GitHub.EnterpriseServerHost
	enterpriseSlug := token.Spec.GitHub.EnterpriseSlug
	if enterpriseOverride != "" || enterpriseSlug != "" {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, fmt.Errorf(
				"github enterprise server joining: %w",
				ErrRequiresEnterprise,
			)
		}
	}

	var claims *githubactions.IDTokenClaims
	if token.Spec.GitHub.StaticJWKS != "" {
		claims, err = a.ghaIDTokenJWKSValidator(
			a.clock.Now().UTC(),
			[]byte(token.Spec.GitHub.StaticJWKS),
			req.IDToken,
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with jwks")
		}
	} else {
		claims, err = a.ghaIDTokenValidator.Validate(
			ctx, enterpriseOverride, enterpriseSlug, req.IDToken,
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with oidc")
		}
	}

	a.logger.InfoContext(ctx, "Github actions run trying to join cluster",
		"claims", claims,
		"token", pt.GetName(),
	)

	return claims, trace.Wrap(checkGithubAllowRules(token, claims))
}

func checkGithubAllowRules(token *types.ProvisionTokenV2, claims *githubactions.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.GitHub.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2Github proto spec.
		if rule.Sub != "" && claims.Sub != rule.Sub {
			continue
		}
		if rule.Repository != "" && claims.Repository != rule.Repository {
			continue
		}
		if rule.RepositoryOwner != "" && claims.RepositoryOwner != rule.RepositoryOwner {
			continue
		}
		if rule.Workflow != "" && claims.Workflow != rule.Workflow {
			continue
		}
		if rule.Environment != "" && claims.Environment != rule.Environment {
			continue
		}
		if rule.Actor != "" && claims.Actor != rule.Actor {
			continue
		}
		if rule.Ref != "" && claims.Ref != rule.Ref {
			continue
		}
		if rule.RefType != "" && claims.RefType != rule.RefType {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
