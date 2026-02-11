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

package spacelift

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/joinutils"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/modules"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "spacelift")

type Validator interface {
	Validate(
		ctx context.Context, domain string, token string,
	) (*IDTokenClaims, error)
}

// IDTokenClaims
// See the following for the structure:
// https://docs.spacelift.io/integrations/cloud-providers/oidc/#standard-claims
type IDTokenClaims struct {
	oidc.TokenClaims

	// Sub provides some information about the Spacelift run that generated this
	// token.
	// space:<space_id>:(stack|module):<stack_id|module_id>:run_type:<run_type>:scope:<read|write>
	Sub string `json:"sub"`
	// SpaceID is the ID of the space in which the run that owns the token was
	// executed.
	SpaceID string `json:"spaceId"`
	// CallerType is the type of the caller, ie. the entity that owns the run -
	// either stack or module.
	CallerType string `json:"callerType"`
	// CallerID is the ID of the caller, ie. the stack or module that generated
	// the run.
	CallerID string `json:"callerId"`
	// RunType is the type of the run.
	// (PROPOSED, TRACKED, TASK, TESTING or DESTROY)
	RunType string `json:"runType"`
	// RunID is the ID of the run that owns the token.
	RunID string `json:"runId"`
	// Scope is the scope of the token - either read or write.
	Scope string `json:"scope"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsSpacelift {
	return &workloadidentityv1pb.JoinAttrsSpacelift{
		Sub:        c.Sub,
		SpaceId:    c.SpaceID,
		CallerType: c.CallerType,
		CallerId:   c.CallerID,
		RunType:    c.RunType,
		RunId:      c.RunID,
		Scope:      c.Scope,
	}
}

// CheckIDTokenParams are parameters used to validate Spacelift OIDC tokens.
type CheckIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Validator      Validator
}

func (p *CheckIDTokenParams) validate() error {
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

// CheckIDToken validates a Spacelift OIDC token, verifying both the validity of
// the OIDC token itself, as well as ensuring claims match any configured allow
// rules in the provided provision token.
func CheckIDToken(
	ctx context.Context,
	params *CheckIDTokenParams,
) (*IDTokenClaims, error) {
	if err := params.validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("spacelift join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	if err := modules.GetModules().RequireEnterpriseBuild("spacelift joining"); err != nil {
		return nil, trace.Wrap(err)
	}

	claims, err := params.Validator.Validate(
		ctx, token.Spec.Spacelift.Hostname, string(params.IDToken),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "Spacelift run trying to join cluster",
		"claims", claims,
		"token", token.GetName(),
	)

	return claims, trace.Wrap(checkSpaceliftAllowRules(token, claims))
}

func checkSpaceliftAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
	globCheck := func(want string, got string) (bool, error) {
		if token.Spec.Spacelift.EnableGlobMatching {
			return joinutils.GlobMatchAllowEmptyPattern(want, got)
		}
		if want == "" {
			return true, nil
		}
		return want == got, nil
	}

	// If a single rule passes, accept the IDToken
	for i, rule := range token.Spec.Spacelift.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2Spacelift proto spec.
		spaceIDMatch, err := globCheck(rule.SpaceID, claims.SpaceID)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) space_id glob match", i)
		}
		if !spaceIDMatch {
			continue
		}
		callerIDMatch, err := globCheck(rule.CallerID, claims.CallerID)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) caller_id glob match", i)
		}
		if !callerIDMatch {
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
