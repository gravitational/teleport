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

// CircleCI workload identity
//
// Key values:
// Issuer of tokens: `https://oidc.circleci.com/org/ORGANIZATION_ID`
// Audience: `ORGANIZATION_ID`
//
// `iat` and `exp` will be included and should be respected.
//
// Useful references:
// - https://circleci.com/docs/openid-connect-tokens/

package circleci

import (
	"context"
	"net/url"
	"slices"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "circleci")

// Validator is a function that can be used to validate CircleCI tokens
type Validator func(ctx context.Context, issuerURL, organizationID, token string) (*IDTokenClaims, error)

// issuerURL generates a CircleCI issuer URL such that it avoids potential SSRF
// from the user-provided organization ID.
func issuerURL(organizationID string) string {
	u := url.URL{
		Scheme: "https",
		Host:   "oidc.circleci.com",
	}

	return u.JoinPath("org", organizationID).String()
}

// IDTokenClaims is the structure of claims contained with a CircleCI issued
// ID token.
// See https://circleci.com/docs/openid-connect-tokens/
type IDTokenClaims struct {
	oidc.TokenClaims

	// Sub identifies who is running the CircleCI job and where.
	// In the format of: `org/ORGANIZATION_ID/project/PROJECT_ID/user/USER_ID`
	Sub string `json:"sub"`
	// ContextIDs is a list of UUIDs for the contexts used in the job.
	ContextIDs []string `json:"oidc.circleci.com/context-ids"`
	// ProjectID is the ID of the project in which the job is running.
	ProjectID string `json:"oidc.circleci.com/project-id"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsCircleCI {
	return &workloadidentityv1pb.JoinAttrsCircleCI{
		Sub:        c.Sub,
		ContextIds: c.ContextIDs,
		ProjectId:  c.ProjectID,
	}
}

// CheckIDTokenParams are parameters used to validate CircleCI OIDC tokens.
type CheckIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Validator      Validator
}

func (p *CheckIDTokenParams) checkAndSetDefaults() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.Validator == nil:
		return trace.BadParameter("Validator is required")
	}
	return nil
}

// CheckIDToken validates a CircleCI OIDC token, verifying both the validity of
// the OIDC token itself, as well as ensuring claims match any configured allow
// rules in the provided provision token.
func CheckIDToken(
	ctx context.Context,
	params *CheckIDTokenParams,
) (*IDTokenClaims, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.AccessDenied("%s", err.Error())
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("circleci join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	claims, err := params.Validator(
		ctx,
		issuerURL(token.Spec.CircleCI.OrganizationID),
		token.Spec.CircleCI.OrganizationID,
		string(params.IDToken),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "CircleCI run trying to join cluster",
		"claims", claims,
		"token", params.ProvisionToken.GetName(),
	)

	return claims, trace.Wrap(checkCircleCIAllowRules(token, claims))
}

func checkCircleCIAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
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
