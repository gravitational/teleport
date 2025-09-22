// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

const (
	getTokenBaseURL     = "https://login.microsoftonline.com"
	getTokenContentType = "application/x-www-form-urlencoded"
)

// Token represents utility struct used for parsing GetToken resposne
type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// tokenWithTTL represents struct which handles token refresh on expiration
type tokenWithTTL struct {
	mu        sync.RWMutex
	token     Token
	scope     string
	expiresAt int64
	baseURL   string
}

// Bearer returns current token value and refreshes it if token is expired.
//
// MS Graph API issues no refresh_token for client_credentials grant type. There also is no
// extended validity window for this grant type.
func (c *tokenWithTTL) Bearer(ctx context.Context, config Config) (string, error) {
	c.mu.RLock()
	expiresAt := c.expiresAt
	c.mu.RUnlock()

	if expiresAt == 0 || expiresAt < time.Now().UnixNano() {
		token, err := c.getToken(ctx, c.scope, config)
		if err != nil {
			return "", trace.Wrap(err)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		c.token = token
		// We renew the token 1 minute before its expiration to deal with possible time skew
		c.expiresAt = time.Now().UnixNano() + (token.ExpiresIn * int64(time.Second)) - int64(time.Minute)
	}

	return "Bearer " + c.token.AccessToken, nil
}

// getToken calls /token endpoint and returns Bearer string
func (c *tokenWithTTL) getToken(ctx context.Context, scope string, config Config) (Token, error) {
	client := http.Client{Timeout: httpTimeout}
	t := Token{}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", config.AppID)
	data.Set("client_secret", config.AppSecret)
	data.Set("scope", scope)

	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = getTokenBaseURL
	}

	getTokenURL := baseURL + "/" + config.TenantID + "/oauth2/v2.0/token"

	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		getTokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return t, trace.Wrap(err)
	}

	u, err := url.Parse(getTokenBaseURL)
	if err != nil {
		return t, trace.Wrap(err)
	}

	r.Header.Add("Host", u.Host)
	r.Header.Add("Content-Type", getTokenContentType)

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(backoffBase),
		First:  backoffBase,
		Max:    backoffMax,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return t, trace.Wrap(err)
	}
	for {
		resp, err := client.Do(r)
		if err != nil {
			return t, trace.Wrap(err)
		}

		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return t, trace.Wrap(err)
		}

		if resp.StatusCode != http.StatusOK {
			select {
			case <-ctx.Done():
				return t, trace.Errorf("Failed to get auth token %v %v %v", resp.StatusCode, scope, string(b))
			case <-retry.After():
				continue
			}
		}

		err = json.NewDecoder(bytes.NewReader(b)).Decode(&t)
		if err != nil {
			return t, trace.Wrap(err)
		}

		return t, nil
	}
}
