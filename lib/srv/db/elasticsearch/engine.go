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

package elasticsearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolElasticsearch)
}

// newEngine create new elasticsearch engine.
func newEngine(ec common.EngineConfig) common.Engine {
	return &Engine{EngineConfig: ec}
}

type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
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
		e.Log.Errorf("elasticsearch error: %+v", trace.Unwrap(err))
		return
	}
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	if err := e.authorizeConnection(ctx); err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	time.Sleep(time.Millisecond)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		err = e.process(ctx, sessionCtx, req)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func copyRequest(ctx context.Context, req *http.Request, body io.Reader) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.Header = req.Header.Clone()

	return reqCopy, nil
}

// process reads request from connected elasticsearch client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, sessionCtx *common.Session, req *http.Request) error {
	reqCopy, err := copyRequest(ctx, req, req.Body)
	if err != nil {
		return trace.Wrap(err)
	}

	// Force HTTPS usage even and update the host url.
	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = sessionCtx.Database.GetURI()

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Send the request to elasticsearch API
	resp, err := client.Do(reqCopy)

	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	return trace.Wrap(e.sendResponse(resp))
}

// sendResponse sends the response back to the elasticsearch client.
func (e *Engine) sendResponse(resp *http.Response) error {
	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// authorizeConnection does authorization check for elasticsearch connection about
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
