/*
Copyright 2021 Gravitational, Inc.

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

package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/stretchr/testify/require"
)

// TestAuthPOST tests the handler of POST /x-teleport-auth.
func TestAuthPOST(t *testing.T) {
	const (
		stateValue  = "012ac605867e5a7d693cd6f49c7ff0fb"
		cookieValue = "5588e2be54a2834b4f152c56bafcd789f53b15477129d2ab4044e9a3c1bf0f3b"
	)

	tests := []struct {
		desc           string
		stateInRequest string
		stateInCookie  string
		sessionError   error
		outStatusCode  int
	}{
		{
			desc:           "success",
			stateInRequest: stateValue,
			stateInCookie:  stateValue,
			sessionError:   nil,
			outStatusCode:  http.StatusOK,
		},
		{
			desc:           "missing state token in request",
			stateInRequest: "",
			stateInCookie:  stateValue,
			sessionError:   nil,
			outStatusCode:  http.StatusForbidden,
		},
		{
			desc:           "invalid session",
			stateInRequest: stateValue,
			stateInCookie:  stateValue,
			sessionError:   trace.NotFound("invalid session"),
			outStatusCode:  http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			p := setup(t, test.sessionError)

			req, err := json.Marshal(fragmentRequest{
				StateValue:  test.stateInRequest,
				CookieValue: cookieValue,
			})
			require.NoError(t, err)

			status := p.makeRequest(t, "POST", "/x-teleport-auth", test.stateInCookie, req)
			require.Equal(t, test.outStatusCode, status)
		})
	}
}

type testServer struct {
	serverURL *url.URL
}

func setup(t *testing.T, sessionError error) *testServer {
	fakeClock := clockwork.NewFakeClockAt(time.Date(2017, 05, 10, 18, 53, 0, 0, time.UTC))
	authClient := mockAuthClient{
		sessionError: sessionError,
	}
	appHandler, err := NewHandler(context.Background(), &HandlerConfig{
		Clock:        fakeClock,
		AuthClient:   authClient,
		AccessPoint:  authClient,
		CipherSuites: utils.DefaultCipherSuites(),
	})
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(appHandler)
	server.StartTLS()

	url, err := url.Parse(server.URL)
	require.NoError(t, err)

	return &testServer{
		serverURL: url,
	}
}

func (p *testServer) makeRequest(t *testing.T, method, endpoint, stateInCookie string, reqBody []byte) int {
	u := url.URL{
		Scheme: p.serverURL.Scheme,
		Host:   p.serverURL.Host,
		Path:   endpoint,
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(reqBody))
	require.NoError(t, err)

	// Attach state token cookie.
	req.AddCookie(&http.Cookie{
		Name:  AuthStateCookieName,
		Value: stateInCookie,
	})

	// Issue request.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode
}

type mockAuthClient struct {
	auth.ClientI
	sessionError error
}

type mockClusterName struct {
	types.ClusterName
}

func (c mockAuthClient) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return mockClusterName{}, nil
}

func (n mockClusterName) GetClusterName() string {
	return "local-cluster"
}

func (c mockAuthClient) GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error) {
	return nil, c.sessionError
}
