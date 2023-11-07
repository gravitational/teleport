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

package gitlab

import (
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
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
	RunnerID string `json:"runner_id"`
	// The type of runner used by the job. Can be either gitlab-hosted or self-hosted
	RunnerEnvironment string `json:"runner_environment"`
	// Deployment tier of the environment the job specifies
	DeploymentTier string `json:"deployment_tier"`
	// The visibility of the project where the pipeline is running. Can be internal, private, or public.
	ProjectVisibility string `json:"project_visibility"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *IDTokenClaims) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}
