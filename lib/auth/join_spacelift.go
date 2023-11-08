/*
Copyright 2023 Gravitational, Inc.

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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/spacelift"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type spaceliftIDTokenValidator interface {
	Validate(
		ctx context.Context, domain string, token string,
	) (*spacelift.IDTokenClaims, error)
}

func (a *Server) checkSpaceliftJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*spacelift.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("id_token not provided for spacelift join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("spacelift join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	claims, err := a.spaceliftIDTokenValidator.Validate(
		ctx, token.Spec.Spacelift.Hostname, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  pt.GetName(),
	}).Info("Spacelift run trying to join cluster")

	return claims, trace.Wrap(checkSpaceliftAllowRules(token, claims))
}

func checkSpaceliftAllowRules(token *types.ProvisionTokenV2, claims *spacelift.IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.Spacelift.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2Spacelift proto spec.
		if rule.SpaceID != "" && claims.SpaceID != rule.SpaceID {
			continue
		}
		if rule.CallerID != "" && claims.CallerID != rule.CallerID {
			continue
		}
		if rule.CallerType != "" && claims.CallerType != rule.CallerType {
			continue
		}
		if rule.Scope != "" && claims.Scope != rule.Scope {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
