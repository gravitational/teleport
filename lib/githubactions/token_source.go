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
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"github.com/gravitational/trace"
)

type tokenResponse struct {
	Value string `json:"value"`
}

// IDTokenSource allows a GitHub ID token to be fetched whilst executing
// within the context of a GitHub actions workflow.
type IDTokenSource struct {
	getIDTokenURL   func() string
	getRequestToken func() string
	client          http.Client
}

func NewIDTokenSource() *IDTokenSource {
	return &IDTokenSource{
		getIDTokenURL: func() string {
			return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
		},
		getRequestToken: func() string {
			return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		},
	}
}

// GetIDToken utilizes values set in the environment and the GitHub API to
// fetch a GitHub issued IDToken.
func (ip *IDTokenSource) GetIDToken(ctx context.Context) (string, error) {
	audience := "teleport.cluster.local"

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

	var data tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", trace.Wrap(err)
	}

	if data.Value == "" {
		return "", trace.BadParameter("response did not include ID token")
	}

	return data.Value, nil
}
