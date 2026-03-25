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

package githubactions

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "githubactions")

// GitHub Workload Identity
//
// GH provides workloads with two environment variables to facilitate fetching
// a ID token for that workload.
//
// ACTIONS_ID_TOKEN_REQUEST_TOKEN: A token that can be redeemed against the
// identity service for an ID token.
// ACTIONS_ID_TOKEN_REQUEST_URL: Indicates the URL of the identity service.
//
// To redeem the request token for an ID token, a GET request shall be made
// to the specified URL with the specified token provided as a Bearer token
// using the Authorization header.
//
// The `audience` query parameter can be used to customize the audience claim
// within the resulting ID token.
//
// Valuable reference:
// - https://github.com/actions/toolkit/blob/main/packages/core/src/oidc-utils.ts
// - https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers
//
// The GitHub Issuer's well-known OIDC document is at
// https://token.actions.githubusercontent.com/.well-known/openid-configuration
// For GitHub Enterprise Servers, this will be at
// https://$HOSTNAME/_services/token/.well-known/openid-configuration

const DefaultIssuerHost = "token.actions.githubusercontent.com"

// IDTokenClaims is the structure of claims contained within a Github issued
// ID token.
//
// See the following for the structure:
// https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect#understanding-the-oidc-token
type IDTokenClaims struct {
	oidc.TokenClaims
	// Sub also known as Subject is a string that roughly uniquely indentifies
	// the workload. The format of this varies depending on the type of
	// github action run.
	Sub string `json:"sub"`
	// The personal account that initiated the workflow run.
	Actor string `json:"actor"`
	// The ID of personal account that initiated the workflow run.
	ActorID string `json:"actor_id"`
	// The target branch of the pull request in a workflow run.
	BaseRef string `json:"base_ref"`
	// The name of the environment used by the job.
	Environment string `json:"environment"`
	// The name of the event that triggered the workflow run.
	EventName string `json:"event_name"`
	// The source branch of the pull request in a workflow run.
	HeadRef string `json:"head_ref"`
	// This is the ref path to the reusable workflow used by this job.
	JobWorkflowRef string `json:"job_workflow_ref"`
	// The git ref that triggered the workflow run.
	Ref string `json:"ref"`
	// The type of ref, for example: "branch".
	RefType string `json:"ref_type"`
	// The visibility of the repository where the workflow is running. Accepts the following values: internal, private, or public.
	RepositoryVisibility string `json:"repository_visibility"`
	// The repository from where the workflow is running.
	// This includes the name of the owner e.g `gravitational/teleport`
	Repository string `json:"repository"`
	// The ID of the repository from where the workflow is running.
	RepositoryID string `json:"repository_id"`
	// The name of the organization in which the repository is stored.
	RepositoryOwner string `json:"repository_owner"`
	// The ID of the organization in which the repository is stored.
	RepositoryOwnerID string `json:"repository_owner_id"`
	// The ID of the workflow run that triggered the workflow.
	RunID string `json:"run_id"`
	// The number of times this workflow has been run.
	RunNumber string `json:"run_number"`
	// The number of times this workflow run has been retried.
	RunAttempt string `json:"run_attempt"`
	// SHA is the commit SHA that triggered the workflow run.
	SHA string `json:"sha"`
	// The name of the workflow.
	Workflow string `json:"workflow"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsGitHub {
	attrs := &workloadidentityv1pb.JoinAttrsGitHub{
		Sub:             c.Sub,
		Actor:           c.Actor,
		Environment:     c.Environment,
		Ref:             c.Ref,
		RefType:         c.RefType,
		Repository:      c.Repository,
		RepositoryOwner: c.RepositoryOwner,
		Workflow:        c.Workflow,
		EventName:       c.EventName,
		Sha:             c.SHA,
		RunId:           c.RunID,
	}

	return attrs
}

// GithubIDTokenValidator is a validator for Github OIDC tokens.
type GithubIDTokenValidator interface {
	Validate(
		ctx context.Context, GHESHost string, enterpriseSlug string, token string,
	) (*IDTokenClaims, error)
}

// GithubIDTokenJWKSValidator defines a validator function that can be used to
// check an OIDC token against a known key.
type GithubIDTokenJWKSValidator func(
	now time.Time, jwksData []byte, token string,
) (*IDTokenClaims, error)

type CheckGithubIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Clock          clockwork.Clock
	Validator      GithubIDTokenValidator
	JWKSValidator  GithubIDTokenJWKSValidator
}

func (p *CheckGithubIDTokenParams) checkAndSetDefaults() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.Validator == nil:
		return trace.BadParameter("Validator is required")
	case p.JWKSValidator == nil:
		return trace.BadParameter("JWKSValidator is required")
	case p.Clock == nil:
		p.Clock = clockwork.NewRealClock()
	}
	return nil
}

// CheckGithubIDToken checks a Github OIDC token against a provision token.
// If the token is valid and its claims match at least one allow rule, the
// claims are returned.
func CheckGithubIDToken(ctx context.Context, params *CheckGithubIDTokenParams) (*IDTokenClaims, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.AccessDenied("%s", err.Error())
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("github join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	// enterpriseOverride is a hostname to use instead of github.com when
	// validating tokens. This allows GHES instances to be connected.
	enterpriseOverride := token.Spec.GitHub.EnterpriseServerHost
	enterpriseSlug := token.Spec.GitHub.EnterpriseSlug
	if enterpriseOverride != "" || enterpriseSlug != "" {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, trace.Wrap(services.ErrRequiresEnterprise, "github enterprise server joining")
		}
	}

	var claims *IDTokenClaims
	var err error
	if token.Spec.GitHub.StaticJWKS != "" {
		claims, err = params.JWKSValidator(
			params.Clock.Now().UTC(),
			[]byte(token.Spec.GitHub.StaticJWKS),
			string(params.IDToken),
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with jwks")
		}
	} else {
		claims, err = params.Validator.Validate(
			ctx, enterpriseOverride, enterpriseSlug, string(params.IDToken),
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with oidc")
		}
	}

	log.InfoContext(ctx, "Github actions run trying to join cluster",
		"claims", claims,
		"token", params.ProvisionToken.GetName(),
	)

	return claims, trace.Wrap(checkGithubAllowRules(token, claims))
}

func checkGithubAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
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
