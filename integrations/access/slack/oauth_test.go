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

package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"
)

type testOAuthServer struct {
	clientID          string
	clientSecret      string
	authorizationCode string
	redirectURI       string
	refreshToken      string

	exchangeResponse *slack.OAuthV2Response
	refreshResponse  *slack.OAuthV2Response

	srv *httptest.Server
	t   *testing.T
}

func (s *testOAuthServer) handler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	require.NoError(s.t, r.ParseForm())
	if grantType := r.Form.Get("grant_type"); grantType == "refresh_token" {
		s.refresh(w, r)
	} else {
		s.exchange(w, r)
	}
}

func (s *testOAuthServer) exchange(w http.ResponseWriter, r *http.Request) {
	require.Equal(s.t, s.clientID, r.Form.Get("client_id"))
	require.Equal(s.t, s.clientSecret, r.Form.Get("client_secret"))
	require.Equal(s.t, s.redirectURI, r.Form.Get("redirect_uri"))
	require.Equal(s.t, s.authorizationCode, r.Form.Get("code"))

	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(s.exchangeResponse)
	require.NoError(s.t, err)
}

func (s *testOAuthServer) refresh(w http.ResponseWriter, r *http.Request) {
	require.Equal(s.t, s.clientID, r.Form.Get("client_id"))
	require.Equal(s.t, s.clientSecret, r.Form.Get("client_secret"))
	require.Equal(s.t, s.refreshToken, r.Form.Get("refresh_token"))

	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(s.refreshResponse)
	require.NoError(s.t, err)
}

func (s *testOAuthServer) start() {
	router := httprouter.New()
	router.POST("/api/oauth.v2.access", s.handler)

	s.srv = httptest.NewServer(router)
}

func (s *testOAuthServer) url() string {
	return s.srv.URL + "/"
}

func (s *testOAuthServer) close() {
	s.srv.Close()
}

type testOAuthRoundTripper struct {
	replaceAPIURL *url.URL
	delegate      http.RoundTripper
}

func (t *testOAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	newReq.URL.Host = t.replaceAPIURL.Host
	newReq.Host = t.replaceAPIURL.Host
	return t.delegate.RoundTrip(newReq)
}

func newTestOAuthHttpClient(t *testing.T, replaceURL string) *http.Client {
	t.Helper()

	parsedURL, err := url.Parse(replaceURL)
	require.NoError(t, err)

	return &http.Client{
		Transport: &testOAuthRoundTripper{
			replaceAPIURL: parsedURL,
			delegate:      http.DefaultTransport,
		},
	}
}

func TestOAuth(t *testing.T) {
	const (
		clientID          = "my-client-id"
		clientSecret      = "my-client-secret"
		authorizationCode = "12345678"
		redirectURI       = "https://foobar.com/callback"
		refreshToken      = "my-refresh-token1"
		expiresInSeconds  = 43200
	)

	newServer := func(t *testing.T) *testOAuthServer {
		s := &testOAuthServer{
			clientID:          clientID,
			clientSecret:      clientSecret,
			authorizationCode: authorizationCode,
			redirectURI:       redirectURI,
			refreshToken:      refreshToken,

			t: t,
		}
		s.start()
		return s
	}

	ok := func(accessToken string, refreshToken string, expiresInSeconds int) *slack.OAuthV2Response {
		return &slack.OAuthV2Response{
			SlackResponse: slack.SlackResponse{
				Ok: true,
			},
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresInSeconds,
		}
	}

	fail := func(e string) *slack.OAuthV2Response {
		return &slack.OAuthV2Response{
			SlackResponse: slack.SlackResponse{
				Ok:    false,
				Error: e,
			},
		}
	}

	t.Run("ExchangeOK", func(t *testing.T) {
		s := newServer(t)
		defer s.close()
		s.exchangeResponse = ok("my-access-token1", "my-refresh-token2", expiresInSeconds)

		authorizer := newAuthorizer(newTestOAuthHttpClient(t, s.url()), clientID, clientSecret)

		creds, err := authorizer.Exchange(context.Background(), s.authorizationCode, s.redirectURI)
		require.NoError(t, err)
		require.Equal(t, s.exchangeResponse.AccessToken, creds.AccessToken)
		require.Equal(t, s.exchangeResponse.RefreshToken, creds.RefreshToken)
		require.WithinDuration(t, time.Now().Add(time.Duration(expiresInSeconds)*time.Second), creds.ExpiresAt, 1*time.Second)
	})

	t.Run("ExchangeFail", func(t *testing.T) {
		s := newServer(t)
		defer s.close()
		s.exchangeResponse = fail("invalid_code")

		authorizer := newAuthorizer(newTestOAuthHttpClient(t, s.url()), clientID, clientSecret)

		_, err := authorizer.Exchange(context.Background(), s.authorizationCode, s.redirectURI)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid_code")
	})

	t.Run("RefreshOK", func(t *testing.T) {
		s := newServer(t)
		defer s.close()
		s.refreshResponse = ok("my-access-token2", "my-refresh-token3", expiresInSeconds)

		authorizer := newAuthorizer(newTestOAuthHttpClient(t, s.url()), clientID, clientSecret)

		creds, err := authorizer.Refresh(context.Background(), refreshToken)
		require.NoError(t, err)
		require.Equal(t, s.refreshResponse.AccessToken, creds.AccessToken)
		require.Equal(t, s.refreshResponse.RefreshToken, creds.RefreshToken)
		require.WithinDuration(t, time.Now().Add(time.Duration(expiresInSeconds)*time.Second), creds.ExpiresAt, 1*time.Second)
	})

	t.Run("RefreshFail", func(t *testing.T) {

		s := newServer(t)
		defer s.close()
		s.refreshResponse = fail("expired_token")

		authorizer := newAuthorizer(newTestOAuthHttpClient(t, s.url()), clientID, clientSecret)

		_, err := authorizer.Refresh(context.Background(), refreshToken)
		require.Error(t, err)
		require.ErrorContains(t, err, "expired_token")
	})
}
