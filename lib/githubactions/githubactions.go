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
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
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
