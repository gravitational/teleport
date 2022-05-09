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
	}
}

type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session

	connectionToken string
}

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx

	return nil
}

func (e *Engine) SendError(err error) {
	if err == nil || utils.IsOKNetworkError(err) {
		return
	}

	//TODO(jakule): implement
	e.Log.Errorf("snowflake error: %+v", trace.Unwrap(err))

	if e.clientConn == nil {
		return
	}

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: 401,
		Body:       io.NopCloser(bytes.NewBufferString(fmt.Sprintf(`{"success": false, "message:"%s"}`, err.Error()))),
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
	uri := sessionCtx.Database.GetURI()
	uriParts := strings.Split(uri, ".")
	if len(uriParts) != 5 && len(uriParts) != 3 && !strings.Contains(uri, "localhost") {
		return trace.BadParameter("invalid Snowflake url: %s", uri)
	}

	accountName := uriParts[0]

	err := e.authorizeConnection(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
	}

	clientConnReader := bufio.NewReader(e.clientConn)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		e.Log.Debugf("%+v", req)

		origURLPath := req.URL.String()
		e.Log.Debugf("orig url: %s", req.URL.String())

		body, err := readRequest(req)
		if err != nil {
			return trace.Wrap(err)
		}

		e.Log.Debugf("%s", string(body))

		e.connectionToken, err = e.getConnectionToken(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		body, err = e.process(ctx, sessionCtx, origURLPath, accountName, body)
		if err != nil {
			return trace.Wrap(err)
		}

		buf := e.writeResponse(req, body)

		e.Log.Debugf("newbody size: %d", buf.Len())

		body = buf.Bytes()
		req.URL.Scheme = "https"
		req.URL.Host = sessionCtx.Database.GetURI()
		newUrl := req.URL.String()
		e.Log.Debugf("new url: %s", newUrl)
		e.Log.Debugf("method: %s", req.Method)

		reqCopy, err := e.copyRequest(ctx, req, newUrl, body)
		if err != nil {
			return trace.Wrap(err)
		}

		if e.connectionToken != "" {
			e.Log.Debugf("setting Snowflake token %s", e.connectionToken)
			if strings.Contains(e.connectionToken, "Snowflake Token") {
				reqCopy.Header.Set("Authorization", e.connectionToken)
			} else {
				reqCopy.Header.Set("Authorization", fmt.Sprintf("Snowflake Token=\"%s\"", e.connectionToken))
			}
		}

		resp, err := httpClient.Do(reqCopy)
		if err != nil {
			return trace.Wrap(err)
		}

		dumpResp, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return trace.Wrap(err)
		}

		e.printBody(resp, resp.Body)

		if resp.StatusCode != 200 {
			e.Log.Warnf("Not 200 response code: %d", resp.StatusCode)
		}

		if strings.HasPrefix(origURLPath, loginRequestPath) {
			dumpResp, err = e.saveSessionToken(ctx, sessionCtx, dumpResp, accountName)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		_, err = e.clientConn.Write(dumpResp)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// authorizeConnection does authorization check for Redis connection about
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

func (e *Engine) writeResponse(req *http.Request, body []byte) *bytes.Buffer {
	buf := &bytes.Buffer{}
	var wr io.WriteCloser
	if req.Header.Get("Content-Encoding") == "gzip" {
		wr = gzip.NewWriter(buf)
	} else {
		wr = &MyWriteCloser{buf}
	}

	if _, err := wr.Write(body); err != nil {
		e.Log.Error(err)
	}

	if err := wr.Close(); err != nil {
		e.Log.Error(err)
	}
	return buf
}

func (e *Engine) printBody(resp *http.Response, body io.ReadCloser) {
	var bodyReader io.Reader
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(body)
		if err != nil {
			panic(err)
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	} else {
		bodyReader = resp.Body
	}

	body23, err := io.ReadAll(bodyReader)
	if err != nil {
		panic(err)
	}

	e.Log.Debugf("response body: %s", string(body23))
}

func (e *Engine) saveSessionToken(ctx context.Context, sessionCtx *common.Session, dumpResp []byte, accountName string) ([]byte, error) {
	if newResp, err := e.extractToken(dumpResp, func(sessionToken string) (string, error) {
		snowflakeSession, err := e.AuthClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
			Username:             sessionCtx.Identity.Username,
			SnowflakeAccountName: accountName,
			SnowflakeUsername:    sessionCtx.DatabaseUser,
			SessionToken:         e.connectionToken,
		})
		if err != nil {
			return "", trace.Wrap(err)
		}

		return snowflakeSession.GetName(), nil
	}); err == nil {
		e.Log.Debugf("extracted token")
		e.Log.Debugf("old resp: %s", string(dumpResp))
		e.Log.Debugf("new resp: %s", string(newResp))
		dumpResp = newResp
	} else {
		e.Log.Debugf("failed to extract token: %v", err)
		return nil, trace.Wrap(err)
	}

	return dumpResp, nil
}

