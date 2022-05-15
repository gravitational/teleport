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
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolSnowflake)
}

// newEngine create new Redis engine.
func newEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
		HTTPClient:   getDefaultHTTPClient(),
	}
}

// getDefaultHTTPClient returns default HTTP client used by the Snowflake engine.
func getDefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
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
}

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
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

	jsonBody := fmt.Sprintf(`{"success": false, "message:"%s"}`, err.Error())

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
		Header: map[string][]string{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonBody))},
		},
	}

	dumpResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		e.Log.Errorf("snowflake error: %+v", trace.Unwrap(err))
		return
	}

	_, err = e.clientConn.Write(dumpResponse)
	if err != nil {
		e.Log.Errorf("snowflake error: %+v", trace.Unwrap(err))
		return
	}
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	accountName, err := extractAccountName(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := e.authorizeConnection(ctx); err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		err = e.processRequest(ctx, sessionCtx, req, accountName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) processRequest(ctx context.Context, sessionCtx *common.Session, req *http.Request, accountName string) error {
	snowflakeToken, err := e.getConnectionToken(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	requestBodyReader, err := e.process(ctx, req, accountName)
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy, err := copyRequest(ctx, req, requestBodyReader)
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = sessionCtx.Database.GetURI()

	e.setAuthorizationHeader(reqCopy, snowflakeToken)

	// Send the request to Snowflake API
	resp, err := e.HTTPClient.Do(reqCopy)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch req.URL.Path {
	case loginRequestPath:
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

		// Return here - processLoginResponse sends the response.
		return trace.Wrap(err)
	case tokenRequestPath:
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

			e.tokens.setSessionToken(snowflakeSession.GetName(), renewSessResp.Data.SessionToken)
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

				e.tokens.setMasterToken(masterToken.GetName(), renewSessResp.Data.MasterToken)
				renewSessResp.Data.MasterToken = teleportAuthHeaderPrefix + masterToken.GetName()
			}

			newBody, err := json.Marshal(renewSessResp)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return newBody, nil
		})

		// Return here - processLoginResponse sends the response.
		return trace.Wrap(err)
	}

	return trace.Wrap(e.sendResponse(resp))
}

func (e *Engine) sendResponse(resp *http.Response) error {
	dumpResp, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = e.clientConn.Write(dumpResp)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := io.Copy(e.clientConn, resp.Body); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) setAuthorizationHeader(reqCopy *http.Request, snowflakeToken string) {
	if snowflakeToken != "" {
		reqCopy.Header.Set("Authorization", fmt.Sprintf("Snowflake Token=\"%s\"", snowflakeToken))
	} else {
		reqCopy.Header.Del("Authorization")
	}
}

func (e *Engine) processResponse(resp *http.Response, modifyReqFn func(body []byte) ([]byte, error)) error {
	dumpResp, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return trace.Wrap(err)
	}

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

		dumpResp, err = copyResponse(resp, buf.Bytes())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	_, err = e.clientConn.Write(dumpResp)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// authorizeConnection does authorization check for Snowflake connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	mfaParams := services.AccessMFAParams{
		Verified:       e.sessionCtx.Identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}

	dbRoleMatchers := role.DatabaseRoleMatchers(
		e.sessionCtx.Database.GetProtocol(),
		e.sessionCtx.DatabaseUser,
		e.sessionCtx.DatabaseName,
	)
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		mfaParams,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) process(ctx context.Context, req *http.Request, accountName string) (io.Reader, error) {
	var (
		newBody io.Reader
		err     error
	)

	switch req.URL.Path {
	case loginRequestPath:
		newBody, err = e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
			jwtToken, err := e.AuthClient.GenerateSnowflakeJWT(ctx, types.GenerateSnowflakeJWT{
				Username: e.sessionCtx.DatabaseUser,
				Account:  accountName,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			newLoginPayload, err := replaceLoginReqToken(body, jwtToken, accountName)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return newLoginPayload, nil
		})

		if err != nil {
			return nil, trace.Wrap(err)
		}
	case queryRequestPath:
		newBody, err = e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
			query, err := extractSQLStmt(body)
			if err != nil {
				return nil, trace.Wrap(err, "failed to extract SQL query")
			}

			e.Audit.OnQuery(ctx, e.sessionCtx, common.Query{Query: query})

			return body, nil
		})

		if err != nil {
			return nil, trace.Wrap(err)
		}
	case tokenRequestPath:
		newBody, err = e.modifyRequestBody(req, func(body []byte) ([]byte, error) {
			refreshReq := &renewSessionRequest{}
			if err := json.Unmarshal(body, &refreshReq); err != nil {
				return nil, trace.Wrap(err)
			}

			sessionToken := strings.TrimPrefix(refreshReq.OldSessionToken, teleportAuthHeaderPrefix)
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

		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		newBody = req.Body
	}

	return newBody, nil
}

