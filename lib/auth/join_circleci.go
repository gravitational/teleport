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
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/circleci"
)

func (a *Server) checkCircleCIJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	if req.IDToken == "" {
		return trace.BadParameter("IDToken not provided for %q join request", types.JoinMethodCircleCI)
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return trace.Wrap(err)
	}
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("%q join method only support ProvisionTokenV2, '%T' was provided", types.JoinMethodCircleCI, pt)
	}

	claims, err := a.circleCITokenValidate(
		ctx,
		token.Spec.CircleCI.OrganizationID,
		req.IDToken,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(checkCircleCIAllowRules(token, claims))
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
