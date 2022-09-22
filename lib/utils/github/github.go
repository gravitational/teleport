package github

import (
	"context"
	"encoding/json"
	"github.com/gravitational/trace"
	"io"
	"net/http"
	"net/url"
	"os"
)

// GitHub Workload Identity
//
// GH provides workloads with two environment variables to faciliate fetching
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
// The `audience` query parameter can be used to customise the audience claim
// within the resulting ID token.
//
// Valuable reference:
// - https://github.com/actions/toolkit/blob/main/packages/core/src/oidc-utils.ts
// - https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers

type tokenResponse struct {
	Value string `json:"value"`
}

// IdentityProvider allows a GitHub ID token to be fetched whilst executing
// within the context of a GitHub actions workflow.
type IdentityProvider struct {
	getIDTokenURL   func() string
	getRequestToken func() string
	client          http.Client
}

func NewIdentityProvider() *IdentityProvider {
	return &IdentityProvider{
		getIDTokenURL: func() string {
			return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
		},
		getRequestToken: func() string {
			return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		},
	}
}

func (ip *IdentityProvider) GetIDToken(ctx context.Context) (string, error) {
	// TODO: Inject audience to be set
	audience := "teleport.ottr.sh"

	tokenURL := ip.getIDTokenURL()
	requestToken := ip.getRequestToken()
	if tokenURL == "" {
		return "", trace.BadParameter(
			"ACTIONS_ID_TOKEN_REQUEST_URL environment variable missing",
		)
	}
	if requestToken == "" {
		return "", trace.BadParameter(
			"ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable missing",
		)
	}

	tokenURL = tokenURL + "&audience=" + url.QueryEscape(audience)
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, tokenURL, nil,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	req.Header.Set("Authorization", "Bearer "+requestToken)
	req.Header.Set("Accept", "application/json; api-version=2.0")
	req.Header.Set("Content-Type", "application/json")
	res, err := ip.client.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer res.Body.Close()

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var data tokenResponse
	if err := json.Unmarshal(bytes, &data); err != nil {
		return "", trace.Wrap(err)
	}

	if data.Value == "" {
		return "", trace.Errorf("response did not include ID token")
	}

	return data.Value, nil
}

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
	// The name of the workflow.
	Workflow string `json:"workflow"`
}
