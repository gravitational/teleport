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

package gitlab

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/joinutils"
	"github.com/gravitational/teleport/lib/join/provision"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "gitlab")

// Validator provides implementations for verifying both standard OIDC and JWKS
// tokens issues from GitLab instances.
type Validator interface {
	Validate(
		ctx context.Context, domain string, token string,
	) (*IDTokenClaims, error)
	ValidateTokenWithJWKS(
		ctx context.Context, jwks []byte, token string,
	) (*IDTokenClaims, error)
}

// GitLab Workload Identity
//
// GL provides workloads with the ID token in an environment variable included
// in the workflow config, with the specified audience:
//
// ```yaml
// job-name:
//  id_tokens:
//    TBOT_GITLAB_JWT:
//      aud: https://teleport.example.com
// ```
//
// We will require the user to configure this to be `TBOT_GITLAB_JWT` and to
// set this value to the name of their Teleport cluster.
//
// Valuable reference:
// - https://docs.gitlab.com/ee/ci/secrets/id_token_authentication.html
// - https://docs.gitlab.com/ee/ci/cloud_services/
// - https://docs.gitlab.com/ee/ci/yaml/index.html#id_tokens
//
// The GitLab issuer's well-known OIDC document is at
// https://gitlab.com/.well-known/openid-configuration
// For GitLab self-hosted servers, this will be at
// https://$HOSTNAME/.well-known/openid-configuration
//
// The minimum supported GitLab version is 15.7, as this is when ID token
// support was introduced.

// IDTokenClaims is the structure of claims contained within a GitLab issued
// ID token.
//
// See the following for the structure:
// https://docs.gitlab.com/ee/ci/secrets/id_token_authentication.html#id-tokens
type IDTokenClaims struct {
	oidc.TokenClaims
	// Sub roughly uniquely identifies the workload. Example:
	// `project_path:mygroup/my-project:ref_type:branch:ref:main`
	// project_path:{group}/{project}:ref_type:{type}:ref:{branch_name}
	Sub string `json:"sub"`
	// Git ref for this job
	Ref string `json:"ref"`
	// Git ref type. Example:
	// `branch` or `tag`
	RefType string `json:"ref_type"`
	// 	true if the Git ref is protected, false otherwise.
	RefProtected string `json:"ref_protected"`
	// Use this to scope to group or user level namespace by path. Example:
	// `mygroup`
	NamespacePath string `json:"namespace_path"`
	// Use this to scope to group or user level namespace by ID.
	NamespaceID string `json:"namespace_id"`
	// Use this to scope to project by path. Example:
	// `mygroup/myproject`
	ProjectPath string `json:"project_path"`
	// Use this to scope to project by ID.
	ProjectID string `json:"project_id"`
	// ID of the user executing the job
	UserID string `json:"user_id"`
	// Username of the user executing the job
	UserLogin string `json:"user_login"`
	// Email of the user executing the job
	UserEmail string `json:"user_email"`
	// Pipeline source.
	// https://docs.gitlab.com/ee/ci/jobs/job_control.html#common-if-clauses-for-rules
	// Example: `web`
	PipelineSource string `json:"pipeline_source"`
	// ID of the pipeline.
	PipelineID string `json:"pipeline_id"`
	// Environment this job deploys to (if one is associated)
	Environment string `json:"environment"`
	// 	true if deployed environment is protected, false otherwise
	EnvironmentProtected string `json:"environment_protected"`
	// 	Environment action (environment:action) specified in the job.
	EnvironmentAction string `json:"environment_action"`
	// The ref path to the top-level pipeline definition, for example, gitlab.example.com/my-group/my-project//.gitlab-ci.yml@refs/heads/main.
	CIConfigRefURI string `json:"ci_config_ref_uri"`
	// Git commit SHA for the ci_config_ref_uri.
	CIConfigSHA string `json:"ci_config_sha"`
	// 	The commit SHA for the job.
	SHA string `json:"sha"`
	// ID of the runner executing the job.
	RunnerID int `json:"runner_id"`
	// The type of runner used by the job. Can be either gitlab-hosted or self-hosted
	RunnerEnvironment string `json:"runner_environment"`
	// Deployment tier of the environment the job specifies
	DeploymentTier string `json:"deployment_tier"`
	// The visibility of the project where the pipeline is running. Can be internal, private, or public.
	ProjectVisibility string `json:"project_visibility"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsGitLab {
	attrs := &workloadidentityv1pb.JoinAttrsGitLab{
		Sub:                  c.Sub,
		Ref:                  c.Ref,
		RefType:              c.RefType,
		RefProtected:         c.RefProtected == "true",
		NamespacePath:        c.NamespacePath,
		ProjectPath:          c.ProjectPath,
		UserLogin:            c.UserLogin,
		UserEmail:            c.UserEmail,
		PipelineId:           c.PipelineID,
		Environment:          c.Environment,
		EnvironmentProtected: c.EnvironmentProtected == "true",
		RunnerId:             int64(c.RunnerID),
		RunnerEnvironment:    c.RunnerEnvironment,
		Sha:                  c.SHA,
		CiConfigRefUri:       c.CIConfigRefURI,
		CiConfigSha:          c.CIConfigSHA,
	}

	return attrs
}

// CheckIDTokenParams are parameters used to validate GitLab OIDC tokens.
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

// CheckIDToken verifies a GitLab OIDC token
func CheckIDToken(ctx context.Context, params *CheckIDTokenParams) (*IDTokenClaims, error) {
	if err := params.validate(); err != nil {
		return nil, trace.AccessDenied("%s", err.Error())
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("gitlab join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	var claims *IDTokenClaims
	var err error
	if token.Spec.GitLab.StaticJWKS != "" {
		claims, err = params.Validator.ValidateTokenWithJWKS(
			ctx, []byte(token.Spec.GitLab.StaticJWKS), string(params.IDToken),
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with static jwks")
		}
	} else {
		claims, err = params.Validator.Validate(
			ctx, token.Spec.GitLab.Domain, string(params.IDToken),
		)
		if err != nil {
			return nil, trace.Wrap(err, "validating with oidc")
		}
	}

	log.InfoContext(ctx, "GitLab CI run trying to join cluster",
		"claims", claims,
		"token", params.ProvisionToken.GetName(),
	)

	return claims, trace.Wrap(checkGitLabAllowRules(token, claims))
}

func checkGitLabAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
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
		subMatches, err := joinutils.GlobMatchAllowEmptyPattern(rule.Sub, claims.Sub)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) sub match", i)
		}
		if !subMatches {
			continue
		}
		refMatches, err := joinutils.GlobMatchAllowEmptyPattern(rule.Ref, claims.Ref)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) ref match", i)
		}
		if !refMatches {
			continue
		}
		if rule.RefType != "" && claims.RefType != rule.RefType {
			continue
		}
		namespacePathMatches, err := joinutils.GlobMatchAllowEmptyPattern(rule.NamespacePath, claims.NamespacePath)
		if err != nil {
			return trace.Wrap(err, "evaluating rule (%d) namespace_path match", i)
		}
		if !namespacePathMatches {
			continue
		}
		projectPathMatches, err := joinutils.GlobMatchAllowEmptyPattern(rule.ProjectPath, claims.ProjectPath)
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
