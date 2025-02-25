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

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/elasticsearch"
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
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
}

// InitializeConnection initializes the engine with the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	return nil
}

// errorDetails contains error details.
type errorDetails struct {
	Reason string `json:"reason"`
	Type   string `json:"type"`
}

// errorResponse will be returned to the client in case of error.
type errorResponse struct {
	Error  errorDetails `json:"error"`
	Status int          `json:"status"`
}

// SendError sends an error to OpenSearch client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}

	cause := errorResponse{
		Error: errorDetails{
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
		e.Log.ErrorContext(e.Context, "Failed to marshal error response.", "error", err)
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
		e.Log.ErrorContext(e.Context, "Failed to send an error to the client.", "error", err)
		return
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target OpenSearch server and starts proxying requests between client/server.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(e.sessionCtx.Database)

	err := e.checkAccess(ctx)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	// TODO(Tener):
	//  Consider rewriting to support HTTP2 clients.
	//  Ideally we should have shared middleware for DB clients using HTTP, instead of separate per-engine implementations.

	tr, err := e.getTransport(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, e.sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)

	observe()

	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := e.process(ctx, tr, req, msgFromClient, msgFromServer); err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected OpenSearch client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, tr *http.Transport, req *http.Request, msgFromClient prometheus.Counter, msgFromServer prometheus.Counter) error {
	msgFromClient.Inc()

	if req.Body != nil {
		// make sure we close the incoming request's body. ignore any close error.
		defer req.Body.Close()
		req.Body = io.NopCloser(utils.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	}
	reqCopy, payload, err := e.rewriteRequest(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	// emit an audit event regardless of failure
	var responseStatusCode uint32
	defer func() {
		e.emitAuditEvent(reqCopy, payload, responseStatusCode, err == nil)
	}()

	signedReq, err := e.getSignedRequest(reqCopy)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:bodyclose // resp will be closed in sendResponse().
	resp, err := tr.RoundTrip(signedReq)
	if err != nil {
		return trace.Wrap(err)
	}
	responseStatusCode = uint32(resp.StatusCode)

	msgFromServer.Inc()

	return trace.Wrap(e.sendResponse(resp))
}

func (e *Engine) getTransport(ctx context.Context) (*http.Transport, error) {
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx.GetExpiry(), e.sessionCtx.Database, e.sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tr.TLSClientConfig = tlsConfig
	return tr, nil
}

func (e *Engine) getSignedRequest(reqCopy *http.Request) (*http.Request, error) {
	roleArn, err := libaws.BuildRoleARN(e.sessionCtx.DatabaseUser, e.sessionCtx.Database.GetAWS().Region, e.sessionCtx.Database.GetAWS().AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := e.sessionCtx.Database.GetAWS()
	awsCfg, err := e.AWSConfigProvider.GetConfig(e.Context, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithDetailedAssumeRole(awsconfig.AssumeRole{
			RoleARN:     roleArn,
			ExternalID:  meta.ExternalID,
			SessionName: e.sessionCtx.Identity.Username,
		}),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signCtx := &libaws.SigningCtx{
		Clock:         e.Clock,
		Credentials:   awsCfg.Credentials,
		SigningName:   "es",
		SigningRegion: e.sessionCtx.Database.GetAWS().Region,
	}

	signedReq, err := libaws.SignRequest(e.Context, reqCopy, signCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return signedReq, nil
}

func (e *Engine) rewriteRequest(ctx context.Context, req *http.Request) (*http.Request, []byte, error) {
	payload, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	reqCopy := req.Clone(ctx)
	reqCopy.RequestURI = ""
	reqCopy.Body = io.NopCloser(bytes.NewReader(payload))

	// Connection is hop-by-hop header, drop.
	reqCopy.Header.Del("Connection")

	// force HTTPS, set host URL.
	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = e.sessionCtx.Database.GetURI()
	reqCopy.Host = e.sessionCtx.Database.GetURI()

	return reqCopy, payload, nil
}

// emitAuditEvent writes the request and response to audit stream.
func (e *Engine) emitAuditEvent(req *http.Request, body []byte, statusCode uint32, noErr bool) {
	var eventCode string
	if noErr && statusCode != 0 {
		eventCode = events.OpenSearchRequestCode
	} else {
		eventCode = events.OpenSearchRequestFailureCode
	}

	// Normally the query is passed as request body, and body content type as a header.
	// Yet it can also be passed as `source` and `source_content_type` URL params, and we handle that here.
	contentType := req.Header.Get("Content-Type")

	source := req.URL.Query().Get("source")
	if len(source) > 0 {
		e.Log.InfoContext(e.Context, "'source' parameter found, overriding request body.")
		body = []byte(source)
		contentType = req.URL.Query().Get("source_content_type")
	}

	target, category := parsePath(req.URL.Path)

	// Heuristic to calculate the query field.
	// The priority is given to 'q' URL param. If not found, we look at the request body.
	// This is not guaranteed to give us actual query, for example:
	// - we may not support given API
	// - we may not support given content encoding
	query := req.URL.Query().Get("q")
	if query == "" {
		query = elasticsearch.GetQueryFromRequestBody(e.EngineConfig, contentType, body)
	}

	ev := &apievents.OpenSearchRequest{
		Metadata: common.MakeEventMetadata(e.sessionCtx,
			events.DatabaseSessionOpenSearchRequestEvent,
			eventCode),
		UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
		StatusCode:       statusCode,
		Method:           req.Method,
		Path:             req.URL.Path,
		RawQuery:         req.URL.RawQuery,
		Body:             body,
		Headers:          wrappers.Traits(req.Header),
		Category:         category,
		Target:           target,
		Query:            query,
	}

	e.Audit.EmitEvent(e.Context, ev)
}

// sendResponse sends the response back to the OpenSearch client.
func (e *Engine) sendResponse(serverResponse *http.Response) error {
	if serverResponse.Body != nil {
		defer serverResponse.Body.Close()
		serverResponse.Body = io.NopCloser(io.LimitReader(serverResponse.Body, teleport.MaxHTTPResponseSize))
	}
	payload, err := utils.GetAndReplaceResponseBody(serverResponse)
	if err != nil {
		return trace.Wrap(err)
	}
	// serverResponse may be HTTP2 response, but we should reply with HTTP 1.1
	clientResponse := &http.Response{
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    serverResponse.StatusCode,
		Body:          io.NopCloser(bytes.NewBuffer(payload)),
		Header:        serverResponse.Header.Clone(),
		ContentLength: int64(len(payload)),
	}

	if err := clientResponse.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkAccess does authorization check for OpenSearch connection about
// to be established.
func (e *Engine) checkAccess(ctx context.Context) error {
	if e.sessionCtx.Identity.RouteToDatabase.Username == "" {
		return trace.BadParameter("database username required for OpenSearch")
	}

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
	return trace.Wrap(err)
}
