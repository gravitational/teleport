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
	req.Header.Set("Authorization", requestToken)
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
