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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	e.Log.Errorf("snowflake error: %+v", err)
}

const (
	certPath    = "/Users/jnyckowski/projects/certs/example.com+5.pem"
	privKeyPath = "/Users/jnyckowski/projects/certs/example.com+5-key.pem"
)

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	uri := sessionCtx.Database.GetURI()
	uriParts := strings.Split(uri, ".")
	if len(uriParts) != 5 {
		return trace.BadParameter("invalid Snowflake url: %s", uri)
	}

	accountName := uriParts[0]

	jwtToken, err := e.AuthClient.GenerateDatabaseJWT(ctx, types.GenerateSnowflakeJWT{
		Username: sessionCtx.DatabaseUser,
		Account:  accountName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	e.Log.Debugf("JWT token: %s", jwtToken)

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
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

		e.Log.Debugf("orig url: %s", req.URL.String())
		origURLPath := req.URL.String()

		// TODO(jakule): Add limiter
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return trace.Wrap(err)
		}

		if req.Method == http.MethodConnect {
			fmt.Println("CONNECT message")
			if _, err := e.clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
				e.Log.Println(err)
				break
			}

			cert, err := tls.LoadX509KeyPair(certPath, privKeyPath)
			if err != nil {
				e.Log.Fatal(err)
			}

			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{cert},
				ServerName:   "example.com",
			}

			tlsConn := tls.Server(e.clientConn, tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				e.Log.Printf("handshake error: %v", err)
				break
			}
			e.Log.Println("handshake success")

			e.clientConn = tlsConn
			continue
		}

		if req.Header.Get("Content-Encoding") == "gzip" {
			bodyGZ, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				return trace.Wrap(err)
			}

			body, err = io.ReadAll(bodyGZ)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		e.Log.Debugf("%s", string(body))

		if strings.Contains(req.Header.Get("Authorization"), "Snowflake Token") &&
			!strings.Contains(req.Header.Get("Authorization"), "None") {
			e.connectionToken = req.Header.Get("Authorization")
		}

		if req.Header.Get("Authorization") == "Basic" {
			req.Header.Del("Authorization")
		}

		switch {
		case strings.HasPrefix(origURLPath, loginRequestPath):
			if newBody, err := replaceToken(body, jwtToken, accountName); err == nil {
				e.Log.Debugf("new body: %s", string(newBody))

				body = newBody
			} else {
				e.Log.Errorf("failed to unmarshal login JSON: %v", err)
			}

		case strings.HasPrefix(origURLPath, queryRequestPath):
			query, err := extractSQLStmt(body)
			if err != nil {
				return trace.Wrap(err, "failed to extract SQL query")
			}
			// TODO(jakule): Add request ID??
			e.Audit.OnQuery(ctx, sessionCtx, common.Query{Query: query})
		}

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

		e.Log.Debugf("newbody size: %d", buf.Len())

		body = buf.Bytes()

		req.URL.Scheme = "https"
		req.URL.Host = sessionCtx.Database.GetURI()
		newUrl := req.URL.String()
		e.Log.Debugf("new url: %s", newUrl)
		e.Log.Debugf("method: %s", req.Method)

		reqCopy, err := http.NewRequestWithContext(ctx, req.Method, newUrl, bytes.NewReader(body))
		if err != nil {
			return trace.Wrap(err)
		}

		for k, v := range req.Header {
			if reqCopy.Header.Get(k) != "" {
				continue
			}
			e.Log.Debugf("setting header %s: %s", k, strings.Join(v, ","))
			reqCopy.Header.Set(k, strings.Join(v, ","))
		}

		reqCopy.Header.Set("Content-Length", strconv.Itoa(len(body)))

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

		//e.Log.Debugf("resp: %s", string(dumpResp))

		if resp.StatusCode != 200 {
			e.Log.Warnf("Not 200 response code: %d", resp.StatusCode)
		}

		if strings.HasPrefix(origURLPath, loginRequestPath) {
			if err := e.extractToken(dumpResp); err == nil {
				e.Log.Debugf("extracted token")
			} else {
				e.Log.Debugf("failed to extract token: %v", err)
			}
		}

		_, err = e.clientConn.Write(dumpResp)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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

func (e *Engine) extractToken(dumpResp []byte) error {
	respBytes := bytes.NewReader(dumpResp)
	respBufio := bufio.NewReader(respBytes)

	resp, err := http.ReadResponse(respBufio, nil)
	if err != nil {
		return err
	}

	gzBody, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(gzBody)
	if err != nil {
		return err
	}

	loginResp := &LoginResponse{}
	if err := json.Unmarshal(bodyBytes, loginResp); err != nil {
		return trace.Wrap(err)
	}

	e.connectionToken = loginResp.Data.Token

	return nil
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
	Data struct {
		MasterToken             string      `json:"masterToken"`
		Token                   string      `json:"token"`
		ValidityInSeconds       int         `json:"validityInSeconds"`
		MasterValidityInSeconds int         `json:"masterValidityInSeconds"`
		DisplayUserName         string      `json:"displayUserName"`
		ServerVersion           string      `json:"serverVersion"`
		FirstLogin              bool        `json:"firstLogin"`
		RemMeToken              interface{} `json:"remMeToken"`
		RemMeValidityInSeconds  int         `json:"remMeValidityInSeconds"`
		HealthCheckInterval     int         `json:"healthCheckInterval"`
		NewClientForUpgrade     interface{} `json:"newClientForUpgrade"`
		SessionID               int64       `json:"sessionId"`
		//Parameters              []struct {
		//	Name  string `json:"name"`
		//	Value int    `json:"value"`
		//} `json:"parameters"`
		SessionInfo struct {
			DatabaseName  interface{} `json:"databaseName"`
			SchemaName    interface{} `json:"schemaName"`
			WarehouseName interface{} `json:"warehouseName"`
			RoleName      string      `json:"roleName"`
		} `json:"sessionInfo"`
		IDToken                   interface{} `json:"idToken"`
		IDTokenValidityInSeconds  int         `json:"idTokenValidityInSeconds"`
		ResponseData              interface{} `json:"responseData"`
		MfaToken                  interface{} `json:"mfaToken"`
		MfaTokenValidityInSeconds int         `json:"mfaTokenValidityInSeconds"`
	} `json:"data"`
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