func (e *Engine) copyRequest(ctx context.Context, req *http.Request, newUrl string, body []byte) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, req.Method, newUrl, bytes.NewReader(body))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for k, v := range req.Header {
		if reqCopy.Header.Get(k) != "" {
			continue
		}
		e.Log.Debugf("setting header %s: %s", k, strings.Join(v, ","))
		reqCopy.Header.Set(k, strings.Join(v, ","))
	}

	reqCopy.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return reqCopy, nil
}

func (e *Engine) process(ctx context.Context, sessionCtx *common.Session, origURLPath string, accountName string, body []byte) ([]byte, error) {
	switch {
	case strings.HasPrefix(origURLPath, loginRequestPath):
		jwtToken, err := e.AuthClient.GenerateDatabaseJWT(ctx, types.GenerateSnowflakeJWT{
			Username: sessionCtx.DatabaseUser,
			Account:  accountName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		e.Log.Debugf("JWT token: %s", jwtToken)

		if newBody, err := replaceToken(body, jwtToken, accountName); err == nil {
			e.Log.Debugf("new body: %s", string(newBody))

			body = newBody
		} else {
			e.Log.Errorf("failed to unmarshal login JSON: %v", err)
		}

	case strings.HasPrefix(origURLPath, queryRequestPath):
		query, err := extractSQLStmt(body)
		if err != nil {
			return nil, trace.Wrap(err, "failed to extract SQL query")
		}
		// TODO(jakule): Add request ID??
		e.Audit.OnQuery(ctx, sessionCtx, common.Query{Query: query})
	}
	return body, nil
}

func readRequest(req *http.Request) ([]byte, error) {
	// TODO(jakule): Add limiter
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Header.Get("Content-Encoding") == "gzip" {
		bodyGZ, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		body, err = io.ReadAll(bodyGZ)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return body, nil
}

func (e *Engine) getConnectionToken(ctx context.Context, req *http.Request) (string, error) {
	var connectionToken string

	if strings.Contains(req.Header.Get("Authorization"), "Snowflake Token") &&
		!strings.Contains(req.Header.Get("Authorization"), "None") {

		sessionID := req.Header.Get("Authorization")
		sessionID = strings.TrimPrefix(sessionID, "Snowflake Token=\"")
		sessionID = strings.TrimSuffix(sessionID, "\"")

		var (
			snowflakeSession types.WebSession
			err              error
		)
		for i := 0; i < 10; i++ {
			snowflakeSession, err = e.AuthClient.GetAppSession(ctx, types.GetAppSessionRequest{
				SessionID: sessionID,
			})
			if err != nil {
				if trace.IsNotFound(err) {
					e.Log.Debugf("not found %s, retry", sessionID)
					time.Sleep(time.Second)
					continue
				}
				return "", trace.Wrap(err)
			}

			connectionToken = snowflakeSession.GetBearerToken()
			break
		}

		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if req.Header.Get("Authorization") == "Basic" {
		req.Header.Del("Authorization")
	}

	return connectionToken, nil
}

func copyResponse(resp *http.Response, body []byte) ([]byte, error) {
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	resp.ContentLength = int64(len(body))
	if _, ok := resp.Header["Content-Length"]; ok {
		delete(resp.Header, "Content-Length")
	}
	return httputil.DumpResponse(resp, true)
}

type queryRequest struct {
	SQLText string `json:"sqlText"`
}

func extractSQLStmt(body []byte) (string, error) {
	queryRequest := &queryRequest{}
	if err := json.Unmarshal(body, queryRequest); err != nil {
		return "", trace.Wrap(err)
	}

	return queryRequest.SQLText, nil
}

func (e *Engine) extractToken(dumpResp []byte, sessCb func(string) (string, error)) ([]byte, error) {
	respBytes := bytes.NewReader(dumpResp)
	respBufio := bufio.NewReader(respBytes)

	resp, err := http.ReadResponse(respBufio, nil)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()

		bodyReader = gzipReader
	} else {
		bodyReader = resp.Body
	}

	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, err
	}

	loginResp := &LoginResponse{}
	if err := json.Unmarshal(bodyBytes, loginResp); err != nil {
		return nil, trace.Wrap(err)
	}

	if loginResp.Success == false {
		return nil, trace.Errorf("snowflake authentication failed: %s", loginResp.Message)
	}

	e.connectionToken = loginResp.Data["token"].(string)

	sessionToken, err := sessCb(e.connectionToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginResp.Data["token"] = sessionToken
	loginResp.Data["masterToken"] = sessionToken

	newResp, err := json.Marshal(loginResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	buf := &bytes.Buffer{}
	if resp.Header.Get("Content-Encoding") == "gzip" {
		newGzBody := gzip.NewWriter(buf)

		if _, err := newGzBody.Write(newResp); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := newGzBody.Close(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		buf.Write(newResp)
	}

	return copyResponse(resp, buf.Bytes())
}

func replaceToken(loginReq []byte, jwtToken string, accountName string) ([]byte, error) {
	logReq := &LoginRequest{}
	if err := json.Unmarshal(loginReq, logReq); err != nil {
		return nil, trace.Wrap(err)
	}

	logReq.Data["TOKEN"] = jwtToken
	logReq.Data["ACCOUNT_NAME"] = accountName
	logReq.Data["AUTHENTICATOR"] = "SNOWFLAKE_JWT"

	if _, ok := logReq.Data["PASSWORD"]; ok {
		delete(logReq.Data, "PASSWORD")
	}

	if _, ok := logReq.Data["EXT_AUTHN_DUO_METHOD"]; ok {
		delete(logReq.Data, "EXT_AUTHN_DUO_METHOD")
	}

	return json.Marshal(logReq)
}

type LoginResponse struct {
	Data map[string]interface{} `json:"data"`
	//Data struct {
	//	MasterToken             string      `json:"masterToken"`
	//	Token                   string      `json:"token"`
	//	ValidityInSeconds       int         `json:"validityInSeconds"`
	//	MasterValidityInSeconds int         `json:"masterValidityInSeconds"`
	//	DisplayUserName         string      `json:"displayUserName"`
	//	ServerVersion           string      `json:"serverVersion"`
	//	FirstLogin              bool        `json:"firstLogin"`
	//	RemMeToken              interface{} `json:"remMeToken"`
	//	RemMeValidityInSeconds  int         `json:"remMeValidityInSeconds"`
	//	HealthCheckInterval     int         `json:"healthCheckInterval"`
	//	NewClientForUpgrade     interface{} `json:"newClientForUpgrade"`
	//	SessionID               int64       `json:"sessionId"`
	//	//Parameters              []struct {
	//	//	Name  string `json:"name"`
	//	//	Value int    `json:"value"`
	//	//} `json:"parameters"`
	//	SessionInfo struct {
	//		DatabaseName  interface{} `json:"databaseName"`
	//		SchemaName    interface{} `json:"schemaName"`
	//		WarehouseName interface{} `json:"warehouseName"`
	//		RoleName      string      `json:"roleName"`
	//	} `json:"sessionInfo"`
	//	IDToken                   interface{} `json:"idToken"`
	//	IDTokenValidityInSeconds  int         `json:"idTokenValidityInSeconds"`
	//	ResponseData              interface{} `json:"responseData"`
	//	MfaToken                  interface{} `json:"mfaToken"`
	//	MfaTokenValidityInSeconds int         `json:"mfaTokenValidityInSeconds"`
	//} `json:"data"`
	Code    interface{} `json:"code"`
	Message interface{} `json:"message"`
	Success bool        `json:"success"`
}

type LoginRequest struct {
	//Data struct {
	//	ClientAppID       string      `json:"CLIENT_APP_ID"`
	//	ClientAppVersion  string      `json:"CLIENT_APP_VERSION"`
	//	SvnRevision       interface{} `json:"SVN_REVISION"`
	//	AccountName       string      `json:"ACCOUNT_NAME"`
	//	LoginName         string      `json:"LOGIN_NAME"`
	//	ClientEnvironment struct {
	//		Application    string      `json:"APPLICATION"`
	//		Os             string      `json:"OS"`
	//		OsVersion      string      `json:"OS_VERSION"`
	//		PythonVersion  string      `json:"PYTHON_VERSION"`
	//		PythonRuntime  string      `json:"PYTHON_RUNTIME"`
	//		PythonCompiler string      `json:"PYTHON_COMPILER"`
	//		OcspMode       string      `json:"OCSP_MODE"`
	//		Tracing        interface{} `json:"TRACING"`
	//		LoginTimeout   int         `json:"LOGIN_TIMEOUT"`
	//		NetworkTimeout interface{} `json:"NETWORK_TIMEOUT"`
	//	} `json:"CLIENT_ENVIRONMENT"`
	//	Authenticator     string `json:"AUTHENTICATOR"`
	//	Token             string `json:"TOKEN"`
	//	SessionParameters struct {
	//		AbortDetachedQuery     bool `json:"ABORT_DETACHED_QUERY"`
	//		Autocommit             bool `json:"AUTOCOMMIT"`
	//		ClientSessionKeepAlive bool `json:"CLIENT_SESSION_KEEP_ALIVE"`
	//		ClientPrefetchThreads  int  `json:"CLIENT_PREFETCH_THREADS"`
	//	} `json:"SESSION_PARAMETERS"`
	//} `json:"data"`
	Data map[string]interface{} `json:"data"`
}

type MyWriteCloser struct {
	io.Writer
}

func (mwc *MyWriteCloser) Close() error {
	return nil
}
