/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// IDTokenSource allows a Azure Devops OIDC token to be fetched whilst within a
// Pipelines execution.
type IDTokenSource struct {
	// getEnv is a function that returns a string from the environment, usually
	// os.Getenv except in tests.
	getEnv     func(key string) string
	httpClient *http.Client
}

// GetIDToken attempts to fetch a Azure Devops OIDC token from the environment.
func (its *IDTokenSource) GetIDToken(ctx context.Context) (string, error) {
	tok := its.getEnv("SYSTEM_ACCESSTOKEN")
	if tok == "" {
		return "", trace.BadParameter(
			"SYSTEM_ACCESSTOKEN environment variable missing",
		)
	}

	rawBaseURL := its.getEnv("SYSTEM_OIDCREQUESTURI")
	if rawBaseURL == "" {
		return "", trace.BadParameter(
			"SYSTEM_OIDCREQUESTURI environment variable missing",
		)
	}

	idToken, err := its.exchangeToken(ctx, tok, rawBaseURL)
	if err != nil {
		return "", trace.Wrap(err, "exchanging token")
	}

	return idToken, nil
}

// See https://learn.microsoft.com/en-us/rest/api/azure/devops/distributedtask/oidctoken/create?view=azure-devops-rest-7.1&preserve-view=true
type createOidctokenResp struct {
	OIDCToken string `json:"oidcToken"`
}

func (its *IDTokenSource) exchangeToken(
	ctx context.Context, accessToken string, rawBaseURL string,
) (string, error) {
	// Exchange Access Token for OIDC token using Oidctoken - Create API
	// https://learn.microsoft.com/en-us/rest/api/azure/devops/distributedtask/oidctoken/create?view=azure-devops-rest-7.1&preserve-view=true
	apiURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", trace.Wrap(err, "parsing base URL")
	}
	query := apiURL.Query()
	query.Set("api-version", "7.1")
	apiURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, apiURL.String(), nil,
	)
	if err != nil {
		return "", trace.Wrap(err, "creating request for token")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	res, err := its.httpClient.Do(req)
	if err != nil {
		return "", trace.Wrap(err, "making request for token")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", trace.BadParameter(
			"received status code %d, expected 200", res.StatusCode,
		)
	}

	var data createOidctokenResp
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", trace.Wrap(err)
	}

	if data.OIDCToken == "" {
		return "", trace.BadParameter("resp did not include oidc token")
	}
	return data.OIDCToken, nil
}

// NewIDTokenSource builds a helper that can extract a Azure Devops OIDC token
// from the environment, using `getEnv`.
func NewIDTokenSource(getEnv func(key string) string) *IDTokenSource {
	return &IDTokenSource{
		getEnv:     getEnv,
		httpClient: otelhttp.DefaultClient,
	}
}
