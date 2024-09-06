// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package azureoidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

type msalTokenCache struct {
	RefreshToken map[string]msalToken `json:"RefreshToken"`
}

type msalToken struct {
	ClientID string `json:"client_id"`
	Secret   string `json:"secret"`
}

type exchangeResponse struct {
	AccessToken string `json:"access_token"`
}

// getRefreshTokens returns all current refresh tokens from the Azure CLI token cache.
func getRefreshTokens() ([]msalToken, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := os.Open(filepath.Join(usr.HomeDir, ".azure/msal_token_cache.json"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tokenCache msalTokenCache
	if err := json.NewDecoder(f).Decode(&tokenCache); err != nil {
		return nil, trace.Wrap(err)
	}

	var results []msalToken
	for _, tok := range tokenCache.RefreshToken {
		results = append(results, tok)
	}
	if len(results) == 0 {
		return nil, trace.NotFound("no refresh tokens found in MSAL token cache")
	}
	return results, nil
}

// exchangeToken takes az CLI token and exchanges it for one suitable for the private API
func exchangeToken(ctx context.Context, tenantID string, token msalToken) (string, error) {
	params := url.Values{
		"client_id":     {token.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.Secret},
		"scope":         {"74658136-14ec-4630-ad9b-26e160ff0fc6/.default openid profile offline_access"},
	}

	uri := url.URL{
		Host:   "login.microsoftonline.com",
		Path:   path.Join(tenantID, "oauth2/v2.0/token"),
		Scheme: "https",
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uri.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return "", trace.Wrap(err)
	}

	client, err := defaults.HTTPClient()
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", trace.Errorf("failed to exchange token: %s", string(payload))
	}

	var response exchangeResponse
	err = json.Unmarshal(payload, &response)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if response.AccessToken == "" {
		return "", trace.NotFound("expected non-empty AccessToken in the response")
	}

	return response.AccessToken, nil
}

// getPrivateAPIToken uses the azure CLI token cache to exchange a refresh token
// for an access token authenticated to the "private" Azure API.
func getPrivateAPIToken(ctx context.Context, tenantID string) (string, error) {
	var err error
	tokens, err := getRefreshTokens()
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, token := range tokens {
		var tokenStr string
		slog.DebugContext(ctx, "trying token", "client_id", token.ClientID)
		tokenStr, err = exchangeToken(ctx, tenantID, token)
		if err != nil {
			slog.DebugContext(ctx, "error exchanging token", "err", err)
		} else {
			return tokenStr, nil
		}
	}
	return "", trace.Wrap(err, "no viable token")
}

// privateAPIGet invokes GET on the given endpoint of the "private" main.iam.ad.ext.azure.com azure API.
// On status code 200 OK, it returns the payload received.
// On any other status code, or protocol errors, it returns an error.
func privateAPIGet(ctx context.Context, accessToken string, endpoint string) ([]byte, error) {
	uri := url.URL{
		Scheme: "https",
		Host:   "main.iam.ad.ext.azure.com",
		Path:   path.Join("api", endpoint),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("x-ms-client-request-id", uuid.NewString())

	client, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("request to %s failed: %s", endpoint, string(payload))
	}
	return payload, trace.Wrap(err)
}
