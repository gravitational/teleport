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

	resp, err := (&http.Client{}).Do(req)
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

func getPrivateAPIToken(ctx context.Context, tenantID string) (string, error) {
	tokens, err := getRefreshTokens()
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, token := range tokens {
		slog.Info("trying token", "client_id", token.ClientID)
		token, err := exchangeToken(ctx, tenantID, token)
		if err != nil {
			slog.Error("error exchanging token", "err", err)
		} else {
			return token, nil
		}
	}
	return "", trace.Errorf("no viable token")
}

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

	resp, err := (&http.Client{}).Do(req)
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
