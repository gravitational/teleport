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

package snowflake

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Snowflake engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
		HTTPClient:   getDefaultHTTPClient(),
		tokens:       newTokenCache(),
	}
}

// getDefaultHTTPClient returns default HTTP client used by the Snowflake engine.
func getDefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			// We don't want the http.Client to automatically decompress the data.
			DisableCompression: true,
		},
	}
}

type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// HTTPClient is the client being used to talk to Snowflake API.
	HTTPClient *http.Client
	// tokens is a tokens cache that holds the current session and master token
	// used in the communication with the Snowflake.
	tokens tokenCache
	// accountName is the Snowflake account name
	accountName string
	// snowflakeHost is the Snowflake host URL that the engine is talking to.
	snowflakeHost string
}

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	// Make sure that tokens are initialized. Tokens are not initialized in tests.
	if e.tokens.tokens == nil {
		e.tokens = newTokenCache()
	}
	return nil
}

func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}

	// Assume internal server error HTTP 500 and override if possible.
	statusCode := http.StatusInternalServerError
	if trace.IsAccessDenied(err) {
		statusCode = http.StatusUnauthorized
	}

	jsonBody, err := json.Marshal(struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: false,
		Message: err.Error(),
	})
	if err != nil {
		e.Log.WithError(err).Errorf("failed to marshal error response")
		return
	}

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBuffer(jsonBody)),
		Header: map[string][]string{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonBody))},
		},
	}

	if err := response.Write(e.clientConn); err != nil {
		e.Log.Errorf("snowflake error: %+v", trace.Unwrap(err))
		return
	}
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)

	var err error
	e.accountName, e.snowflakeHost, err = parseConnectionString(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := e.authorizeConnection(ctx); err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)

	observe()

	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		err = e.process(ctx, sessionCtx, req, msgFromClient, msgFromServer)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected Snowflake client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, sessionCtx *common.Session, req *http.Request, msgFromClient prometheus.Counter, msgFromServer prometheus.Counter) error {
	msgFromClient.Inc()

	snowflakeToken, err := e.getConnectionToken(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	requestBodyReader, err := e.processRequest(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy, err := copyRequest(ctx, req, requestBodyReader)
	if err != nil {
		return trace.Wrap(err)
	}

	// Force HTTPS usage even and update the host url.
	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = e.snowflakeHost

	e.setAuthorizationHeader(reqCopy, snowflakeToken)

	// Send the request to Snowflake API
	resp, err := e.HTTPClient.Do(reqCopy)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	msgFromServer.Inc()

	switch req.URL.Path {
	case loginRequestPath:
		// Return here - loginResponseHandler sends the response.
		return trace.Wrap(e.loginResponseHandler(ctx, sessionCtx, resp))
	case tokenRequestPath:
		// Return here - tokenResponseHandler sends the response.
		return trace.Wrap(e.tokenResponseHandler(ctx, sessionCtx, resp))
	}

	return trace.Wrap(e.sendResponse(resp))
}

func (e *Engine) tokenResponseHandler(ctx context.Context, sessionCtx *common.Session, resp *http.Response) error {
	err := e.processResponse(resp, func(body []byte) ([]byte, error) {
		renewSessResp := &renewSessionResponse{}
		if err := json.Unmarshal(body, renewSessResp); err != nil {
			return nil, trace.Wrap(err)
		}

		if renewSessResp.Data.SessionToken == "" {
			return nil, trace.Errorf("Snowflake renew token response doesn't contain new session token")
		}

		snowflakeSession, err := e.AuthClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
			Username:     sessionCtx.Identity.Username,
			SessionToken: renewSessResp.Data.SessionToken,
			TokenTTL:     renewSessResp.Data.ValidityInSecondsST,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		e.tokens.setToken(snowflakeSession.GetName(), renewSessResp.Data.SessionToken)
		renewSessResp.Data.SessionToken = teleportAuthHeaderPrefix + snowflakeSession.GetName()

		if renewSessResp.Data.MasterToken != "" {
			masterToken, err := e.AuthClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
				Username:     sessionCtx.Identity.Username,
				SessionToken: renewSessResp.Data.MasterToken,
				TokenTTL:     renewSessResp.Data.ValidityInSecondsMT,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			e.tokens.setToken(masterToken.GetName(), renewSessResp.Data.MasterToken)
			renewSessResp.Data.MasterToken = teleportAuthHeaderPrefix + masterToken.GetName()
		}

		newBody, err := json.Marshal(renewSessResp)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return newBody, nil
	})
	return trace.Wrap(err)
}

func (e *Engine) loginResponseHandler(ctx context.Context, sessionCtx *common.Session, resp *http.Response) error {
	err := e.processResponse(resp, func(body []byte) ([]byte, error) {
		newResp, err := e.processLoginResponse(body, func(tokens sessionTokens) (string, string, error) {
			// Create one session for connection token.
			snowflakeSession, err := e.AuthClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
				Username:     sessionCtx.Identity.Username,
				SessionToken: tokens.session.token,
				// TokenTTL is the TTL of the token in auth server. Technically we could remove the token when
				// the Snowflake token expires too, but Snowflake API requires the old token to be added as a part
				// of the renewal request. Because of that adding extra 10 minutes should allow the client to
				// renew the token even if it's not in our "tokens" cache.
				TokenTTL: tokens.session.ttl + 600, // add 10 minutes
			})
			if err != nil {
				return "", "", trace.Wrap(err)
			}

			// And another one for master/renew one.
			snowflakeMasterSession, err := e.AuthClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
				Username:     sessionCtx.Identity.Username,
				SessionToken: tokens.master.token,
				TokenTTL:     tokens.master.ttl + 600, // add 10 minutes
			})
			if err != nil {
				return "", "", trace.Wrap(err)
			}

			return snowflakeSession.GetName(), snowflakeMasterSession.GetName(), nil
		})
		if err != nil {
			return nil, trace.Wrap(err, "failed to extract Snowflake session token")
		}

		return newResp, nil
	})
	return trace.Wrap(err)
}

