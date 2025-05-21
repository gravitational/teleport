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
	"github.com/zitadel/oidc/v3/pkg/oidc"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

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
