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
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	e.Log.Errorf("snowflake error: %+v", err)
}

func (e *Engine) HandleConnection(ctx context.Context, session *common.Session) error {
	uri := session.Database.GetURI()
	accountName := strings.Split(uri, ".")[0]

	jwtToken, err := e.AuthClient.GenerateDatabaseJWT(ctx, types.GenerateSnowflakeJWT{
		Username: session.DatabaseUser,
		Account:  accountName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	e.Log.Debugf("JWT token: %s", jwtToken)

	for {
		req, err := http.ReadRequest(bufio.NewReader(e.clientConn))
		if err != nil {
			return trace.Wrap(err)
		}

		e.Log.Debugf("%+v", req)

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

			const (
				certPath    = "/Users/jnyckowski/projects/certs/example.com+5.pem"
				privKeyPath = "/Users/jnyckowski/projects/certs/example.com+5-key.pem"
			)

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

		bodyGZ, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return trace.Wrap(err)
		}

		bodyGzAll, err := io.ReadAll(bodyGZ)
		if err != nil {
			return trace.Wrap(err)
		}

		e.Log.Debugf("%s", string(bodyGzAll))

		if e.connectionToken == "" {
			if newBody, err := replaceToken(bodyGzAll, jwtToken); err == nil {
				e.Log.Debugf("new body: %s", string(newBody))
				buf := &bytes.Buffer{}
				wr := gzip.NewWriter(buf)
				if _, err := wr.Write(newBody); err != nil {
					e.Log.Error(err)
				}

				if err := wr.Close(); err != nil {
					e.Log.Error(err)
				}

				e.Log.Debugf("newbody size: %d", buf.Len())

				body = buf.Bytes()
			}
		}

		req.URL.Scheme = "https"
		req.URL.Host = session.Database.GetURI()
		newUrl := req.URL.String()
		e.Log.Debugf("new url: %s", newUrl)

		reqCopy, err := http.NewRequest(req.Method, newUrl, bytes.NewReader(body))
		if err != nil {
			return trace.Wrap(err)
		}

		for k, v := range req.Header {
			if reqCopy.Header.Get(k) != "" {
				continue
			}
			reqCopy.Header.Set(k, strings.Join(v, ","))
		}

		if e.connectionToken != "" {
			e.Log.Debugf("setting Snowflake token %s", e.connectionToken)
			reqCopy.Header.Set("Authorization", fmt.Sprintf("Snowflake Token=\"%s\"", e.connectionToken))
		}

		c := http.Client{}
		resp, err := c.Do(reqCopy)
		if err != nil {
			return trace.Wrap(err)
		}

		dumpResp, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return trace.Wrap(err)
		}

		if e.connectionToken == "" {
			if err := e.extractToken(dumpResp); err == nil {
				e.Log.Debugf("extracted token")
			} else {
				e.Log.Debugf("failed to extract token: %v", err)
			}
		}

		e.Log.Debugf("resp: %s", string(dumpResp))

		_, err = e.clientConn.Write(dumpResp)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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

func replaceToken(loginReq []byte, jwtToken string) ([]byte, error) {
	logReq := &LoginRequest{}
	if err := json.Unmarshal(loginReq, logReq); err != nil {
		return nil, trace.Wrap(err)
	}

	logReq.Data.Token = jwtToken

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
	Data struct {
		ClientAppID       string      `json:"CLIENT_APP_ID"`
		ClientAppVersion  string      `json:"CLIENT_APP_VERSION"`
		SvnRevision       interface{} `json:"SVN_REVISION"`
		AccountName       string      `json:"ACCOUNT_NAME"`
		LoginName         string      `json:"LOGIN_NAME"`
		ClientEnvironment struct {
			Application    string      `json:"APPLICATION"`
			Os             string      `json:"OS"`
			OsVersion      string      `json:"OS_VERSION"`
			PythonVersion  string      `json:"PYTHON_VERSION"`
			PythonRuntime  string      `json:"PYTHON_RUNTIME"`
			PythonCompiler string      `json:"PYTHON_COMPILER"`
			OcspMode       string      `json:"OCSP_MODE"`
			Tracing        int         `json:"TRACING"`
			LoginTimeout   int         `json:"LOGIN_TIMEOUT"`
			NetworkTimeout interface{} `json:"NETWORK_TIMEOUT"`
		} `json:"CLIENT_ENVIRONMENT"`
		Authenticator     string `json:"AUTHENTICATOR"`
		Token             string `json:"TOKEN"`
		SessionParameters struct {
			AbortDetachedQuery     bool `json:"ABORT_DETACHED_QUERY"`
			Autocommit             bool `json:"AUTOCOMMIT"`
			ClientSessionKeepAlive bool `json:"CLIENT_SESSION_KEEP_ALIVE"`
			ClientPrefetchThreads  int  `json:"CLIENT_PREFETCH_THREADS"`
		} `json:"SESSION_PARAMETERS"`
	} `json:"data"`
}