// sendResponse sends the response back to the Snowflake client.
func (e *Engine) sendResponse(resp *http.Response) error {
	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *Engine) setAuthorizationHeader(reqCopy *http.Request, snowflakeToken string) {
	if snowflakeToken != "" {
		reqCopy.Header.Set("Authorization", fmt.Sprintf("Snowflake Token=\"%s\"", snowflakeToken))
	} else {
		// If the authorization header hasn't been set by us remove it as only we know the session token.
		reqCopy.Header.Del("Authorization")
	}
}

func (e *Engine) processResponse(resp *http.Response, modifyReqFn func(body []byte) ([]byte, error)) error {
	// Process response only if successful. Send to the client otherwise.
	if resp.StatusCode == http.StatusOK {
		body, err := readResponseBody(resp)
		if err != nil {
			return trace.Wrap(err)
		}

		newPayload, err := modifyReqFn(body)
		if err != nil {
			return trace.Wrap(err)
		}

		buf, err := writeResponse(resp, newPayload)
		if err != nil {
			return trace.Wrap(err)
		}

		resp.Body = io.NopCloser(buf)
		resp.ContentLength = int64(buf.Len())
	}

	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// authorizeConnection does authorization check for Snowflake connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	state := e.sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     e.sessionCtx.Database,
		DatabaseUser: e.sessionCtx.DatabaseUser,
		DatabaseName: e.sessionCtx.DatabaseName,
	})
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) processRequest(ctx context.Context, req *http.Request) (io.Reader, error) {
	var (
		newBody io.Reader
		err     error
	)

	switch req.URL.Path {
	case loginRequestPath:
		newBody, err = e.handleLoginRequest(ctx, req)
	case queryRequestPath:
		newBody, err = e.handleQueryRequest(ctx, req)
	case tokenRequestPath:
		newBody, err = e.handleTokenRequest(ctx, req)
	default:
		newBody = req.Body
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newBody, nil
}

func (e *Engine) handleTokenRequest(ctx context.Context, req *http.Request) (io.Reader, error) {
	return e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
		refreshReq := &renewSessionRequest{}
		if err := json.Unmarshal(body, &refreshReq); err != nil {
			return nil, trace.Wrap(err)
		}

		sessionToken := strings.TrimPrefix(refreshReq.OldSessionToken, teleportAuthHeaderPrefix)

		var err error
		refreshReq.OldSessionToken, err = e.getSnowflakeToken(ctx, sessionToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		newData, err := json.Marshal(refreshReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return newData, nil
	})
}

func (e *Engine) handleLoginRequest(ctx context.Context, req *http.Request) (io.Reader, error) {
	return e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
		jwtToken, err := e.AuthClient.GenerateSnowflakeJWT(ctx, types.GenerateSnowflakeJWT{
			Username: e.sessionCtx.DatabaseUser,
			Account:  e.accountName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		newLoginPayload, err := replaceLoginReqToken(body, jwtToken, e.accountName, e.sessionCtx.DatabaseUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return newLoginPayload, nil
	})
}

// processLoginResponse extracts session and master token from /queries/v1/query-request and store in auth server as
// web session. Session ID replaces the session/master token returned to the Snowflake client, so only Teleport
// has access to the Snowflake access tokens.
func (e *Engine) processLoginResponse(bodyBytes []byte, createSessionFn func(tokens sessionTokens) (string, string, error)) ([]byte, error) {
	loginResp := &loginResponse{}
	if err := json.Unmarshal(bodyBytes, loginResp); err != nil {
		return nil, trace.Wrap(err)
	}

	if !loginResp.Success {
		e.Log.Errorf("Snowflake authentication failed: %s", loginResp.Message)
		// Return not modified response, so client can handle it. Otherwise, the client my keep retrying where
		// most likely each response will return the same error (invalid JWT when user doesn't exist for ex.)
		return bodyBytes, nil
	}

	tokens, err := loginResp.checkAndGetTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionToken, masterToken, err := createSessionFn(tokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// reset tokens to prevent from endlessly growing map. Very unlikely, but you never know.
	e.tokens.reset()
	e.tokens.setToken(sessionToken, tokens.session.token)
	e.tokens.setToken(masterToken, tokens.master.token)

	// Add Teleport: prefix to the authentication header, so we know that the header
	// was set by us.
	loginResp.Data.Token = teleportAuthHeaderPrefix + sessionToken
	loginResp.Data.MasterToken = teleportAuthHeaderPrefix + masterToken

	newResp, err := json.Marshal(loginResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newResp, err
}

func (e *Engine) handleQueryRequest(ctx context.Context, req *http.Request) (io.Reader, error) {
	return e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
		query, err := extractSQLStmt(body)
		if err != nil {
			return nil, trace.Wrap(err, "failed to extract SQL query")
		}

		e.Audit.OnQuery(ctx, e.sessionCtx, common.Query{
			Query:      query.SQLText,
			Parameters: query.paramsToSlice(),
		})

		return body, nil
	})
}

func (e *Engine) getConnectionToken(ctx context.Context, req *http.Request) (string, error) {
	sessionToken := extractSnowflakeToken(req.Header)

	// Authentication header always starts with "Teleport:", so we know that the token was set by us.
	// Some SDK set this header to a "random" value (Python SDK sends None for example).
	if !strings.Contains(sessionToken, teleportAuthHeaderPrefix) {
		return "", nil
	}

	sessionToken = strings.TrimPrefix(sessionToken, teleportAuthHeaderPrefix)

	snowflakeToken, err := e.getSnowflakeToken(ctx, sessionToken)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return snowflakeToken, nil
}

func (e *Engine) modifyRequestBody(req *http.Request, modifyReqFn func(body []byte) ([]byte, error)) (*bytes.Buffer, error) {
	if req.Method != http.MethodPost {
		return nil, trace.Errorf("unexpected request method, expected POST called %q", req.Method)
	}

	body, err := readRequestBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	body, err = modifyReqFn(body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	buf := &bytes.Buffer{}
	if req.Header.Get("Content-Encoding") == "gzip" {
		newGzBody := gzip.NewWriter(buf)

		if _, err := newGzBody.Write(body); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := newGzBody.Close(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		buf.Write(body)
	}

	// Update Content-Length header as the modified payload may have a different length.
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	return buf, nil
}

// getSnowflakeToken returns the Snowflake token from local in memory cache or from auth server if local cache doesn't have it.
// This function may time out if auth server doesn't respond in reasonable time.
func (e *Engine) getSnowflakeToken(ctx context.Context, sessionToken string) (string, error) {
	snowflakeToken := e.tokens.getToken(sessionToken)
	if snowflakeToken != "" {
		return snowflakeToken, nil
	}

	// Fetch the token from the auth server if not found in the local cache.
	if err := authclient.WaitForSnowflakeSession(ctx, sessionToken, e.sessionCtx.Identity.Username, e.AuthClient); err != nil {
		return "", trace.Wrap(err)
	}

	snowflakeSession, err := e.AuthClient.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: sessionToken})
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Add token to the local cache, so we don't need to fetch it every time.
	e.tokens.setToken(sessionToken, snowflakeSession.GetBearerToken())
	return snowflakeSession.GetBearerToken(), nil
}

// parseConnectionString extracts account name from provided Snowflake URL
// ref: https://docs.snowflake.com/en/user-guide/admin-account-identifier.html
func parseConnectionString(uri string) (string, string, error) {
	if !strings.Contains(uri, defaults.SnowflakeURL) {
		return "", "", trace.BadParameter("Snowflake address should contain " + defaults.SnowflakeURL)
	}

	// if the protocol is missing add it, so we can parse it.
	if !strings.Contains(uri, "://") {
		uri = "https://" + uri
	}

	snowflakeURL, err := url.Parse(uri)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	query := snowflakeURL.Query()
	// Read the account name from account query if provided. This should help with some Snowflake corner cases
	// where the account URL is in different format.
	if query.Has("account") {
		return query.Get("account"), snowflakeURL.Host, nil
	}

	uriParts := strings.Split(snowflakeURL.Host, ".")

	switch len(uriParts) {
	case 3:
		// address in https://test.snowflakecomputing.com format
		return uriParts[0], snowflakeURL.Host, nil
	case 4:
		// address in https://test.eu-central-1.snowflakecomputing.com format
		return strings.Join(uriParts[:2], "."), snowflakeURL.Host, nil
	case 5:
		// address in https://test.us-east-2.aws.snowflakecomputing.com format
		return strings.Join(uriParts[:3], "."), snowflakeURL.Host, nil
	default:
		return "", "", trace.BadParameter("invalid Snowflake url: %s", uri)
	}
}

func extractSnowflakeToken(headers http.Header) string {
	const (
		tokenPrefix = `Snowflake Token="`
		tokenSuffix = `"`
	)

	token := headers.Get("Authorization")

	if len(token) > len(tokenPrefix)+len(tokenSuffix) &&
		strings.HasPrefix(token, tokenPrefix) && strings.HasSuffix(token, tokenSuffix) {
		return token[len(tokenPrefix) : len(token)-len(tokenSuffix)]
	}

	return ""
}

// replaceLoginReqToken modifies the login request sent by Snowflake client with Teleports credentials.
func replaceLoginReqToken(loginReq []byte, jwtToken, accountName, dbUser string) ([]byte, error) {
	logReq := &loginRequest{}
	if err := json.Unmarshal(loginReq, logReq); err != nil {
		return nil, trace.Wrap(err)
	}

	// Always use JWT authentication.
	logReq.Data.Token = jwtToken
	logReq.Data.AccountName = accountName
	logReq.Data.LoginName = dbUser
	logReq.Data.Authenticator = "SNOWFLAKE_JWT"

	// Erase other authentication methods as we're using JWT.
	logReq.Data.Password = ""
	logReq.Data.ExtAuthnDuoMethod = ""
	logReq.Data.RawSAMLResponse = ""
	logReq.Data.Passcode = ""
	logReq.Data.BrowserModeRedirectPort = ""
	logReq.Data.ProofKey = ""

	resp, err := json.Marshal(logReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func extractSQLStmt(body []byte) (*queryRequest, error) {
	queryReq := &queryRequest{}
	if err := json.Unmarshal(body, queryReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return queryReq, nil
}

type tokenTTL struct {
	token string
	ttl   time.Duration
}

type sessionTokens struct {
	session tokenTTL
	master  tokenTTL
}
