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
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/gitlab"
)

type gitlabIDTokenValidator interface {
	Validate(
		ctx context.Context, domain string, token string,
	) (*gitlab.IDTokenClaims, error)
	ValidateTokenWithJWKS(
		ctx context.Context, jwks []byte, token string,
	) (*gitlab.IDTokenClaims, error)
}

func (a *Server) checkGitLabJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	pt types.ProvisionToken,
) (*gitlab.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for gitlab join request")
	}
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("gitlab join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	var claims *gitlab.IDTokenClaims
	var err error
	if token.Spec.GitLab.StaticJWKS != "" {
		claims, err = a.gitlabIDTokenValidator.ValidateTokenWithJWKS(
			ctx, []byte(token.Spec.GitLab.StaticJWKS), req.IDToken,
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with static jwks")
		}
	} else {
		claims, err = a.gitlabIDTokenValidator.Validate(
			ctx, token.Spec.GitLab.Domain, req.IDToken,
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with oidc")
		}
	}

	a.logger.InfoContext(ctx, "GitLab CI run trying to join cluster",
		"claims", claims,
		"token", pt.GetName(),
	)

	return claims, trace.Wrap(checkGitLabAllowRules(token, claims))
}

// joinRuleGlobMatch is used when comparing some rule fields from a
// ProvisionToken  against a claim from a token. It allows simple pattern
// matching:
// - '*' matches zero or more characters.
// - '?' matches any single character.
// It returns true if a match is detected.
func joinRuleGlobMatch(want string, got string) (bool, error) {
	if want == "" {
		return true, nil
	}
	return globMatch(want, got)
}

// globMatch performs simple a simple glob-style match test on a string.
// - '*' matches zero or more characters.
// - '?' matches any single character.
// It returns true if a match is detected.
func globMatch(pattern, str string) (bool, error) {
	pattern = regexp.QuoteMeta(pattern)
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\?`, ".")
	pattern = "^" + pattern + "$"
	matched, err := regexp.MatchString(pattern, str)
	return matched, trace.Wrap(err)
}

func checkGitLabAllowRules(token *types.ProvisionTokenV2, claims *gitlab.IDTokenClaims) error {
	// Helper for comparing a BoolOption with GitLabs string bool.
	// Returns true if OK - returns false if not OK
	boolEqual := func(want *types.BoolOption, got string) bool {
		if want == nil {
			return true
		}
		return (want.Value && got == "true") || (!want.Value && got == "false")
	}

	// If a single rule passes, accept the IDToken
	for i, rule := range token.Spec.GitLab.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2GitLab proto spec.
		subMatches, err := joinRuleGlobMatch(rule.Sub, claims.Sub)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) sub match", i)
		}
		if !subMatches {
			continue
		}
		refMatches, err := joinRuleGlobMatch(rule.Ref, claims.Ref)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) ref match", i)
		}
		if !refMatches {
			continue
		}
		if rule.RefType != "" && claims.RefType != rule.RefType {
			continue
		}
		namespacePathMatches, err := joinRuleGlobMatch(rule.NamespacePath, claims.NamespacePath)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) namespace_path match", i)
		}
		if !namespacePathMatches {
			continue
		}
		projectPathMatches, err := joinRuleGlobMatch(rule.ProjectPath, claims.ProjectPath)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) project_path match", i)
		}
		if !projectPathMatches {
			continue
		}
		if rule.PipelineSource != "" && claims.PipelineSource != rule.PipelineSource {
			continue
		}
		if rule.Environment != "" && claims.Environment != rule.Environment {
			continue
		}
		if rule.UserLogin != "" && claims.UserLogin != rule.UserLogin {
			continue
		}
		if rule.UserID != "" && claims.UserID != rule.UserID {
			continue
		}
		if rule.UserEmail != "" && claims.UserEmail != rule.UserEmail {
			continue
		}
		if !boolEqual(rule.RefProtected, claims.RefProtected) {
			continue
		}
		if !boolEqual(rule.EnvironmentProtected, claims.EnvironmentProtected) {
			continue
		}
		if rule.CIConfigSHA != "" && claims.CIConfigSHA != rule.CIConfigSHA {
			continue
		}
		if rule.CIConfigRefURI != "" && claims.CIConfigRefURI != rule.CIConfigRefURI {
			continue
		}
		if rule.DeploymentTier != "" && claims.DeploymentTier != rule.DeploymentTier {
			continue
		}
		if rule.ProjectVisibility != "" && claims.ProjectVisibility != rule.ProjectVisibility {
			continue
		}
		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
