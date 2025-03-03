/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package helpers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/web"
	websession "github.com/gravitational/teleport/lib/web/session"
	"github.com/gravitational/teleport/lib/web/ui"
)

// WebClientPack is an authenticated HTTP Client for Teleport.
type WebClientPack struct {
	clt         *http.Client
	host        string
	webCookie   string
	bearerToken string
	clusterName string
}

// LoginWebClient receives the host url, the username and a password.
// It will login into that host and return a WebClientPack.
func LoginWebClient(t *testing.T, host, username, password string) *WebClientPack {
	csReq, err := json.Marshal(web.CreateSessionReq{
		User: username,
		Pass: password,
	})
	require.NoError(t, err)

	// Create POST request to create session.
	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v1/webapi/sessions/web",
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(csReq))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Issue request.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Read in response.
	var csResp *web.CreateSessionResponse
	err = json.NewDecoder(resp.Body).Decode(&csResp)
	require.NoError(t, err)

	// Extract session cookie and bearer token.
	require.Len(t, resp.Cookies(), 1)
	cookie := resp.Cookies()[0]
	require.Equal(t, websession.CookieName, cookie.Name)

	webClient := &WebClientPack{
		clt:         client,
		host:        host,
		webCookie:   cookie.Value,
		bearerToken: csResp.Token,
	}

	respStatusCode, bs := webClient.DoRequest(t, http.MethodGet, "sites", nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(bs))

	var clusters []ui.Cluster
	require.NoError(t, json.Unmarshal(bs, &clusters), string(bs))
	require.NotEmpty(t, clusters)

	webClient.clusterName = clusters[0].Name
	return webClient
}

// LoginMFAWebClient receives the host url and a passwordless
// device to carry out login and return a WebClientPack.
func LoginMFAWebClient(t *testing.T, host string, passwordlessDevice *mocku2f.Key) *WebClientPack {
	// Begin login
	loginReq, err := json.Marshal(client.MFAChallengeRequest{
		Passwordless: true,
	})
	require.NoError(t, err)

	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v1/webapi/mfa/login/begin",
	}
	beginLoginReq, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(loginReq))
	require.NoError(t, err)

	beginLoginReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	clt := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	beginLoginResp, err := clt.Do(beginLoginReq)
	require.NoError(t, err)
	defer beginLoginResp.Body.Close()
	require.Equal(t, http.StatusOK, beginLoginResp.StatusCode)

	// Read mfa challenge from response and pass it to the passwordless device.
	var mfaChallenge *client.MFAAuthenticateChallenge
	err = json.NewDecoder(beginLoginResp.Body).Decode(&mfaChallenge)
	require.NoError(t, err)

	origin := fmt.Sprintf("https://%v", host)
	webauthnResponse, err := passwordlessDevice.SignAssertion(origin, mfaChallenge.WebauthnChallenge)
	require.NoError(t, err)

	// Finish login and get a web session.
	finishReq, err := json.Marshal(client.AuthenticateWebUserRequest{
		WebauthnAssertionResponse: webauthnResponse,
	})
	require.NoError(t, err)

	u = url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v1/webapi/mfa/login/finishsession",
	}
	finishLoginReq, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(finishReq))
	require.NoError(t, err)

	finishLoginReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Issue request.
	finishLoginResp, err := clt.Do(finishLoginReq)
	require.NoError(t, err)
	defer finishLoginResp.Body.Close()
	require.Equal(t, http.StatusOK, finishLoginResp.StatusCode)

	// Read in response.
	var csResp *web.CreateSessionResponse
	err = json.NewDecoder(finishLoginResp.Body).Decode(&csResp)
	require.NoError(t, err)

	// Extract session cookie and bearer token.
	require.Len(t, finishLoginResp.Cookies(), 1)
	cookie := finishLoginResp.Cookies()[0]
	require.Equal(t, websession.CookieName, cookie.Name)

	webClient := &WebClientPack{
		clt:         clt,
		host:        host,
		webCookie:   cookie.Value,
		bearerToken: csResp.Token,
	}

	respStatusCode, bs := webClient.DoRequest(t, http.MethodGet, "sites", nil)
	require.Equal(t, http.StatusOK, respStatusCode, string(bs))

	var clusters []ui.Cluster
	require.NoError(t, json.Unmarshal(bs, &clusters), string(bs))
	require.NotEmpty(t, clusters)

	webClient.clusterName = clusters[0].Name
	return webClient
}

// DoRequest receives a method, endpoint and payload and sends an HTTP Request to the Teleport API.
// The endpoint must not contain the host neither the base path ('/v1/webapi/').
// Status Code and Body are returned.
// "$site" in the endpoint is substituted by the current site.
func (w *WebClientPack) DoRequest(t *testing.T, method, endpoint string, payload any) (int, []byte) {
	endpoint = fmt.Sprintf("https://%s/v1/webapi/%s", w.host, endpoint)
	endpoint = strings.ReplaceAll(endpoint, "$site", w.clusterName)
	u, err := url.Parse(endpoint)
	require.NoError(t, err)

	bs, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(bs))
	require.NoError(t, err)

	req.AddCookie(&http.Cookie{
		Name:  websession.CookieName,
		Value: w.webCookie,
	})
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", w.bearerToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := w.clt.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, body
}

// OpenWebsocket opens a websocket on a given Teleport API endpoint.
// The endpoint must not contain the host neither the base path ('/v1/webapi/').
// Raw websocket and HTTP response are returned.
// "$site" in the endpoint is substituted by the current site.
func (w *WebClientPack) OpenWebsocket(t *testing.T, endpoint string, params any) (*websocket.Conn, *http.Response, error) {
	path, err := url.JoinPath("v1", "webapi", strings.ReplaceAll(endpoint, "$site", w.clusterName))
	require.NoError(t, err)

	u := url.URL{
		Host:   w.host,
		Scheme: client.WSS,
		Path:   path,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AuthBearer, w.bearerToken)
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	cookie := &http.Cookie{
		Name:  websession.CookieName,
		Value: w.webCookie,
	}

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	header.Add("Cookie", cookie.String())

	ws, resp, err := dialer.Dial(u.String(), header)
	require.NoError(t, err)

	authReq, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: w.bearerToken})
	require.NoError(t, err)

	if err := ws.WriteMessage(websocket.TextMessage, authReq); err != nil {
		return nil, nil, err
	}

	return ws, resp, nil
}
