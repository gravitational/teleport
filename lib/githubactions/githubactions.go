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

package githubactions

import (
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
)

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
