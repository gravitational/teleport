/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/modules"
)

type ghaIDTokenValidator interface {
	Validate(
		ctx context.Context, GHESHost string, token string,
	) (*githubactions.IDTokenClaims, error)
}

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
	if enterpriseOverride != "" {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, fmt.Errorf(
				"github enterprise server joining: %w",
				ErrRequiresEnterprise,
			)
		}
	}

	claims, err := a.ghaIDTokenValidator.Validate(
		ctx, enterpriseOverride, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  pt.GetName(),
	}).Info("Github actions run trying to join cluster")

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
