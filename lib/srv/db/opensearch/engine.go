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

package opensearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	libaws "github.com/gravitational/teleport/lib/utils/aws"
)

// NewEngine create new OpenSearch engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine handles connections from OpenSearch clients coming from Teleport
// proxy over reverse tunnel.
type Engine struct {
	// signingSvc will be used by the engine to provide the AWS sigv4 authorization header
	// required by AWS for request validation: https://docs.aws.amazon.com/general/latest/gr/signing-elements.html
	signingSvc *libaws.SigningService

	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// GetSigningCredsFn allows to set the function responsible for obtaining STS credentials.
	// Used in tests to set static AWS credentials and skip API call.
	GetSigningCredsFn libaws.GetSigningCredentialsFunc
}

// InitializeConnection initializes the engine with the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx

	awsSession, err := e.CloudClients.GetAWSSession(sessionCtx.Database.GetAWS().Region)
	if err != nil {
		return trace.Wrap(err)
	}

	svc, err := libaws.NewSigningService(libaws.SigningServiceConfig{
		Clock:                 e.Clock,
		Session:               awsSession,
		GetSigningCredentials: e.GetSigningCredsFn,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	e.signingSvc = svc

	return nil
}

// ErrorDetails contains error details.
type ErrorDetails struct {
	Reason string `json:"reason"`
	Type   string `json:"type"`
}

// ErrorResponse will be returned to the client in case of error.
type ErrorResponse struct {
	Error  ErrorDetails `json:"error"`
	Status int          `json:"status"`
}

// SendError sends an error to OpenSearch client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}

	cause := ErrorResponse{
		Error: ErrorDetails{
			Reason: err.Error(),
			Type:   "internal_server_error_exception",
		},
		Status: http.StatusInternalServerError,
	}

	// Different error for access denied case.
	if trace.IsAccessDenied(err) {
		cause.Status = http.StatusUnauthorized
		cause.Error.Type = "access_denied_exception"
	}

	jsonBody, err := json.Marshal(cause)
	if err != nil {
		e.Log.WithError(err).Error("failed to marshal error response")
		return
	}

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: cause.Status,
		Body:       io.NopCloser(bytes.NewBuffer(jsonBody)),
		Header: map[string][]string{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonBody))},
		},
	}

	if err := response.Write(e.clientConn); err != nil {
		e.Log.Errorf("OpenSearch error: %+v", trace.Unwrap(err))
		return
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target OpenSearch server and starts proxying requests between client/server.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	err := e.checkAccess(ctx)
	e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
	if err != nil {
		return trace.Wrap(err)
	}
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)
	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := e.process(ctx, req); err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected OpenSearch client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, req *http.Request) error {
	reqCopy, _, err := utils.CloneRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// force HTTPS, set host URL.
	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = e.sessionCtx.Database.GetURI()

	e.emitAuditEvent(reqCopy)

	roleArn, err := libaws.BuildRoleARN(e.sessionCtx.DatabaseUser, e.sessionCtx.Database.GetAWS().Region, e.sessionCtx.Database.GetAWS().AccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	signCtx := &libaws.SigningCtx{
		SigningName:   opensearchservice.EndpointsID,
		SigningRegion: e.sessionCtx.Database.GetAWS().Region,
		Expiry:        e.sessionCtx.Identity.Expires,
		SessionName:   e.sessionCtx.Identity.Username,
		AWSRoleArn:    roleArn,
		AWSExternalID: e.sessionCtx.Database.GetAWS().ExternalID,
	}

	signedReq, err := e.signingSvc.SignRequest(e.Context, reqCopy, signCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	clt := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Send the request to elasticsearch API
	resp, err := clt.Do(signedReq)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	return trace.Wrap(e.sendResponse(resp))
}

// emitAuditEvent writes the request and response to audit stream.
func (e *Engine) emitAuditEvent(req *http.Request) {
	// Try to read the body and JSON unmarshal it.
	// If this fails, we still want to emit the rest of the event info; the request event Body is nullable,
	// so it's ok if body is nil here.
	body, err := libaws.UnmarshalRequestBody(req)
	if err != nil {
		e.Log.WithError(err).Warn("Failed to read request body as JSON, omitting the body from the audit event.")
	}

	ev := &apievents.OpenSearchRequest{
		Metadata: common.MakeEventMetadata(e.sessionCtx,
			events.DatabaseSessionOpenSearchRequestEvent,
			events.OpenSearchRequestCode),
		UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
		Method:           req.Method,
		Path:             req.URL.Path,
		RawQuery:         req.URL.RawQuery,
		Body:             body,
	}

	e.Audit.EmitEvent(req.Context(), ev)
}

// sendResponse sends the response back to the OpenSearch client.
func (e *Engine) sendResponse(resp *http.Response) error {
	// calculate content length if missing.
	if resp.ContentLength == -1 {
		respBody, err := utils.GetAndReplaceResponseBody(resp)
		if err != nil {
			return trace.Wrap(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		resp.ContentLength = int64(len(respBody))
	}

	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkAccess does authorization check for OpenSearch connection about
// to be established.
func (e *Engine) checkAccess(ctx context.Context) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := e.sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.DatabaseRoleMatchers(
		e.sessionCtx.Database,
		e.sessionCtx.DatabaseUser,
		e.sessionCtx.DatabaseName,
	)
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)

	if e.sessionCtx.Identity.RouteToDatabase.Username == "" {
		return trace.BadParameter("database username required for OpenSearch")
	}

	return trace.Wrap(err)
}