func (e *Engine) modifyRequestBody(req *http.Request, modifyReqFn func(body []byte) ([]byte, error)) (*bytes.Buffer, error) {
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

	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	return buf, nil
}

func (e *Engine) getConnectionToken(ctx context.Context, req *http.Request) (string, error) {
	sessionToken := extractSnowflakeToken(req.Header)

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

func (e *Engine) getSnowflakeToken(ctx context.Context, sessionToken string) (string, error) {
	snowflakeToken := e.tokens.getSessionToken(sessionToken)
	if snowflakeToken != "" {
		return snowflakeToken, nil
	}

	// Fetch the token from the auth server if not found in the local cache.
	if err := auth.WaitForSnowflakeSession(ctx, sessionToken, e.sessionCtx.Identity.Username, e.AuthClient); err != nil {
		return "", trace.Wrap(err)
	}

	snowflakeSession, err := e.AuthClient.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{
		SessionID: sessionToken,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Add token to the local cache, so we don't need to fetch it every time.
	e.tokens.setSessionToken(sessionToken, snowflakeSession.GetBearerToken())
	return snowflakeSession.GetBearerToken(), nil
}

func (e *Engine) processLoginResponse(bodyBytes []byte, createSessionFn func(tokens sessionTokens) (string, string, error)) ([]byte, error) {
	loginResp := &loginResponse{}
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(loginResp); err != nil {
		return nil, trace.Wrap(err)
	}

	if !loginResp.Success {
		return nil, trace.Errorf("Snowflake authentication failed: %s", loginResp.Message)
	}

	tokens, err := loginResp.getTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionToken, masterToken, err := createSessionFn(tokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.tokens.setSessionToken(sessionToken, tokens.session.token)
	e.tokens.setMasterToken(masterToken, tokens.master.token)

	loginResp.Data["token"] = teleportAuthHeaderPrefix + sessionToken
	loginResp.Data["masterToken"] = teleportAuthHeaderPrefix + masterToken

	newResp, err := json.Marshal(loginResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newResp, err
}

// extractAccountName extracts account name from provided Snowflake URL
// ref: https://docs.snowflake.com/en/user-guide/admin-account-identifier.html
func extractAccountName(uri string) (string, error) {
	if !strings.Contains(uri, "snowflakecomputing.com") {
		return "", trace.BadParameter("Snowflake address should contain snowflakecomputing.com")
	}

	if strings.HasPrefix(uri, "https://") {
		uri = strings.TrimSuffix(uri, "https://")
	}

	uriParts := strings.Split(uri, ".")

	switch len(uriParts) {
	case 3:
		// address in https://test.snowflakecomputing.com format
		return uriParts[0], nil
	case 5:
		// address in https://test.us-east-2.aws.snowflakecomputing.com format
		return strings.Join(uriParts[:3], "."), nil
	default:
		return "", trace.BadParameter("invalid Snowflake url: %s", uri)
	}
}

func extractSnowflakeToken(headers http.Header) string {
	const (
		tokenPrefix = "Snowflake Token=\""
		tokenSuffix = "\""
	)

	token := headers.Get("Authorization")

	if len(token) > len(tokenPrefix)+len(tokenSuffix) &&
		strings.HasPrefix(token, tokenPrefix) && strings.HasSuffix(token, tokenSuffix) {
		return token[len(tokenPrefix) : len(token)-len(tokenSuffix)]
	}

	return ""
}

type tokenTTL struct {
	token string
	ttl   time.Duration
}

type sessionTokens struct {
	session tokenTTL
	master  tokenTTL
}

func extractSQLStmt(body []byte) (string, error) {
	queryRequest := &queryRequest{}
	if err := json.Unmarshal(body, queryRequest); err != nil {
		return "", trace.Wrap(err)
	}

	return queryRequest.SQLText, nil
}

func replaceLoginReqToken(loginReq []byte, jwtToken string, accountName string) ([]byte, error) {
	logReq := &loginRequest{}
	if err := json.Unmarshal(loginReq, logReq); err != nil {
		return nil, trace.Wrap(err)
	}

	logReq.Data.Token = jwtToken
	logReq.Data.AccountName = accountName
	logReq.Data.Authenticator = "SNOWFLAKE_JWT"

	// Erase other authentication methods as we're using JWT method
	//logReq.Data.LoginName = "" TODO(jakule)
	logReq.Data.Password = ""
	logReq.Data.ExtAuthnDuoMethod = ""

	return json.Marshal(logReq)
}
