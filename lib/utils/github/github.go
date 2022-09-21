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

// See https://github.com/actions/toolkit/blob/main/packages/core/src/oidc-utils.ts
// for reference.

type tokenResponse struct {
	Value string `json:"value"`
}

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
	tokenURL := ip.getIDTokenURL()
	requestToken := ip.getRequestToken()
	// TODO: Inject audience to be set
	audience := "teleport.ottr.sh"

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
