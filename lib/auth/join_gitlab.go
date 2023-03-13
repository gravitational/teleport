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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/gitlab"
)

type gitlabIDTokenValidator interface {
	Validate(
		ctx context.Context, domain string, token string,
	) (*gitlab.IDTokenClaims, error)
}

func (a *Server) checkGitLabJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*gitlab.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for gitlab join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("gitlab join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	claims, err := a.gitlabIDTokenValidator.Validate(
		ctx, token.Spec.GitLab.Domain, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  pt.GetName(),
	}).Info("GitLab CI run trying to join cluster")

	return claims, trace.Wrap(checkGitLabAllowRules(token, claims))
}

func checkGitLabAllowRules(token *types.ProvisionTokenV2, claims *gitlab.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.GitLab.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2GitLab proto spec.
		if rule.Sub != "" && claims.Sub != rule.Sub {
			continue
		}
		if rule.Ref != "" && claims.Ref != rule.Ref {
			continue
		}
		if rule.RefType != "" && claims.RefType != rule.RefType {
			continue
		}
		if rule.NamespacePath != "" && claims.NamespacePath != rule.NamespacePath {
			continue
		}
		if rule.ProjectPath != "" && claims.ProjectPath != rule.ProjectPath {
			continue
		}
		if rule.PipelineSource != "" && claims.PipelineSource != rule.PipelineSource {
			continue
		}
		if rule.Environment != "" && claims.Environment != rule.Environment {
			continue
		}
		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
